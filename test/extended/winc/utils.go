package winc

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func waitWindowsNodesReady(oc *exutil.CLI, nodesNumber int, interval time.Duration, timeout time.Duration) {
	pollErr := wait.Poll(interval, timeout, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "--no-headers").Output()
		nodesReady := strings.Count(msg, "Ready")
		if nodesReady != nodesNumber {
			e2e.Logf("Expected %v Windows nodes are not ready yet and waiting up to %v minutes ...", nodesNumber, timeout)
			return false, nil
		}
		e2e.Logf("Expected %v Windows nodes are ready", nodesNumber)
		return true, nil
	})
	if pollErr != nil {
		e2e.Failf("Expected %v Windows nodes are not ready after waiting up to %v minutes ...", nodesNumber, timeout)
	}
}

// This function returns the windows build e.g windows-build: '10.0.19041'
func getWindowsBuildID(oc *exutil.CLI, nodeID string) (string, error) {
	build, err := oc.WithoutNamespace().Run("get").Args("node", nodeID, "-o=jsonpath={.metadata.labels.node\\.kubernetes\\.io\\/windows-build}").Output()
	return build, err
}

func checkPodsHaveSimilarHostIP(oc *exutil.CLI, pods []string, nodeIP string) bool {
	for _, pod := range pods {
		e2e.Logf("Pod host IP is %v, of node IP, %v", pod, nodeIP)
		if pod != nodeIP {
			return false
		}
	}
	return true
}

