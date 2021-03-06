package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func getESIndices(oc *exutil.CLI, ns string, pod string) ([]ESIndex, error) {
	cmd := "es_util --query=_cat/indices?format=JSON"
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 9*time.Second)
	indices := []ESIndex{}
	json.Unmarshal([]byte(stdout), &indices)
	return indices, err
}

func getESIndicesByName(oc *exutil.CLI, ns string, pod string, indexName string) ([]ESIndex, error) {
	cmd := "es_util --query=_cat/indices/" + indexName + "*?format=JSON"
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 9*time.Second)
	indices := []ESIndex{}
	json.Unmarshal([]byte(stdout), &indices)
	return indices, err
}

func waitForIndexAppear(oc *exutil.CLI, ns string, pod string, indexName string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		indices, err := getESIndices(oc, ns, pod)
		count := 0
		for _, index := range indices {
			if strings.Contains(index.Index, indexName) {
				if index.Health != "red" {
					doc_count, _ := strconv.Atoi(index.DocsCount)
					count += doc_count
				}
			}
		}
		if count > 0 && err == nil {
			return true, nil
		} else {
			return false, err
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("index %s is not counted", indexName))
}

func getDocCountByQuery(oc *exutil.CLI, ns string, pod string, indexName string, queryString string) (int, error) {
	cmd := "es_util --query=" + indexName + "*/_count?format=JSON -d '" + queryString + "'"
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 30*time.Second)
	res := CountResult{}
	json.Unmarshal([]byte(stdout), &res)
	return res.Count, err
}

func waitForProjectLogsAppear(oc *exutil.CLI, ns string, pod string, projectName string, indexName string) {
	query := "{\"query\": {\"regexp\": {\"kubernetes.namespace_name\": \"" + projectName + "\"}}}"
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		logCount, err := getDocCountByQuery(oc, ns, pod, indexName, query)
		if err != nil {
			return false, err
		} else {
			if logCount > 0 {
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("log of index %s is not got", indexName))
}

func searchDocByQuery(oc *exutil.CLI, ns string, pod string, indexName string, queryString string) SearchResult {
	cmd := "es_util --query=" + indexName + "*/_search?format=JSON -d '" + queryString + "'"
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 30*time.Second)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	res := SearchResult{}
	//data := bytes.NewReader([]byte(stdout))
	//_ = json.NewDecoder(data).Decode(&res)
	json.Unmarshal([]byte(stdout), &res)
	return res
}

type externalES struct {
	namespace  string
	version    string // support 6.8 and 7.16
	serverName string // ES cluster name, configmap/sa/deploy/svc name
	httpSSL    bool   // `true` means enable `xpack.security.http.ssl`
	clientAuth bool   // `true` means `xpack.security.http.ssl.client_authentication: required`, only can be set to `true` when httpSSL is `true`
	userAuth   bool   // `true` means enable user auth
	username   string // shouldn't be empty when `userAuth: true`
	password   string // shouldn't be empty when `userAuth: true`
	secretName string //the name of the secret for the collector to use, it shouldn't be empty when `httpSSL: true` or `userAuth: true`
	loggingNS  string //the namespace where the collector pods deployed in
}

func (es externalES) createPipelineSecret(oc *exutil.CLI, keysPath string) {
	// create pipeline secret if needed
	cmd := []string{"secret", "generic", es.secretName, "-n", es.loggingNS}
	if es.clientAuth {
		cmd = append(cmd, "--from-file=tls.key="+keysPath+"/logging-es.key", "--from-file=tls.crt="+keysPath+"/logging-es.crt", "--from-file=ca-bundle.crt="+keysPath+"/ca.crt")
	} else if es.httpSSL && !es.clientAuth {
		cmd = append(cmd, "--from-file=ca-bundle.crt="+keysPath+"/ca.crt")
	}
	if es.userAuth {
		cmd = append(cmd, "--from-literal=username="+es.username, "--from-literal=password="+es.password)
	}

	err := oc.AsAdmin().WithoutNamespace().Run("create").Args(cmd...).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	resource{"secret", es.secretName, es.loggingNS}.WaitForResourceToAppear(oc)
}

