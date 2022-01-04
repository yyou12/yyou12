package image_registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	asAdmin          = true
	withoutNamespace = true
)

type PrometheusResponse struct {
	Status string                 `json:"status"`
	Error  string                 `json:"error"`
	Data   prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	ResultType string       `json:"resultType"`
	Result     model.Vector `json:"result"`
}

func ListPodStartingWith(prefix string, oc *exutil.CLI, namespace string) (pod []corev1.Pod) {
	podsToAll := []corev1.Pod{}
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		e2e.Logf("Error listing pods: %v", err)
		return nil
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, prefix) {
			podsToAll = append(podsToAll, pod)
		}
	}
	return podsToAll
}

func DePodLogs(pods []corev1.Pod, oc *exutil.CLI, matchlogs string) bool {
	for _, pod := range pods {
		depOutput, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-n", pod.Namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(depOutput, matchlogs) {
			return true
		}
	}
	return false
}

func getBearerTokenURLViaPod(ns string, execPodName string, url string, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl --retry 15 --max-time 2 --retry-delay 1 -s -k -H 'Authorization: Bearer %s' %s", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func runQuery(queryUrl, ns, execPodName, bearerToken string) (*PrometheusResponse, error) {
	contents, err := getBearerTokenURLViaPod(ns, execPodName, queryUrl, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("unable to execute query %v", err)
	}
	var result PrometheusResponse
	if err := json.Unmarshal([]byte(contents), &result); err != nil {
		return nil, fmt.Errorf("unable to parse query response: %v", err)
	}
	metrics := result.Data.Result
	if result.Status != "success" {
		data, _ := json.MarshalIndent(metrics, "", "  ")
		return nil, fmt.Errorf("incorrect response status: %s with error %s", data, result.Error)
	}
	return &result, nil
}

func metricReportStatus(queryUrl, ns, execPodName, bearerToken string, value model.SampleValue) bool {
	result, err := runQuery(queryUrl, ns, execPodName, bearerToken)
	o.Expect(err).NotTo(o.HaveOccurred())
	if result.Data.Result[0].Value == value {
		return true
	}
	return false
}

type bcSource struct {
	outname   string
	name      string
	namespace string
	template  string
}

type authRole struct {
	namespace string
	rolename  string
	template  string
}

func (bcsrc *bcSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", bcsrc.template, "-p", "OUTNAME="+bcsrc.outname, "NAME="+bcsrc.name, "NAMESPACE="+bcsrc.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (authrole *authRole) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", authrole.template, "-p", "NAMESPACE="+authrole.namespace, "ROLE_NAME="+authrole.rolename)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "Applying resources from template is failed")
	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

//the method is to get something from resource. it is "oc get xxx" actaully
func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("output is %v, error is %v, and try next", result, err)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Failed to get %v", parameters))
	e2e.Logf("$oc get %v, the returned resource:%v", parameters, result)
	return result
}

//the method is to do something with oc.
func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}

func comparePodHostIp(oc *exutil.CLI) (int, int) {
	var hostsIp = []string{}
	var numi, numj int
	podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
	for _, pod := range podList.Items {
		hostsIp = append(hostsIp, pod.Status.HostIP)
	}
	for i := 0; i < len(hostsIp)-1; i++ {
		for j := i + 1; j < len(hostsIp); j++ {
			if hostsIp[i] == hostsIp[j] {
				numi++
			} else {
				numj++
			}
		}
	}
	return numi, numj
}

func imagePruneLog(oc *exutil.CLI, matchlogs string) bool {
	podsOfImagePrune := []corev1.Pod{}
	podsOfImagePrune = ListPodStartingWith("image-pruner", oc, "openshift-image-registry")
	for _, pod := range podsOfImagePrune {
		depOutput, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-n", pod.Namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(depOutput, matchlogs) {
			return true
			break
		}
	}
	return false
}

