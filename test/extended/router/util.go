package router

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type ingctrlNodePortDescription struct {
	name        string
	namespace   string
	defaultCert string
	domain      string
	replicas    int
	template    string
}

type ipfailoverDescription struct {
	name      string
	namespace string
	image     string
	vip       string
	template  string
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

func getBaseDomain(oc *exutil.CLI) string {
	var basedomain string

	basedomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("dns.config/cluster", "-o=jsonpath={.spec.baseDomain}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the base domain of the cluster: %v", basedomain)
	return basedomain
}

func (ingctrl *ingctrlNodePortDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ingctrl.template, "-p", "NAME="+ingctrl.name, "NAMESPACE="+ingctrl.namespace, "DOMAIN="+ingctrl.domain)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (ingctrl *ingctrlNodePortDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ingctrl.namespace, "ingresscontroller", ingctrl.name).Execute()
}

func createResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "-temp-resource.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("the file of resource is %s", jsonCfg)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", jsonCfg).Execute()
}

func waitForCustomIngressControllerAvailable(oc *exutil.CLI, icname string) error {
	e2e.Logf("check ingresscontroller if available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresscontroller", icname, "--namespace=openshift-ingress-operator", "-ojsonpath={.status.conditions[?(@.type==\"Available\")].status}").Output()
		e2e.Logf("the status of ingresscontroller is %v", status)
		if err != nil {
			e2e.Logf("failed to get ingresscontroller %s: %v, retrying...", icname, err)
			return false, nil
		}
		if strings.Contains(status, "False") {
			e2e.Logf("ingresscontroller %s conditions not available, retrying...", icname)
			return false, nil
		}
		return true, nil
	})
}

func waitForPodWithLabelReady(oc *exutil.CLI, ns, label string) error {
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", label, "-ojsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}").Output()
		e2e.Logf("the Ready status of pod is %v", status)
		if err != nil || status == "" {
			e2e.Logf("failed to get pod status: %v, retrying...", err)
			return false, nil
		}
		if strings.Contains(status, "False") {
			e2e.Logf("the pod Ready status not met; wanted True but got %v, retrying...", status)
			return false, nil
		}
		return true, nil
	})
}

// wait for the named resource is disappeared, e.g. used while router deployment rolled out
func waitForResourceToDisappear(oc *exutil.CLI, ns, rsname string) error {
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(rsname, "-n", ns).Output()
		e2e.Logf("check resource %v and got: %v", rsname, status)
		if err != nil {
			if strings.Contains(status, "NotFound") {
				e2e.Logf("the resource is disappeared!")
				return true, nil
			} else {
				e2e.Logf("failed to get the resource: %v, retrying...", err)
				return false, nil
			}
		} else {
			e2e.Logf("the resource is still there, retrying...")
			return false, nil
		}
	})
}

// For normal user to create resources in the specified namespace from the file (not template)
func createResourceFromFile(oc *exutil.CLI, ns, file string) {
	err := oc.WithoutNamespace().Run("create").Args("-f", file, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// For normal user to patch a resource in the specified namespace
func patchResourceAsUser(oc *exutil.CLI, ns, resource, patch string) {
	err := oc.WithoutNamespace().Run("patch").Args(resource, "-p", patch, "--type=merge", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// For Admin to patch a resource in the specified namespace
func patchResourceAsAdmin(oc *exutil.CLI, ns, resource, patch string) {
	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(resource, "-p", patch, "--type=merge", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func exposeRoute(oc *exutil.CLI, ns, resource string) {
	err := oc.Run("expose").Args(resource, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func setAnnotation(oc *exutil.CLI, ns, resource, annotation string) {
	err := oc.Run("annotate").Args("-n", ns, resource, annotation, "--overwrite").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// for collecting a single pod name for general use.
//usage example: podname := getRouterPod(oc, "default/labelname")
func getRouterPod(oc *exutil.CLI, icname string) string {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", "ingresscontroller.operator.openshift.io/deployment-ingresscontroller="+icname, "-o=jsonpath={.items[0].metadata.name}", "-n", "openshift-ingress").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of podname:%v", podName)
	return podName
}

// For collecting env details with grep from router pod [usage example: readDeploymentData(oc, podname, "search string")] .
// NOTE: This requires getRouterPod function to collect the podname variable first!
func readRouterPodEnv(oc *exutil.CLI, routername, envname string) string {
	cmd := fmt.Sprintf("/usr/bin/env | grep %s", envname)
	output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routername, "--", "bash", "-c", cmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the matched Env are: %v", output)
	return output
}

// to collect pod name and check parameter related to OCP-42230
func readHaproxyConfig(oc *exutil.CLI) error {
	e2e.Logf("check Haproxy config file")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		e2e.Logf("Get podname")
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", "ingresscontroller.operator.openshift.io/deployment-ingresscontroller=default", "-o=jsonpath={.items[0].metadata.name}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		final, err1 := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", podName, "--", "grep", "-w", `backend be_http:`+oc.Namespace()+`:service-unsecure`, "-A7", "/var/lib/haproxy/conf/haproxy.config").Output()
		if err1 != nil {
			e2e.Logf("Something went wrong, string search failed")
			return false, nil
		}
		if strings.Contains(final, "acl whitelist") {
			e2e.Logf("string search successful")
			return true, nil
		}
		return false, nil
	})
}

func getImagePullSpecFromPayload(oc *exutil.CLI, image string) string {
	var pullspec string
	baseDir := exutil.FixturePath("testdata", "router")
	indexTmpPath := filepath.Join(baseDir, getRandomString())
	dockerconfigjsonpath := filepath.Join(indexTmpPath, ".dockerconfigjson")
	defer exec.Command("rm", "-rf", indexTmpPath).Output()
	err := os.MkdirAll(indexTmpPath, 0755)
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--confirm", "--to="+indexTmpPath).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	pullspec, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("release", "info", "--image-for="+image, "-a", dockerconfigjsonpath).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the pull spec of image %v is: %v", image, pullspec)
	return pullspec
}

func (ipf *ipfailoverDescription) create(oc *exutil.CLI, ns string) {
	// create ServiceAccount and add it to related SCC
	_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", "ipfailover", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "privileged", "-z", "ipfailover").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	// create the ipfailover deployment
	err = createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ipf.template, "-p", "NAME="+ipf.name, "NAMESPACE="+ipf.namespace, "IMAGE="+ipf.image)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForIpfailoverEnterMaster(oc *exutil.CLI, ns, label string) error {
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		log, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", ns, "-l", label).Output()
		e2e.Logf("the logs of labeled pods are: %v", log)
		if err != nil || log == "" {
			e2e.Logf("failed to get logs: %v, retrying...", err)
			return false, nil
		}
		if !strings.Contains(log, "Entering MASTER STATE") {
			e2e.Logf("no ipfailover pod is in MASTER state, retrying...")
			return false, nil
		}
		return true, nil
	})
}

// For collecting information from router pod [usage example: readRouterPodData(oc, podname, executeCmd, "search string")] .
// NOTE: This requires getRouterPod function to collect the podname variable first!
func readRouterPodData(oc *exutil.CLI, routername, executeCmd string, searchString string) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routername, "--", "bash", "-c", executeCmd, "|", "grep", "-w", searchString).Output()
	e2e.Logf("The output from the search: %v", output)
	o.Expect(err).NotTo(o.HaveOccurred())
	return output
}

func createConfigMapFromFile(oc *exutil.CLI, ns, name, cmFile string) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", name, "--from-file="+cmFile, "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteConfigMap(oc *exutil.CLI, ns, name string) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("configmap", name, "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}