func (es externalES) deploy(oc *exutil.CLI) {
	// create SA
	sa := resource{"serviceaccount", es.serverName, es.namespace}
	err := oc.WithoutNamespace().Run("create").Args("serviceaccount", sa.name, "-n", sa.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	sa.WaitForResourceToAppear(oc)

	if es.userAuth {
		o.Expect(es.username).NotTo(o.BeEmpty(), "Please provide username!")
		o.Expect(es.password).NotTo(o.BeEmpty(), "Please provide password!")
	}

	if es.httpSSL || es.clientAuth || es.userAuth {
		o.Expect(es.secretName).NotTo(o.BeEmpty(), "Please provide pipeline secret name!")

		// create a temporary directory
		baseDir := exutil.FixturePath("testdata", "logging")
		keysPath := filepath.Join(baseDir, "temp"+getRandomString())
		defer exec.Command("rm", "-r", keysPath).Output()
		err = os.MkdirAll(keysPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		cert := certsConf{es.serverName, es.namespace, ""}
		cert.generateCerts(keysPath)
		// create secret for ES if needed
		if es.httpSSL || es.clientAuth {
			r := resource{"secret", es.serverName, es.namespace}
			err = oc.WithoutNamespace().Run("create").Args("secret", "generic", "-n", r.namespace, r.name, "--from-file=elasticsearch.key="+keysPath+"/logging-es.key", "--from-file=elasticsearch.crt="+keysPath+"/logging-es.crt", "--from-file=admin-ca="+keysPath+"/ca.crt").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			r.WaitForResourceToAppear(oc)
		}

		// create pipeline secret in logging project
		es.createPipelineSecret(oc, keysPath)
	}

	// get file path per the configurations
	filePath := []string{"testdata", "logging", "external-log-stores", "elasticsearch", es.version}
	if es.httpSSL {
		filePath = append(filePath, "https")
	} else {
		o.Expect(es.clientAuth).NotTo(o.BeTrue(), "Unsupported configuration, please correct it!")
		filePath = append(filePath, "http")
	}
	if es.userAuth {
		filePath = append(filePath, "user_auth")
	} else {
		filePath = append(filePath, "no_user")
	}

	// create configmap
	cm := resource{"configmap", es.serverName, es.namespace}
	cmFilePath := append(filePath, "configmap.yaml")
	cmFile := exutil.FixturePath(cmFilePath...)
	cmPatch := []string{"-f", cmFile, "-n", cm.namespace, "-p", "NAMESPACE=" + es.namespace, "-p", "NAME=" + es.serverName}
	if es.userAuth {
		cmPatch = append(cmPatch, "-p", "USERNAME="+es.username, "-p", "PASSWORD="+es.password)
	}
	if es.httpSSL {
		if es.clientAuth {
			cmPatch = append(cmPatch, "-p", "CLIENT_AUTH=required")
		} else {
			cmPatch = append(cmPatch, "-p", "CLIENT_AUTH=none")
		}
	}
	cm.applyFromTemplate(oc, cmPatch...)

	// create deployment and expose svc
	deploy := resource{"deployment", es.serverName, es.namespace}
	deployFilePath := append(filePath, "deployment.yaml")
	deployFile := exutil.FixturePath(deployFilePath...)
	err = deploy.applyFromTemplate(oc, "-f", deployFile, "-n", es.namespace, "-p", "NAMESPACE="+es.namespace, "-p", "NAME="+es.serverName)
	o.Expect(err).NotTo(o.HaveOccurred())
	WaitForDeploymentPodsToBeReady(oc, es.namespace, es.serverName)
	err = oc.AsAdmin().WithoutNamespace().Run("expose").Args("-n", es.namespace, "deployment", es.serverName, "--name="+es.serverName).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (es externalES) remove(oc *exutil.CLI) {
	resource{"service", es.serverName, es.namespace}.clear(oc)
	resource{"configmap", es.serverName, es.namespace}.clear(oc)
	resource{"deployment", es.serverName, es.namespace}.clear(oc)
	resource{"serviceaccount", es.serverName, es.namespace}.clear(oc)
	if es.httpSSL || es.userAuth {
		resource{"secret", es.secretName, es.loggingNS}.clear(oc)
	}
	if es.httpSSL {
		resource{"secret", es.serverName, es.namespace}.clear(oc)
	}
}

func (es externalES) getPodName(oc *exutil.CLI) string {
	es_pods, err := oc.AdminKubeClient().CoreV1().Pods(es.namespace).List(metav1.ListOptions{LabelSelector: "app=" + es.serverName})
	o.Expect(err).NotTo(o.HaveOccurred())
	var names []string
	for i := 0; i < len(es_pods.Items); i++ {
		names = append(names, es_pods.Items[i].Name)
	}
	return names[0]
}

func (es externalES) baseCurlString() string {
	curlString := "curl -H \"Content-Type: application/json\""
	if es.userAuth {
		curlString += " -u " + es.username + ":" + es.password
	}
	if es.httpSSL {
		if es.clientAuth {
			curlString += " --cert /usr/share/elasticsearch/config/secret/elasticsearch.crt --key /usr/share/elasticsearch/config/secret/elasticsearch.key"
		}
		curlString += " --cacert /usr/share/elasticsearch/config/secret/admin-ca -s https://localhost:9200/"
	} else {
		curlString += " -s http://localhost:9200/"
	}
	return curlString
}

func (es externalES) getIndices(oc *exutil.CLI) ([]ESIndex, error) {
	cmd := es.baseCurlString() + "_cat/indices?format=JSON"
	stdout, err := e2e.RunHostCmdWithRetries(es.namespace, es.getPodName(oc), cmd, 3*time.Second, 9*time.Second)
	indices := []ESIndex{}
	json.Unmarshal([]byte(stdout), &indices)
	return indices, err
}

func (es externalES) waitForIndexAppear(oc *exutil.CLI, indexName string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		indices, err := es.getIndices(oc)
		count := 0
		for _, index := range indices {
			if strings.Contains(index.Index, indexName) {
				if index.Health != "red" {
					doc_count, _ := strconv.Atoi(index.DocsCount)
					count += doc_count
				}
			}
		}
		if count > 0 && err == nil {
			return true, nil
		} else {
			return false, err
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("index %s is not counted", indexName))
}

func (es externalES) getDocCount(oc *exutil.CLI, indexName string, queryString string) (int, error) {
	cmd := es.baseCurlString() + indexName + "*/_count?format=JSON -d '" + queryString + "'"
	stdout, err := e2e.RunHostCmdWithRetries(es.namespace, es.getPodName(oc), cmd, 3*time.Second, 30*time.Second)
	res := CountResult{}
	json.Unmarshal([]byte(stdout), &res)
	return res.Count, err
}

func (es externalES) waitForProjectLogsAppear(oc *exutil.CLI, projectName string, indexName string) {
	query := "{\"query\": {\"match_phrase\": {\"kubernetes.namespace_name\": \"" + projectName + "\"}}}"
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		logCount, err := es.getDocCount(oc, indexName, query)
		if err != nil {
			return false, err
		} else {
			if logCount > 0 {
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("log of index %s is not got", indexName))
}

func (es externalES) searchDocByQuery(oc *exutil.CLI, indexName string, queryString string) SearchResult {
	cmd := es.baseCurlString() + indexName + "*/_search?format=JSON -d '" + queryString + "'"
	stdout, err := e2e.RunHostCmdWithRetries(es.namespace, es.getPodName(oc), cmd, 3*time.Second, 30*time.Second)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	res := SearchResult{}
	//data := bytes.NewReader([]byte(stdout))
	//_ = json.NewDecoder(data).Decode(&res)
	json.Unmarshal([]byte(stdout), &res)
	return res
}