func configureRegistryStorageToEmptyDir(oc *exutil.CLI) {
	var hasstorage string
	emptydirstorage, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.status.storage.emptyDir}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if emptydirstorage == "{}" {
		g.By("Image registry is using EmptyDir now")
	} else {
		g.By("Set registry to use EmptyDir storage")
		platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		switch platformtype {
		case "AWS":
			hasstorage = "s3"
		case "OpenStack":
			hasstorage = "swift"
		case "GCP":
			hasstorage = "gcs"
		case "Azure":
			hasstorage = "azure"
		default:
			e2e.Logf("Image Registry is using unknown storage type")
		}
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"`+hasstorage+`":null,"emptyDir":{}}, "replicas":1}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(30*time.Second, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(podList.Items) == 1 && podList.Items[0].Status.Phase == corev1.PodRunning {
				return true, nil
			} else {
				e2e.Logf("Continue to next round")
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 1")
		err = oc.AsAdmin().WithoutNamespace().Run("wait").Args("configs.imageregistry/cluster", "--for=condition=Available").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func recoverRegistryStorageConfig(oc *exutil.CLI) {
	g.By("Set image registry storage to default value")
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if platformtype != "VSphere" {
		if platformtype != "None" {
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":null}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Image registry will be auto-recovered to default storage")
		}
	}
}

func recoverRegistryDefaultReplicas(oc *exutil.CLI) {
	g.By("Set image registry to default replicas")
	platforms := map[string]bool{
		"VSphere": true,
		"None":    true,
		"oVirt":   true,
	}
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if !platforms[platformtype] {
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("config.imageregistry/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(30*time.Second, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 2 {
				e2e.Logf("Continue to next round")
			} else {
				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						e2e.Logf("Continue to next round")
						return false, nil
					}
				}
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 2")
	}
}

func restoreRegistryStorageConfig(oc *exutil.CLI) string {
	var storageinfo string
	g.By("Get image registry storage info")
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	switch platformtype {
	case "AWS":
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.s3.bucket}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "Azure":
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.azure.container}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "GCP":
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.gcs.bucket}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "OpenStack":
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.swift.container}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "None", "VSphere":
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.pvc.claim}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if storageinfo == "" {
			storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.emptyDir}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	default:
		e2e.Logf("Image Registry is using unknown storage type")
	}
	return storageinfo
}