func waitVersionAnnotationReady(oc *exutil.CLI, windowsNodeName string, interval time.Duration, timeout time.Duration) {
	pollErr := wait.Poll(interval, timeout, func() (bool, error) {
		retcode, err := checkVersionAnnotationReady(oc, windowsNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !retcode {
			e2e.Logf("Version annotation is not applied to Windows node %s yet", windowsNodeName)
			return false, nil
		}
		e2e.Logf("Version annotation is applied to Windows node %s", windowsNodeName)
		return true, nil
	})
	if pollErr != nil {
		e2e.Failf("Version annotation is not applied to Windows node %s after waiting up to %v minutes ...", windowsNodeName, timeout)
	}
}

func checkVersionAnnotationReady(oc *exutil.CLI, windowsNodeName string) (bool, error) {
	msg, err := oc.WithoutNamespace().Run("get").Args("nodes", windowsNodeName, "-o=jsonpath='{.metadata.annotations.windowsmachineconfig\\.openshift\\.io\\/version}'").Output()
	if msg == "" {
		return false, err
	}
	return true, err
}

func getWindowsMachineSetName(oc *exutil.CLI) string {
	// fetch the Windows MachineSet from all machinesets list
	myJSON := "-o=jsonpath={.items[?(@.spec.template.metadata.labels.machine\\.openshift\\.io\\/os-id==\"Windows\")].metadata.name}"
	windowsMachineSetName, err := oc.WithoutNamespace().Run("get").Args("machineset", "-n", "openshift-machine-api", myJSON).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return windowsMachineSetName
}

func getWindowsHostNames(oc *exutil.CLI) []string {
	winHostNames, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "-o=jsonpath={.items[*].status.addresses[?(@.type==\"Hostname\")].address}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(winHostNames, " ")
}

func getWindowsInternalIPs(oc *exutil.CLI) []string {
	winInternalIPs, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "-o=jsonpath={.items[*].status.addresses[?(@.type==\"InternalIP\")].address}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(winInternalIPs, " ")
}

func getSSHBastionHost(oc *exutil.CLI) string {
	msg, err := oc.WithoutNamespace().Run("get").Args("service", "--all-namespaces", "-l=run=ssh-bastion", "-o=go-template='{{ with (index (index .items 0).status.loadBalancer.ingress 0) }}{{ or .hostname .ip }}{{end}}'").Output()
	if err != nil || msg == "" {
		e2e.Failf("SSH bastion is not install yet")
	}
	return msg
}

// A private function to translate the workload/pod/deployment name
func getWorkloadName(os string) string {
	name := ""
	if os == "windows" {
		name = "win-webserver"
	} else if os == "linux" {
		name = "linux-webserver"
	} else {
		name = "windows-machine-config-operator"
	}
	return name
}

// A private function to determine username by platform
func getAdministratorNameByPlatform(iaasPlatform string) string {
	admin := ""
	if iaasPlatform == "azure" {
		admin = "capi"
	} else {
		admin = "Administrator"
	}
	return admin
}

// A private function to determine selector by OS
func getWorkloadSelector(os string) string {
	selector := "app"
	if os == "linux" {
		selector = "run"
	}
	return selector
}

func runPSCommand(bastionHost string, windowsHost string, command string, privateKey string, iaasPlatform string) (result string, err error) {
	windowsUser := getAdministratorNameByPlatform(iaasPlatform)
	msg, err := exec.Command("bash", "-c", "chmod 600 "+privateKey+"; ssh -i "+privateKey+" -t -o StrictHostKeyChecking=no -o ProxyCommand=\"ssh -i "+privateKey+" -A -o StrictHostKeyChecking=no -o ServerAliveInterval=30 -W %h:%p core@"+bastionHost+"\" "+windowsUser+"@"+windowsHost+" \"powershell "+command+"\"").CombinedOutput()
	return string(msg), err
}

func checkLinuxWorkloadCreated(oc *exutil.CLI, namespace string) bool {
	msg, _ := oc.WithoutNamespace().Run("get").Args("deployment", "linux-webserver", "-o=jsonpath={.status.readyReplicas}", "-n", namespace).Output()
	if msg != "1" {
		e2e.Logf("Linux workload is not created yet")
		return false
	}
	return true
}

func createLinuxWorkload(oc *exutil.CLI, namespace string) {
	oc.WithoutNamespace().Run("new-project").Args(namespace).Output()
	linuxWebServer := filepath.Join(exutil.FixturePath("testdata", "winc"), "linux_web_server.yaml")
	// Wait up to 3 minutes for Linux workload ready
	oc.WithoutNamespace().Run("create").Args("-f", linuxWebServer, "-n", namespace).Output()
	poolErr := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		return checkLinuxWorkloadCreated(oc, namespace), nil
	})
	if poolErr != nil {
		e2e.Failf("Linux workload is not ready after waiting up to 3 minutes ...")
	}
}

// TODO remove this function with
func checkWindowsWorkloadCreated(oc *exutil.CLI, namespace string) bool {
	msg, _ := oc.WithoutNamespace().Run("get").Args("deployment", "win-webserver", "-o=jsonpath={.status.readyReplicas}", "-n", namespace).Output()
	if msg != "1" {
		e2e.Logf("Windows workload is not created yet")
		return false
	}
	return true
}

func checkWindowsWorkloadScaled(oc *exutil.CLI, deploymentName string, namespace string, replicas int) bool {
	msg, _ := oc.WithoutNamespace().Run("get").Args("deployment", deploymentName, "-o=jsonpath={.status.readyReplicas}", "-n", namespace).Output()
	workloads := strconv.Itoa(replicas)
	if msg != workloads {
		e2e.Logf("Deployment " + deploymentName + " did not scaled to " + workloads)
		return false
	}
	return true
}

