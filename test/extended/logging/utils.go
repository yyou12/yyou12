package logging

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// SubscriptionObjects objects are used to create operators via OLM
type SubscriptionObjects struct {
	OperatorName  string
	Namespace     string
	OperatorGroup string // the file used to create operator group
	Subscription  string // the file used to create subscription
	PackageName   string
	CatalogSource CatalogSourceObjects `json:",omitempty"`
}

type CatalogSourceObjects struct {
	Channel         string `json:",omitempty"`
	SourceName      string `json:",omitempty"`
	SourceNamespace string `json:",omitempty"`
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func (so *SubscriptionObjects) getChannelName(oc *exutil.CLI) string {
	var channelName string
	if so.CatalogSource.Channel != "" {
		channelName = so.CatalogSource.Channel
	} else {
		/*
			clusterVersion, err := oc.AsAdmin().AdminConfigClient().ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
			if err != nil {
				return "", err
			}
			e2e.Logf("clusterversion is: %v\n", clusterVersion.Status.Desired.Version)
			channelName = strings.Join(strings.Split(clusterVersion.Status.Desired.Version, ".")[0:2], ".")
		*/
		channelName = "stable-5.3"
	}
	e2e.Logf("the channel name is: %v\n", channelName)
	return channelName
}

func (so *SubscriptionObjects) getSourceNamespace(oc *exutil.CLI) string {
	var catsrcNamespaceName string
	if so.CatalogSource.SourceNamespace != "" {
		catsrcNamespaceName = so.CatalogSource.SourceNamespace
	} else {
		catsrcNamespaceName = "openshift-marketplace"
	}
	e2e.Logf("The source namespace name is: %v\n", catsrcNamespaceName)
	return catsrcNamespaceName
}

func (so *SubscriptionObjects) getCatalogSourceName(oc *exutil.CLI) string {
	var catsrcName, catsrcNamespaceName string
	catsrcNamespaceName = so.getSourceNamespace(oc)
	catsrc, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catsrc", "-n", catsrcNamespaceName, "qe-app-registry").Output()

	err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", catsrcNamespaceName, "packagemanifests", so.PackageName).Execute()
	if err != nil {
		e2e.Logf("Can't check the packagemanifest %s existence: %v", so.PackageName, err)
	}
	if so.CatalogSource.SourceName != "" {
		catsrcName = so.CatalogSource.SourceName
	} else if catsrc != "" && !(strings.Contains(catsrc, "NotFound")) {
		catsrcName = "qe-app-registry"
	} else {
		catsrcName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests", so.PackageName, "-o", "jsonpath={.status.catalogSource}").Output()
		if err != nil {
			e2e.Logf("error getting catalog source name: %v", err)
		}
	}
	e2e.Logf("The catalog source name of %s is: %v\n", so.PackageName, catsrcName)
	return catsrcName
}

// SubscribeLoggingOperators is used to subcribe the CLO and EO
func (so *SubscriptionObjects) SubscribeLoggingOperators(oc *exutil.CLI) {
	// check if the namespace exists, if it doesn't exist, create the namespace
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(so.Namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			e2e.Logf("The project %s is not found, create it now...", so.Namespace)
			namespaceTemplate := exutil.FixturePath("testdata", "logging", "subscription", "namespace.yaml")
			namespaceFile, err := oc.AsAdmin().Run("process").Args("-f", namespaceTemplate, "-p", "NAMESPACE_NAME="+so.Namespace).OutputToFile(getRandomString() + ".json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
				output, err := oc.AsAdmin().Run("apply").Args("-f", namespaceFile).Output()
				if err != nil {
					if strings.Contains(output, "AlreadyExists") {
						return true, nil
					} else {
						return false, err
					}
				} else {
					return true, nil
				}
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can't create project %s", so.Namespace))
		}
	}

	// check the operator group, if no object found, then create an operator group in the project
	og, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", so.Namespace, "og").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	msg := fmt.Sprintf("%v", og)
	if strings.Contains(msg, "No resources found") {
		// create operator group
		ogFile, err := oc.AsAdmin().Run("process").Args("-n", so.Namespace, "-f", so.OperatorGroup, "-p", "OG_NAME="+so.PackageName, "NAMESPACE="+so.Namespace).OutputToFile(getRandomString() + ".json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
			output, err := oc.AsAdmin().Run("apply").Args("-f", ogFile, "-n", so.Namespace).Output()
			if err != nil {
				if strings.Contains(output, "AlreadyExists") {
					return true, nil
				} else {
					return false, err
				}
			} else {
				return true, nil
			}
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can't create operatorgroup %s in %s project", so.PackageName, so.Namespace))
	}

	// subscribe operator if the deployment doesn't exist
	_, err = oc.AdminKubeClient().AppsV1().Deployments(so.Namespace).Get(so.OperatorName, metav1.GetOptions{})
	if err != nil {
		// check subscription, if there is no subscription objets, then create one
		if apierrors.IsNotFound(err) {
			sub, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "-n", so.Namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			msg := fmt.Sprintf("%v", sub)
			if strings.Contains(msg, "No resources found") {
				catsrcNamespaceName := so.getSourceNamespace(oc)
				catsrcName := so.getCatalogSourceName(oc)
				channelName := so.getChannelName(oc)
				//check if the packagemanifest is exists in the source namespace or not
				packages, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", catsrcNamespaceName, "packagemanifests", "-l", "catalog="+catsrcName, "-o", "name").Output()
				o.Expect(packages).Should(o.ContainSubstring(so.PackageName))
				//create subscription object
				subscriptionFile, err := oc.AsAdmin().Run("process").Args("-n", so.Namespace, "-f", so.Subscription, "-p", "PACKAGE_NAME="+so.PackageName, "NAMESPACE="+so.Namespace, "CHANNEL="+channelName, "SOURCE="+catsrcName, "SOURCE_NAMESPACE="+catsrcNamespaceName).OutputToFile(getRandomString() + ".json")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
					output, err := oc.AsAdmin().Run("apply").Args("-f", subscriptionFile, "-n", so.Namespace).Output()
					if err != nil {
						if strings.Contains(output, "AlreadyExists") {
							return true, nil
						} else {
							return false, err
						}
					} else {
						return true, nil
					}
				})
				exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can't create subscription %s in %s project", so.PackageName, so.Namespace))
			}
		}
	}
	WaitForDeploymentPodsToBeReady(oc, so.Namespace, so.OperatorName)
}