func loginRegistryDefaultRoute(oc *exutil.CLI, defroute string, ns string) {
	var podmanCLI = container.NewPodmanCLI()
	containerCLI := podmanCLI

	g.By("Trust ca for default registry route on your client platform")
	crt, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("secret", "-n", "openshift-ingress", "router-certs-default", "-o", `go-template={{index .data "tls.crt"}}`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	sDec, err := base64.StdEncoding.DecodeString(crt)
	if err != nil {
		e2e.Logf("Error decoding string: %s ", err.Error())
	}
	fileName := "/etc/pki/ca-trust/source/anchors/" + defroute + ".crt"
	sDecode := string(sDec)
	cmd := " echo \"" + sDecode + "\"|sudo tee " + fileName + "> /dev/null"
	_, err = exec.Command("bash", "-c", cmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	caCmd := "sudo update-ca-trust enable"
	_, err = exec.Command("bash", "-c", caCmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Get admin permission to push image to your project")
	err = oc.Run("create").Args("serviceaccount", "registry", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.WithoutNamespace().AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "admin", "-z", "registry", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Login to route")
	token, err := oc.WithoutNamespace().AsAdmin().Run("sa").Args("get-token", "registry", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if output, err := containerCLI.Run("login").Args(defroute, "-u", "registry", "-p", token).Output(); err != nil {
		e2e.Logf(output)
	}
}

func recoverRegistryDefaultPods(oc *exutil.CLI) {
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	switch platformtype {
	case "AWS", "Azure", "GCP", "OpenStack":
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).Should(o.Equal("2"))
		err = wait.Poll(25*time.Second, 3*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 2 {
				e2e.Logf("Continue to next round")
				return false, nil
			} else {
				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						e2e.Logf("Continue to next round")
						return false, nil
					}
				}
				return true, nil
			}
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 2")
	case "None", "VSphere":
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).Should(o.Equal("1"))
		err = wait.Poll(25*time.Second, 3*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 1 {
				e2e.Logf("Continue to next round")
				return false, nil
			} else if podList.Items[0].Status.Phase != corev1.PodRunning {
				e2e.Logf("Continue to next round")
				return false, nil
			} else {
				return true, nil
			}
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 1")
	default:
		e2e.Failf("Failed to recover registry pods")
	}
}

func checkRegistrypodsRemoved(oc *exutil.CLI) {
	err := wait.Poll(25*time.Second, 3*time.Minute, func() (bool, error) {
		podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(podList.Items) == 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "Image registry pods are not removed")
}

type staSource struct {
	name      string
	namespace string
	template  string
}

func (stafulsrc *staSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", stafulsrc.template, "-p", "NAME="+stafulsrc.name, "NAMESPACE="+stafulsrc.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkPodsRunningWithLabel(oc *exutil.CLI, namespace string, label string, number int) {
	err := wait.Poll(20*time.Second, 1*time.Minute, func() (bool, error) {
		podList, _ := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: label})
		if len(podList.Items) != number {
			e2e.Logf("the pod number is not %s, Continue to next round", number)
			return false, nil
		} else if podList.Items[0].Status.Phase != corev1.PodRunning {
			e2e.Logf("the pod status is not running, continue to next round")
			return false, nil
		} else {
			return true, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pods list are not %d", number))
}

type icspSource struct {
	name     string
	template string
}

func (icspsrc *icspSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", icspsrc.template, "-p", "NAME="+icspsrc.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (icspsrc *icspSource) delete(oc *exutil.CLI) {
	e2e.Logf("deleting icsp: %s", icspsrc.name)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("imagecontentsourcepolicy", icspsrc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getRegistryDefaultRoute(oc *exutil.CLI) (defaultroute string) {
	err := wait.Poll(2*time.Second, 6*time.Second, func() (bool, error) {
		defroute, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("route/default-route", "-n", "openshift-image-registry", "-o=jsonpath={.spec.host}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(defroute) == 0 {
			e2e.Logf("Continue to next round")
			return false, nil
		} else {
			defaultroute = defroute
			return true, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Did not find registry route"))
	return defaultroute
}

func setImageregistryConfigs(oc *exutil.CLI, pathinfo string, matchlogs string) bool {
	foundInfo := false
	defer recoverRegistrySwiftSet(oc)
	err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"swift":{`+pathinfo+`}}}}`, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("co/image-registry").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, matchlogs) {
			foundInfo = true
			return true, nil
		} else {
			e2e.Logf("Continue to next round")
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("No image registry error info found"))
	return foundInfo
}

func recoverRegistrySwiftSet(oc *exutil.CLI) {
	matchInfo := "True False False"
	err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"swift":{"authURL":null, "regionName":null, "regionID":null, "domainID":null, "domain":null, "tenantID":null}}}}`, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(4*time.Second, 20*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/image-registry", "-o=jsonpath={.status.conditions[*].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, matchInfo) {
			return true, nil
		} else {
			e2e.Logf("Continue to next round")
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Image registry is degrade"))
}

type podSource struct {
	name      string
	namespace string
	image     string
	template  string
}

func (podsrc *podSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podsrc.template, "-p", "NAME="+podsrc.name, "NAMESPACE="+podsrc.namespace, "IMAGE="+podsrc.image)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkRegistryUsingFSVolume(oc *exutil.CLI) bool {
	storageinfo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(storageinfo, "pvc") || strings.Contains(storageinfo, "emptyDir") {
		return true
	}
	return false
}

func saveImageMetadataName(oc *exutil.CLI, image string) string {
	imagemetadata, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("images").OutputToFile("imagemetadata.txt")
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.Remove("imagemetadata.txt")
	manifest, err := exec.Command("bash", "-c", "cat "+imagemetadata+" | grep "+image+" | awk '{print $1}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.TrimSuffix(string(manifest), "\n")
}