// TODO need to replace this function with createWinWorkloadsSimple also to be renamed
func createWindowsWorkload(oc *exutil.CLI, namespace string) {
	oc.WithoutNamespace().Run("new-project").Args(namespace).Output()
	msg, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath={.items[0].metadata.labels.node\\.kubernetes\\.io\\/windows-build}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	windowsImage := "mcr.microsoft.com/windows/servercore:ltsc2019"
	if msg == "10.0.19041" {
		windowsImage = "mcr.microsoft.com/windows/servercore:2004"
	}
	windowsWebServer := getFileContent("winc", "windows_web_server.yaml")
	windowsWebServer = strings.ReplaceAll(windowsWebServer, "<windows_docker_image>", windowsImage)
	ioutil.WriteFile("availWindowsWebServer", []byte(windowsWebServer), 0644)
	oc.WithoutNamespace().Run("create").Args("-f", "availWindowsWebServer", "-n", namespace).Output()
	// Wait up to 5 minutes for Windows workload ready
	poolErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
		return checkWindowsWorkloadCreated(oc, namespace), nil
	})
	if poolErr != nil {
		e2e.Failf("Windows workload is not ready after waiting up to 5 minutes ...")
	}
}

func createWinWorkloadsSimple(oc *exutil.CLI, namespace string, workloadsFile string, deadTime int) bool {
	retcode := false
	oc.WithoutNamespace().Run("new-project").Args(namespace).Output()
	windowsWebServer := getFileContent("winc", workloadsFile)
	ioutil.WriteFile("availWindowsWebServer2004", []byte(windowsWebServer), 0644)
	oc.WithoutNamespace().Run("create").Args("-f", "availWindowsWebServer2004", "-n", namespace).Output()
	// Wait up to 30 minutes for Windows workload ready
	poolErr := wait.Poll(15*time.Second, time.Duration(deadTime)*time.Minute, func() (bool, error) {
		retcode = true
		return checkWindowsWorkloadCreated(oc, namespace), nil
	})
	if poolErr != nil {
		retcode = false
	}
	return retcode
}

// Get an external IP of loadbalancer service
func getExternalIP(iaasPlatform string, oc *exutil.CLI, os string, namespace string) (string, error) {
	extIP := ""
	var err error
	serviceName := getWorkloadName(os)
	if iaasPlatform == "azure" {
		extIP, err = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.status.loadBalancer.ingress[0].ip}", "-n", namespace).Output()
	} else {
		extIP, err = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.status.loadBalancer.ingress[0].hostname}", "-n", namespace).Output()
	}
	return extIP, err
}

// we retrieve the ClusterIP from a pod according to it's OS
func getServiceClusterIP(oc *exutil.CLI, os string, namespace string) (string, error) {
	clusterIP := ""
	var err error
	serviceName := getWorkloadName(os)
	clusterIP, err = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.spec.clusterIP}", "-n", namespace).Output()
	return clusterIP, err
}

// Get file content in test/extended/testdata/<basedir>/<name>
func getFileContent(baseDir string, name string) (fileContent string) {
	filePath := filepath.Join(exutil.FixturePath("testdata", baseDir), name)
	fileOpen, err := os.Open(filePath)
	if err != nil {
		e2e.Failf("Failed to open file: %s", filePath)
	}
	fileRead, _ := ioutil.ReadAll(fileOpen)
	if err != nil {
		e2e.Failf("Failed to read file: %s", filePath)
	}
	return string(fileRead)
}

// this function scale the deployment workloads
func scaleDeployment(oc *exutil.CLI, os string, replicas int, namespace string) error {
	deploymentName := getWorkloadName(os)
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), "deployment", deploymentName, "-n", namespace).Output()
	poolErr := wait.Poll(60*time.Second, 300*time.Second, func() (bool, error) {
		return checkWindowsWorkloadScaled(oc, deploymentName, namespace, replicas), nil
	})
	if poolErr != nil {
		e2e.Failf("Workload did not scaled after waiting up to 5 minutes ...")
	}
	return err
}

func scaleWindowsMachineSet(oc *exutil.CLI, windowsMachineSetName string, replicas int) error {
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), "machineset", windowsMachineSetName, "-n", "openshift-machine-api").Output()
	return err
}

// this function delete a workspace, we intend to do it after each test case run
func deleteProject(oc *exutil.CLI, namespace string) error {
	_, err := oc.WithoutNamespace().Run("delete").Args("project", namespace).Output()
	return err
}