//WaitForDeploymentPodsToBeReady waits for the specific deployment to be ready
func WaitForDeploymentPodsToBeReady(oc *exutil.CLI, namespace string, name string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of %s deployment\n", name)
				return false, nil
			}
			return false, err
		}
		if int(deployment.Status.AvailableReplicas) == int(deployment.Status.Replicas) {
			replicas := int(deployment.Status.Replicas)
			e2e.Logf("Deployment %s available (%d/%d)\n", name, replicas, replicas)
			return true, nil
		} else {
			e2e.Logf("Waiting for full availability of %s deployment (%d/%d)\n", name, deployment.Status.AvailableReplicas, deployment.Status.Replicas)
			return false, nil
		}

	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("deployment %s is not availabile", name))
}

//WaitForDaemonsetPodsToBeReady waits for all the pods controlled by the ds to be ready
func WaitForDaemonsetPodsToBeReady(oc *exutil.CLI, ns string, name string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of %s daemonset\n", name)
				return false, nil
			}
			return false, err
		}
		if int(daemonset.Status.NumberReady) == int(daemonset.Status.DesiredNumberScheduled) {
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s daemonset (%d/%d)\n", name, int(daemonset.Status.NumberReady), int(daemonset.Status.DesiredNumberScheduled))
		return false, nil
	})
	e2e.Logf("Daemonset %s is available\n", name)
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Daemonset %s is not availabile", name))
}

//GetDeploymentsNameByLabel retruns a list of deployment name which have specific labels
func GetDeploymentsNameByLabel(oc *exutil.CLI, ns string, label string) []string {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of deployment\n")
				return false, nil
			}
			return false, err
		}
		if len(deployList.Items) > 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("deployment with label %s is not availabile", label))
	if err == nil {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeployments := make([]string, 0, len(deployList.Items))
		for _, deploy := range deployList.Items {
			expectedDeployments = append(expectedDeployments, deploy.Name)
		}
		return expectedDeployments
	}
	return nil
}

//WaitForEFKPodsToBeReady checks if the EFK pods could become ready or not
func WaitForEFKPodsToBeReady(oc *exutil.CLI, ns string) {
	//wait for ES
	esDeployNames := GetDeploymentsNameByLabel(oc, ns, "cluster-name=elasticsearch")
	for _, name := range esDeployNames {
		WaitForDeploymentPodsToBeReady(oc, ns, name)
	}
	// wait for Kibana
	WaitForDeploymentPodsToBeReady(oc, ns, "kibana")
	// wait for collector
	WaitForDaemonsetPodsToBeReady(oc, ns, "collector")
}

type resource struct {
	kind      string
	name      string
	namespace string
}

//WaitUntilResourceIsGone waits for the resource to be removed cluster
func (r resource) WaitUntilResourceIsGone(oc *exutil.CLI) error {
	return wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", r.namespace, r.kind, r.name).Output()
		if err != nil {
			errstring := fmt.Sprintf("%v", output)
			if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
}

//delete the objects in the cluster
func (r resource) clear(oc *exutil.CLI) error {
	msg, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", r.namespace, r.kind, r.name).Output()
	if err != nil {
		errstring := fmt.Sprintf("%v", msg)
		if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") {
			return nil
		}
		return err
	}
	err = r.WaitUntilResourceIsGone(oc)
	return err
}

func (r resource) WaitForResourceToAppear(oc *exutil.CLI) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", r.namespace, r.kind, r.name).Output()
		if err != nil {
			msg := fmt.Sprintf("%v", output)
			if strings.Contains(msg, "NotFound") {
				return false, nil
			}
			return false, err
		}
		e2e.Logf("Find %s %s", r.kind, r.name)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %s is not appear", r.name))
}

func (r resource) applyFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + ".json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile, "-n", r.namespace).Execute()
	r.WaitForResourceToAppear(oc)
	return err
}

//DeleteClusterLogging deletes the clusterlogging instance and ensures the related resources are removed
func (r resource) deleteClusterLogging(oc *exutil.CLI) {
	err := r.clear(oc)
	if err != nil {
		e2e.Logf("could not delete %s/%s", r.kind, r.name)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("could not delete %s/%s", r.kind, r.name))
	//make sure other resources are removed
	resources := []resource{{"elasticsearches.logging.openshift.io", "elasticsearch", r.namespace}, {"kibanas.logging.openshift.io", "kibana", r.namespace}, {"daemonset", "collector", r.namespace}}
	for i := 0; i < len(resources); i++ {
		err = resources[i].WaitUntilResourceIsGone(oc)
		if err != nil {
			e2e.Logf("%s/%s is not deleted", resources[i].kind, resources[i].name)
		}
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s/%s is not deleted", resources[i].kind, resources[i].name))
	}
	// remove all the pvcs in the namespace
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", r.namespace, "pvc", "--all").Execute()
}