// this function returns an array of workloads names by their OS type
func getWorkloadsNames(oc *exutil.CLI, os string, namespace string) ([]string, error) {
	workloadName := getWorkloadName(os)
	selector := getWorkloadSelector(os)
	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector="+selector+"="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].metadata.name}", "-n", namespace).Output()
	pods := strings.Split(workloads, " ")
	return pods, err
}

// this function returns an array of workloads IP's by their OS type
func getWorkloadsIP(oc *exutil.CLI, os string, namespace string) ([]string, error) {
	workloadName := getWorkloadName(os)
	selector := getWorkloadSelector(os)
	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector="+selector+"="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].status.podIP}", "-n", namespace).Output()
	ips := strings.Split(workloads, " ")
	return ips, err
}

// this function returns an array of workloads IP's by their OS type
func getWorkloadsHostIP(oc *exutil.CLI, os string, namespace string) ([]string, error) {
	workloadName := getWorkloadName(os)
	selector := getWorkloadSelector(os)
	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector="+selector+"="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].status.hostIP}", "-n", namespace).Output()
	ips := strings.Split(workloads, " ")
	return ips, err
}

func scaleDownWMCO(oc *exutil.CLI) error {
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas=0", "deployment", "windows-machine-config-operator", "-n", "openshift-windows-machine-config-operator").Output()
	return err
}

// The output from JSON contains quotes, here we remove them
func removeOuterQuotes(s string) string {
	return regexp.MustCompile(`^"(.*)"$`).ReplaceAllString(s, `$1`)
}

// we truncate the go version to major Go version, e.g. 1.15.13 --> 1.15
func truncatedVersion(s string) string {
	s = removeOuterQuotes(s)
	str := strings.Split(s, ".")
	str = str[:2]
	return strings.Join(str[:], ".")
}
func getMachineset(oc *exutil.CLI, iaasPlatform, winVersion string, machineSetName string, fileName string) (windowsMachineSetName string, err error) {

	windowsMachineSet := ""
	infrastructureID := ""
	region := ""
	zone := ""
	if iaasPlatform == "aws" {
		windowsMachineSet = getFileContent("winc", fileName)
		infrastructureID, err = oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		region, err = oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
		zone, err = oc.WithoutNamespace().Run("get").Args("machines", "-n", "openshift-machine-api", "-o=jsonpath={.items[0].metadata.labels.machine\\.openshift\\.io\\/zone}").Output()
		// TODO, remove hard coded, default is server 2019
		windowsAMI := "ami-06d96a43543089121"
		if winVersion == "2004" {
			windowsAMI = "ami-0d93b5fd197b5d399"
		}
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<name>", machineSetName)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<infrastructureID>", infrastructureID)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<region>", region)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<zone>", zone)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<windows_image_with_container_runtime_installed>", windowsAMI)
		windowsMachineSetName = infrastructureID + "-" + machineSetName + "-worker-" + zone
	} else if iaasPlatform == "azure" {
		windowsMachineSet = getFileContent("winc", "azure_windows_machineset.yaml")
		infrastructureID, err = oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		location := "centralus"
		sku := "2019-Datacenter-with-Containers"
		if winVersion == "2004" {
			sku = "datacenter-core-2004-with-containers-smalldisk"
		}
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<infrastructureID>", infrastructureID)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<location>", location)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<SKU>", sku)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<name>", machineSetName)
		windowsMachineSetName = machineSetName
	} else {
		e2e.Failf("IAAS platform: %s is not automated yet", iaasPlatform)
	}
	ioutil.WriteFile("availWindowsMachineSet"+machineSetName, []byte(windowsMachineSet), 0644)
	return windowsMachineSetName, err
}

func createMachineset(oc *exutil.CLI, file string, machinesetName string) error {
	_, err := oc.WithoutNamespace().Run("create").Args("-f", file).Output()
	return err
}