func (r resource) createClusterLogging(oc *exutil.CLI, parameters ...string) {
	// delete clusterlogging instance first
	r.deleteClusterLogging(oc)
	err := r.applyFromTemplate(oc, parameters...)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func DeleteNamespace(oc *exutil.CLI, ns string) {
	err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = nil
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func WaitForIMCronJobToAppear(oc *exutil.CLI, ns string, name string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		_, err = oc.AdminKubeClient().BatchV1beta1().CronJobs(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of cronjob\n")
				return false, nil
			} else {
				return false, err
			}
		} else {
			return true, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cronjob %s is not availabile", name))
}

func waitForIMJobsToComplete(oc *exutil.CLI, ns string, timeout time.Duration) {
	// wait for jobs to appear
	err := wait.Poll(5*time.Second, timeout, func() (done bool, err error) {
		jobList, err := oc.AdminKubeClient().BatchV1().Jobs(ns).List(metav1.ListOptions{LabelSelector: "component=indexManagement"})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of jobs\n")
				return false, nil
			}
			return false, err
		}
		if len(jobList.Items) >= 3 {
			return true, nil
		} else {
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("jobs %s with is not availabile", "component=indexManagement"))
	// wait for jobs to complete
	jobList, err := oc.AdminKubeClient().BatchV1().Jobs(ns).List(metav1.ListOptions{LabelSelector: "component=indexManagement"})
	if err != nil {
		panic(err)
	}
	for _, job := range jobList.Items {
		err := wait.Poll(2*time.Second, 60*time.Second, func() (bool, error) {
			job, err := oc.AdminKubeClient().BatchV1().Jobs(ns).Get(job.Name, metav1.GetOptions{})
			//succeeded, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "job", job.Name, "-o=jsonpath={.status.succeeded}").Output()
			if err != nil {
				return false, err
			} else {
				if job.Status.Succeeded == 1 {
					e2e.Logf("job %s completed successfully", job.Name)
					return true, nil
				} else {
					e2e.Logf("job %s is not completed yet", job.Name)
					return false, nil
				}
			}
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("job %s is not completed yet", job.Name))
	}
}

func getStorageClassName(oc *exutil.CLI) (string, error) {
	var scName string
	defaultSC := ""
	SCs, err := oc.AdminKubeClient().StorageV1().StorageClasses().List(metav1.ListOptions{})
	for _, sc := range SCs.Items {
		if sc.ObjectMeta.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultSC = sc.Name
			break
		}
	}
	if defaultSC != "" {
		scName = defaultSC
	} else {
		scName = SCs.Items[0].Name
	}
	return scName, err
}

//Assert the status of cluster logging components
func (r resource) assertResourceStatus(oc *exutil.CLI, content string, exptdStatus string) {
	err := wait.Poll(10*time.Second, 180*time.Second, func() (done bool, err error) {
		clStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(r.kind, r.name, "-n", r.namespace, "-o", content).Output()
		if err != nil {
			return false, err
		} else {
			if strings.Compare(clStatus, exptdStatus) != 0 {
				return false, nil
			} else {
				return true, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s %s value for %s is not %s", r.kind, r.name, content, exptdStatus))
}

type PrometheusQueryResult struct {
	Data struct {
		Result []struct {
			Metric struct {
				Name              string `json:"__name__"`
				Cluster           string `json:"cluster"`
				Container         string `json:"container"`
				Endpoint          string `json:"endpoint"`
				Instance          string `json:"instance"`
				Job               string `json:"job"`
				Namespace         string `json:"namespace"`
				Pod               string `json:"pod"`
				Service           string `json:"service"`
				ExportedNamespace string `json:"exported_namespace,omitempty"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}

// queryPrometheus returns the promtheus metrics which match the query string
// token: the user token used to run the http request, if it's not specified, it will use the token of sa/prometheus-k8s in openshift-monitoring project
// path: the api path, for example: /api/v1/query?
// query: the metric/alert you want to search, e.g.: es_index_namespaces_total
// action: it can be "GET", "get", "Get", "POST", "post", "Post"
func queryPrometheus(oc *exutil.CLI, token string, path string, query string, action string) (PrometheusQueryResult, error) {
	var bearerToken string
	if token == "" {
		bearerToken, _ = oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	} else {
		bearerToken = token
	}
	route, err := oc.AdminRouteClient().RouteV1().Routes("openshift-monitoring").Get("prometheus-k8s", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	prometheusURL := "https://" + route.Spec.Host + path
	if query != "" {
		prometheusURL = prometheusURL + "query=" + url.QueryEscape(query)
	}

	var tr *http.Transport
	if os.Getenv("http_proxy") != "" || os.Getenv("https_proxy") != "" {
		var proxy string
		if os.Getenv("http_proxy") != "" {
			proxy = os.Getenv("http_proxy")
		} else {
			proxy = os.Getenv("https_proxy")
		}
		proxyURL, err := url.Parse(proxy)
		o.Expect(err).NotTo(o.HaveOccurred())
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyURL(proxyURL),
		}
	} else {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	client := &http.Client{Transport: tr}
	var request *http.Request
	switch action {
	case "GET", "get", "Get":
		request, err = http.NewRequest("GET", prometheusURL, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
	case "POST", "post", "Post":
		request, err = http.NewRequest("POST", prometheusURL, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
	default:
		e2e.Failf("Unrecogonized action: %s", action)
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+bearerToken)
	response, err := client.Do(request)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer response.Body.Close()
	responseData, err := ioutil.ReadAll(response.Body)
	res := PrometheusQueryResult{}
	json.Unmarshal(responseData, &res)
	return res, err
}

//Wait for pods selected with labelselector to be removed
func WaitUntilPodsAreGone(oc *exutil.CLI, namespace string, labelSelector string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "--selector="+labelSelector, "-n", namespace).Output()
		if err != nil {
			return false, err
		} else {
			errstring := fmt.Sprintf("%v", output)
			if strings.Contains(errstring, "No resources found") {
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Error waiting for pods to be removed using label selector %s", labelSelector))
}

//Check logs from resource
func (r resource) checkLogsFromRs(oc *exutil.CLI, expected string, containerName string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(r.kind+`/`+r.name, "-n", r.namespace, "-c", containerName).Output()
		if err != nil {
			e2e.Logf("Can't get logs from resource, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.Match(expected, []byte(output)); !matched {
			e2e.Logf("Can't find the expected string\n")
			return false, nil
		} else {
			e2e.Logf("Check the logs succeed!!\n")
			return true, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s is not expected for %s", expected, r.name))
}