func waitForMachinesetReady(oc *exutil.CLI, machinesetName string, deadTime int, expectedReplicas int) {
	pollErr := wait.Poll(15*time.Second, time.Duration(deadTime)*time.Minute, func() (bool, error) {
		msg, err := oc.WithoutNamespace().Run("get").Args("machineset", machinesetName, "-o=jsonpath={.status.readyReplicas}", "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if msg != strconv.Itoa(expectedReplicas) {
			e2e.Logf("Windows machine is not provisioned yet and waiting up to %v minutes ...", deadTime)
			return false, nil
		}
		e2e.Logf("Windows machine is provisioned")
		return true, nil
	})
	if pollErr != nil {
		e2e.Failf("Windows machine is not provisioned after waiting up to %v minutes ...", deadTime)
	}
}

func getNodeNameFromIP(oc *exutil.CLI, nodeIP string, iaasPlatform string) (string, error) {
	// Azure and AWS indexes for IP addresses are different
	index := "0"
	if iaasPlatform == "azure" {
		index = "1"
	}
	nodeName, err := oc.WithoutNamespace().Run("get").Args("node", "-o=jsonpath={.items[?(@.status.addresses["+index+"].address==\""+nodeIP+"\")].metadata.name}").Output()
	return nodeName, err
}

func createRuntimeClass(oc *exutil.CLI, runtimeClassFile, node string) error {
	runtimeClass := ""
	runtimeClass = getFileContent("winc", runtimeClassFile)
	buildID, err := getWindowsBuildID(oc, node)
	e2e.Logf("-------- Windows build ID is " + buildID + "-----------")
	runtimeClass = strings.ReplaceAll(runtimeClass, "<kernelID>", buildID)
	ioutil.WriteFile(runtimeClassFile, []byte(runtimeClass), 0644)
	_, err = oc.WithoutNamespace().Run("create").Args("-f", runtimeClassFile).Output()
	return err
}

func checkLBConnectivity(attempts int, externalIP string) bool {
	retcode := true
	for v := 1; v < attempts; v++ {
		e2e.Logf("Check the Load balancer cluster IP responding: " + externalIP)
		msg, _ := exec.Command("bash", "-c", "curl "+externalIP).Output()
		if !strings.Contains(string(msg), "Windows Container Web Server") {
			e2e.Logf("Windows Load balancer isn't working properly on the %v attempt", v)
			retcode = false
			break
		}
	}
	return retcode
}

func fetchAddress(oc *exutil.CLI, addressType string, machinesetName string) []string {

	if addressType == "dns" {
		addressType = "InternalDNS"
	} else {
		addressType = "InternalIP"
	}
	machineAddress := ""
	poolErr := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		var err error
		machineAddress, err = oc.WithoutNamespace().Run("get").Args("machine", "-ojsonpath={.items[?(@.metadata.labels.machine\\.openshift\\.io\\/cluster-api-machineset==\""+machinesetName+"\")].status.addresses[?(@.type==\""+addressType+"\")].address}", "-n", "openshift-machine-api").Output()
		if err != nil {
			e2e.Logf("trying next round")
			return false, nil
		}
		if machineAddress == "" {
			e2e.Logf("Did not get address, trying next round")
			return false, nil
		}
		return true, nil
	})
	if poolErr != nil {
		e2e.Failf("Failed to get address")
	}
	e2e.Logf("Machine Address is %v", machineAddress)
	return strings.Split(string(machineAddress), " ")
}

func setConfigmap(oc *exutil.CLI, address string, administrator string, configMapFile string) error {
	configmap := ""
	configmap = getFileContent("winc", configMapFile)
	configmap = strings.ReplaceAll(configmap, "<address>", address)
	configmap = strings.ReplaceAll(configmap, "<username>", administrator)
	ioutil.WriteFile("configMapFile", []byte(configmap), 0644)
	_, err := oc.WithoutNamespace().Run("create").Args("-f", "configMapFile").Output()
	return err
}
