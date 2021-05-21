package winc

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-windows] Windows_Containers", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLIWithoutNamespace("default")
		iaasPlatform string
		privateKey   = "../internal/config/keys/openshift-qe.pem"
		publicKey    = "../internal/config/keys/openshift-qe.pub"
	)

	g.BeforeEach(func() {
		output, _ := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		iaasPlatform = strings.ToLower(output)
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-33612-Windows node basic check", func() {
		g.By("Check Windows worker nodes run the same kubelet version as other Linux worker nodes")
		linuxKubeletVersion, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=linux", "-o=jsonpath={.items[0].status.nodeInfo.kubeletVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		windowsKubeletVersion, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath={.items[0].status.nodeInfo.kubeletVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if windowsKubeletVersion[0:5] != linuxKubeletVersion[0:5] {
			e2e.Failf("Failed to check Windows %s and Linux %s kubelet version should be the same", windowsKubeletVersion, linuxKubeletVersion)
		}

		g.By("Check worker label is applied to Windows node")
		msg, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "worker") {
			e2e.Failf("Failed to check worker label is applied to Windows node %s", msg)
		}

		g.By("Check version annotation is applied to Windows node")
		// Note: Case 33536 also is covered
		windowsHostName := getWindowsHostNames(oc)[0]
		if !checkVersionAnnotationReady(oc, windowsHostName) {
			e2e.Failf("Failed to check version annotation is applied to Windows node %s", msg)
		}

		bastionHost := getSSHBastionHost(oc)
		winInternalIP := getWindowsInternalIPs(oc)[0]

		g.By("Check windows_exporter service is running")
		msg, _ = runPSCommand(bastionHost, winInternalIP, "Get-Service windows_exporter", privateKey, iaasPlatform)
		if !strings.Contains(msg, "Running") {
			e2e.Failf("Failed to check windows_exporter service is running: %s", msg)
		}

		g.By("Check kubelet service is running")
		msg, _ = runPSCommand(bastionHost, winInternalIP, "Get-Service kubelet", privateKey, iaasPlatform)
		if !strings.Contains(msg, "Running") {
			e2e.Failf("Failed to check kubelet service is running: %s", msg)
		}

		g.By("Check hybrid-overlay-node service is running")
		msg, _ = runPSCommand(bastionHost, winInternalIP, "Get-Service hybrid-overlay-node", privateKey, iaasPlatform)
		if !strings.Contains(msg, "Running") {
			e2e.Failf("Failed to check hybrid-overlay-node service is running: %s", msg)
		}

		g.By("Check kube-proxy service is running")
		msg, _ = runPSCommand(bastionHost, winInternalIP, "Get-Service kube-proxy", privateKey, iaasPlatform)
		if !strings.Contains(msg, "Running") {
			e2e.Failf("Failed to check kube-proxy service is running: %s", msg)
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-28423-Dockerfile prepare required binaries in operator image", func() {
		checkFolders := []struct {
			folder   string
			expected string
		}{
			{
				folder:   "/payload",
				expected: "cni hybrid-overlay-node.exe kube-node powershell windows_exporter.exe wmcb.exe",
			},
			{
				folder:   "/payload/cni",
				expected: "cni-conf-template.json flannel.exe host-local.exe win-bridge.exe win-overlay.exe",
			},
			{
				folder:   "/payload/kube-node",
				expected: "kube-proxy.exe kubelet.exe",
			},
			{
				folder:   "/payload/powershell",
				expected: "hns.psm1 wget-ignore-cert.ps1",
			},
		}
		for _, checkFolder := range checkFolders {
			g.By("Check required files in" + checkFolder.folder)
			command := []string{"exec", "-n", "openshift-windows-machine-config-operator", "deployment.apps/windows-machine-config-operator", "--", "ls", checkFolder.folder}
			msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			actual := strings.ReplaceAll(msg, "\n", " ")
			if actual != checkFolder.expected {
				e2e.Failf("Failed to check required files in /payload, expected: %s actual: %s", checkFolder.expected, actual)
			}
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-32615-Generate userData secret [Serial]", func() {
		g.By("Check secret windows-user-data generated and contain correct public key")
		output, err := exec.Command("bash", "-c", "cat "+publicKey+"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		publicKeyContent := strings.Split(string(output), " ")[1]
		msg, err := oc.WithoutNamespace().Run("get").Args("secret", "windows-user-data", "-n", "openshift-machine-api", "-o=jsonpath={.data.userData}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		decodedUserData, _ := base64.StdEncoding.DecodeString(msg)
		if !strings.Contains(string(decodedUserData), publicKeyContent) {
			e2e.Failf("Failed to check public key in windows-user-data secret %s", string(decodedUserData))
		}
		g.By("Check delete secret windows-user-data")
		// May fail other cases in parallel, so run it in serial
		_, err = oc.WithoutNamespace().Run("delete").Args("secret", "windows-user-data", "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pollErr := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			msg, _ := oc.WithoutNamespace().Run("get").Args("secret", "windows-user-data", "-n", "openshift-machine-api").Output()
			if !strings.Contains(msg, "windows-user-data") {
				e2e.Logf("Secret windows-user-data does not exist yet and wait up to 1 minute ...")
				return false, nil
			}
			e2e.Logf("Secret windows-user-data exist now")
			msg, err := oc.WithoutNamespace().Run("get").Args("secret", "windows-user-data", "-o=jsonpath={.data.userData}", "-n", "openshift-machine-api").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			decodedUserData, _ := base64.StdEncoding.DecodeString(msg)
			if !strings.Contains(string(decodedUserData), publicKeyContent) {
				e2e.Failf("Failed to check public key in windows-user-data secret %s", string(decodedUserData))
			}
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Secret windows-user-data does not exist after waiting up to 1 minutes ...")
		}
		g.By("Check update secret windows-user-data")
		// May fail other cases in parallel, so run it in serial
		// Update userData to "aW52YWxpZAo=" which is base64 encoded "invalid"
		msg, err = oc.WithoutNamespace().Run("patch").Args("secret", "windows-user-data", "-p", `{"data":{"userData":"aW52YWxpZAo="}}`, "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pollErr = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			msg, err := oc.WithoutNamespace().Run("get").Args("secret", "windows-user-data", "-o=jsonpath={.data.userData}", "-n", "openshift-machine-api").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			decodedUserData, _ := base64.StdEncoding.DecodeString(msg)
			if !strings.Contains(string(decodedUserData), publicKeyContent) {
				e2e.Logf("Secret windows-user-data is not updated yet and wait up to 1 minute ...")
				return false, nil
			}
			e2e.Logf("Secret windows-user-data is updated")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Secret windows-user-data is not updated after waiting up to 1 minutes ...")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Low-32554-WMCO run in a pod with HostNetwork", func() {
		winInternalIP := getWindowsInternalIPs(oc)[0]
		curlDest := winInternalIP + ":22"
		command := []string{"exec", "-n", "openshift-windows-machine-config-operator", "deployment.apps/windows-machine-config-operator", "--", "curl", curlDest}
		msg, _ := oc.WithoutNamespace().Run(command...).Args().Output()
		if !strings.Contains(msg, "SSH-2.0-OpenSSH") {
			e2e.Failf("Failed to check WMCO run in a pod with HostNetwork: %s", msg)
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-32856-WMCO watch machineset with Windows label", func() {
		// Note: Create machineset with Windows label covered in Flexy post action
		g.By("Check create machineset without Windows label")
		windowsMachineSetName := ""
		if iaasPlatform == "aws" {
			windowsMachineSet := getFileContent("winc", "aws_windows_machineset_no_label.yaml")
			infrastructureID, err := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			region, err := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			// TODO, remove hard code
			zone := "us-east-2a"
			// TODO, remove hard code
			windowsAMI := "ami-0c7a9c9d17f8a5b64"
			windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<infrastructureID>", infrastructureID)
			windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<region>", region)
			windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<zone>", zone)
			windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<windows_image_with_container_runtime_installed>", windowsAMI)
			ioutil.WriteFile("availWindowsMachineSetWithoutLabel", []byte(windowsMachineSet), 0644)
			windowsMachineSetName = infrastructureID + "-windows-without-label-worker-" + zone
		} else if iaasPlatform == "azure" {
			windowsMachineSet := getFileContent("winc", "azure_windows_machineset_no_label.yaml")
			infrastructureID, err := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			region := "northcentralus"
			windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<infrastructureID>", infrastructureID)
			windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<region>", region)
			ioutil.WriteFile("availWindowsMachineSetWithoutLabel", []byte(windowsMachineSet), 0644)
			windowsMachineSetName = "win-nol"
		} else {
			e2e.Failf("IAAS platform: %s is not automated yet", iaasPlatform)
		}

		// Make sure Windows machineset without label deleted
		defer oc.WithoutNamespace().Run("delete").Args("machineset", windowsMachineSetName, "-n", "openshift-machine-api").Output()
		_, err := oc.WithoutNamespace().Run("create").Args("-f", "availWindowsMachineSetWithoutLabel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Wait up to 3 minutes for Windows machine to be "Provisioned"
		pollErr := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			msg, _ := oc.WithoutNamespace().Run("get").Args("machineset", windowsMachineSetName, "-o=jsonpath={.status.observedGeneration}", "-n", "openshift-machine-api").Output()
			if msg != "1" {
				e2e.Logf("Windows machine is not provisioned yet and waiting up to 3 minutes ...")
				return false, nil
			}
			e2e.Logf("Windows machine is provisioned")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Windows machine is not provisioned after waiting up to 3 minutes ...")
		}
		// WMCO should NOT watch machines created without Windows label
		msg, err := oc.WithoutNamespace().Run("logs").Args("deployment.apps/windows-machine-config-operator", "-n", "openshift-windows-machine-config-operator").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(msg, windowsMachineSetName) {
			e2e.Failf("Failed to check create machineset without Windows label")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-High-29411-Reconcile Windows node [Slow][Disruptive]", func() {
		windowsMachineSetName := getWindowsMachineSetName(oc)
		g.By("Scale up the MachineSet")
		scaleWindowsMachineSet(oc, windowsMachineSetName, 3)
		defer scaleWindowsMachineSet(oc, windowsMachineSetName, 2)
		// Windows node is taking roughly 12 minutes to be shown up in the cluster, set timeout as 20 minutes
		waitWindowsNodesReady(oc, 3, 60*time.Second, 1200*time.Second)
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-28632-Windows and Linux east west network during a long time [Serial]", func() {
		// Note: Duplicate with Case 31276, run again in [Serial]
		namespace := "winc-28632"
		createWindowsWorkload(oc, namespace)
		createLinuxWorkload(oc, namespace)
		defer deleteProject(oc, namespace)

		g.By("Check communication: Windows pod <--> Linux pod")
		winPodNames := getWorkloadsNames(oc, "windows", namespace)
		windPodIPs := getWorkloadsIP(oc, "windows", namespace)
		linuxPodNames := getWorkloadsNames(oc, "linux", namespace)
		linuxPodIPs := getWorkloadsIP(oc, "linux", namespace)
		command := []string{"exec", "-n", namespace, winPodNames[0], "--", "curl", linuxPodIPs[0] + ":8080"}
		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux web server from Windows pod")
		}
		command = []string{"exec", "-n", namespace, linuxPodNames[0], "--", "curl", windPodIPs[0]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Linux pod")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-32273-Configure kube proxy and external networking check", func() {
		namespace := "winc-32273"
		createWindowsWorkload(oc, namespace)
		defer deleteProject(oc, namespace)
		externalIP := getExternalIP(iaasPlatform, oc, "windows", namespace)
		// Load balancer takes about 3 minutes to work, set timeout as 5 minutes
		pollErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
			msg, _ := exec.Command("bash", "-c", "curl "+externalIP).Output()
			if !strings.Contains(string(msg), "Windows Container Web Server") {
				e2e.Logf("Load balancer is not ready yet and waiting up to 5 minutes ...")
				return false, nil
			}
			e2e.Logf("Load balancer is ready")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Load balancer is not ready after waiting up to 5 minutes ...")
		}
	})

	// author rrasouli@redhat.com
	g.It("Author:rrasouli-High-39451-Access Windows workload through clusterIP [Slow][Disruptive]", func() {
		namespace := "winc-39451"
		createWindowsWorkload(oc, namespace)
		createLinuxWorkload(oc, namespace)
		defer deleteProject(oc, namespace)

		g.By("Check access through clusterIP from Linux and Windows pods")
		windowsClusterIP := getServiceClusterIP(oc, "windows", namespace)
		linuxClusterIP := getServiceClusterIP(oc, "linux", namespace)
		winPodArray := getWorkloadsNames(oc, "windows", namespace)
		linuxPodArray := getWorkloadsNames(oc, "linux", namespace)
		e2e.Logf("windows cluster IP: " + windowsClusterIP)
		e2e.Logf("Linux cluster IP: " + linuxClusterIP)

		//we query the Linux ClusterIP by a windows pod
		command := []string{"exec", "-n", namespace, winPodArray[0], "--", "curl", linuxClusterIP + ":8080"}

		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}
		e2e.Logf("***** Now testing Windows node from a linux host : ****")
		command = []string{"exec", "-n", namespace, linuxPodArray[0], "--", "curl", windowsClusterIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows ClusterIP from a linux pod")
		}
		e2e.Logf(" **** Success in testing Windows node from a linux host :-) ****")

		g.By("Scale up the Deployment Windows pod continue to be available to curl Linux web server")
		e2e.Logf("Scalling up the Deployment to 2")
		scaleDeployment(oc, "windows", 2, namespace)

		o.Expect(err).NotTo(o.HaveOccurred())
		winPodArray = getWorkloadsNames(oc, "windows", namespace)
		command = []string{"exec", "-n", namespace, linuxPodArray[0], "--", "curl", windowsClusterIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows ClusterIP from a Linux pod")
		}

		command = []string{"exec", "-n", namespace, winPodArray[1], "--", "curl", linuxClusterIP + ":8080"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}

		g.By("Scale up the MachineSet")
		e2e.Logf("Scalling up the Windows node to 3")
		windowsMachineSetName := getWindowsMachineSetName(oc)
		scaleWindowsMachineSet(oc, windowsMachineSetName, 3)
		defer scaleWindowsMachineSet(oc, windowsMachineSetName, 2)
		waitWindowsNodesReady(oc, 3, 60*time.Second, 1200*time.Second)
		// Testing the Windows server is reachable via Linux pod
		command = []string{"exec", "-n", namespace, linuxPodArray[0], "--", "curl", windowsClusterIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows ClusterIP from a Linux pod")
		}
		// Testing the Linux server is reachable with the second windows pod created
		command = []string{"exec", "-n", namespace, winPodArray[1], "--", "curl", linuxClusterIP + ":8080"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}
		// Testing the Linux server is reachable with the first Windows pod created.
		command = []string{"exec", "-n", namespace, winPodArray[0], "--", "curl", linuxClusterIP + ":8080"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-31276-Configure CNI and internal networking check", func() {
		namespace := "winc-31276"
		createWindowsWorkload(oc, namespace)
		createLinuxWorkload(oc, namespace)
		defer deleteProject(oc, namespace)
		g.By("Check communication: Windows pod <--> Linux pod")
		winPodNameArray := getWorkloadsNames(oc, "windows", namespace)
		linuxPodNameArray := getWorkloadsNames(oc, "linux", namespace)
		winPodIPArray := getWorkloadsIP(oc, "windows", namespace)
		linuxPodIPArray := getWorkloadsIP(oc, "linux", namespace)
		command := []string{"exec", "-n", namespace, linuxPodNameArray[0], "--", "curl", winPodIPArray[0]}
		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Linux pod")
		}

		linuxSVC := linuxPodIPArray[0] + ":8080"
		command = []string{"exec", "-n", namespace, winPodNameArray[0], "--", "curl", linuxSVC}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux web server from Windows pod")
		}

		g.By("Check communication: Windows pod <--> Windows pod in the same node")
		// we scale the deployment to 5 windows pods
		scaleDeployment(oc, "windows", 5, namespace)
		hostIPArray := getWorkloadsHostIP(oc, "windows", namespace)
		if hostIPArray[0] != hostIPArray[1] {
			e2e.Failf("Failed to get Windows pod in the same node")
		}
		podNameArray := getWorkloadsNames(oc, "windows", namespace)
		podIPArray := getWorkloadsIP(oc, "windows", namespace)
		command = []string{"exec", "-n", namespace, podNameArray[0], "--", "curl", podIPArray[0]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}
		command = []string{"exec", "-n", namespace, podNameArray[0], "--", "curl", podIPArray[1]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}

		g.By("Check communication: Windows pod <--> Windows pod across different Windows nodes")
		lastHostIP := hostIPArray[len(hostIPArray)-1]
		if hostIPArray[0] == lastHostIP {
			e2e.Failf("Failed to get Windows pod across different Windows nodes")
		}
		lastIP := winPodIPArray[len(winPodIPArray)-1]
		command = []string{"exec", "-n", namespace, winPodNameArray[0], "--", "curl", lastIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}
		lastPodName := winPodNameArray[len(winPodNameArray)-1]
		command = []string{"exec", "-n", namespace, lastPodName, "--", "curl", winPodIPArray[0]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Medium-33768-NodeWithoutOVNKubeNodePodRunning alert ignore Windows nodes", func() {
		g.By("Check NodeWithoutOVNKubeNodePodRunning alert ignore Windows nodes")
		portForwardCMD := "nohup oc -n openshift-monitoring port-forward svc/prometheus-operated 9090  >/dev/null 2>&1 &"
		killPortForwardCMD := "killall oc"
		exec.Command("bash", "-c", portForwardCMD).Output()
		defer exec.Command("bash", "-c", killPortForwardCMD).Output()
		getAlertCMD := "sleep 5; curl -s http://localhost:9090/api/v1/rules | jq '[.data.groups[].rules[] | select(.name==\"NodeWithoutOVNKubeNodePodRunning\")]'"
		msg, _ := exec.Command("bash", "-c", getAlertCMD).Output()
		if !strings.Contains(string(msg), "kube_node_labels{label_kubernetes_io_os=\\\"windows\\\"}") {
			e2e.Failf("Failed to check NodeWithoutOVNKubeNodePodRunning alert ignore Windows nodes")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-33779-Retrieving Windows node logs", func() {
		g.By("Check a cluster-admin can retrieve kubelet logs")
		msg, err := oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=kubelet/kubelet.log").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		windowsHostNames := getWindowsHostNames(oc)
		for _, winHostName := range windowsHostNames {
			e2e.Logf("Retrieve kubelet log on: " + winHostName)
			if !strings.Contains(string(msg), winHostName+" Log file created at:") {
				e2e.Failf("Failed to retrieve kubelet log on: " + winHostName)
			}
		}

		g.By("Check a cluster-admin can retrieve kube-proxy logs")
		msg, err = oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=kube-proxy/kube-proxy.exe.WARNING").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, winHostName := range windowsHostNames {
			e2e.Logf("Retrieve kube-proxy log on: " + winHostName)
			if !strings.Contains(string(msg), winHostName+" Log file created at:") {
				e2e.Failf("Failed to retrieve kube-proxy log on: " + winHostName)
			}
		}

		g.By("Check a cluster-admin can retrieve hybrid-overlay logs")
		msg, err = oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=hybrid-overlay/hybrid-overlay.log").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, winHostName := range windowsHostNames {
			e2e.Logf("Retrieve hybrid-overlay log on: " + winHostName)
			if !strings.Contains(string(msg), winHostName) {
				e2e.Failf("Failed to retrieve hybrid-overlay log on: " + winHostName)
			}
		}

		g.By("Check a cluster-admin can retrieve container runtime logs")
		msg, err = oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=journal", "-u=docker").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Retrieve container runtime logs")
		if !strings.Contains(string(msg), "Starting up") {
			e2e.Failf("Failed to retrieve container runtime logs")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-33783-Enable must gather on Windows node [Slow][Disruptive]", func() {
		g.By("Check must-gather on Windows node")
		// Note: Marked as [Disruptive] in case of /tmp folder full
		msg, err := oc.WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir=/tmp/must-gather-33783").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/must-gather-33783").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mustGather := string(msg)
		checkMessage := []string{
			"host_service_logs/windows/",
			"host_service_logs/windows/log_files/",
			"host_service_logs/windows/log_files/hybrid-overlay/",
			"host_service_logs/windows/log_files/hybrid-overlay/hybrid-overlay.log",
			"host_service_logs/windows/log_files/kube-proxy/",
			"host_service_logs/windows/log_files/kube-proxy/kube-proxy.exe.ERROR",
			"host_service_logs/windows/log_files/kube-proxy/kube-proxy.exe.INFO",
			"host_service_logs/windows/log_files/kube-proxy/kube-proxy.exe.WARNING",
			"host_service_logs/windows/log_files/kubelet/",
			"host_service_logs/windows/log_files/kubelet/kubelet.log",
			"host_service_logs/windows/log_winevent/",
			"host_service_logs/windows/log_winevent/docker_winevent.log",
		}
		for _, v := range checkMessage {
			if !strings.Contains(mustGather, v) {
				e2e.Failf("Failed to check must-gather on Windows node: " + v)
			}
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-High-33794-Watch cloud private key secret [Slow][Disruptive]", func() {
		g.By("Check watch cloud-private-key secret")
		oc.WithoutNamespace().Run("delete").Args("secret", "cloud-private-key", "-n", "openshift-windows-machine-config-operator").Output()
		defer oc.WithoutNamespace().Run("create").Args("secret", "generic", "cloud-private-key", "--from-file=private-key.pem="+privateKey, "-n", "openshift-windows-machine-config-operator").Output()
		oc.WithoutNamespace().Run("delete").Args("secret", "windows-user-data", "-n", "openshift-machine-api").Output()

		windowsMachineSetName := getWindowsMachineSetName(oc)
		scaleWindowsMachineSet(oc, windowsMachineSetName, 3)
		defer scaleWindowsMachineSet(oc, windowsMachineSetName, 2)

		g.By("Check Windows machine should be in Provisioning phase and not reconciled")
		pollErr := wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
			msg, _ := oc.WithoutNamespace().Run("get").Args("events", "-n", "openshift-machine-api").Output()
			if strings.Contains(msg, "Secret \"windows-user-data\" not found") {
				return true, nil
			}
			return false, nil
		})
		if pollErr != nil {
			e2e.Failf("Failed to check Windows machine should be in Provisioning phase and not reconciled after waiting up to 5 minutes ...")
		}

		oc.WithoutNamespace().Run("create").Args("secret", "generic", "cloud-private-key", "--from-file=private-key.pem="+privateKey, "-n", "openshift-windows-machine-config-operator").Output()
		waitWindowsNodesReady(oc, 3, 60*time.Second, 1200*time.Second)
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Medium-37472-Idempotent check of service running in Windows node [Slow][Disruptive]", func() {
		namespace := "winc-37472"
		createWindowsWorkload(oc, namespace)
		defer deleteProject(oc, namespace)
		windowsHostName := getWindowsHostNames(oc)[0]
		oc.WithoutNamespace().Run("annotate").Args("node", windowsHostName, "windowsmachineconfig.openshift.io/version-").Output()

		g.By("Check after reconciled Windows node should be Ready")
		waitVersionAnnotationReady(oc, windowsHostName, 60*time.Second, 1200*time.Second)

		g.By("Check LB service works well")
		externalIP := getExternalIP(iaasPlatform, oc, "windows", namespace)
		// Load balancer takes about 3 minutes to work, set timeout as 5 minutes
		pollErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
			msg, _ := exec.Command("bash", "-c", "curl "+externalIP).Output()
			if !strings.Contains(string(msg), "Windows Container Web Server") {
				e2e.Logf("Load balancer is not ready yet and waiting up to 5 minutes ...")
				return false, nil
			}
			e2e.Logf("Load balancer is ready")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Load balancer is not ready after waiting up to 5 minutes ...")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Medium-39030-Re queue on Windows machine's edge cases [Slow][Disruptive]", func() {
		g.By("Scale down WMCO")
		_, err := oc.WithoutNamespace().Run("scale").Args("--replicas=0", "deployment", "windows-machine-config-operator", "-n", "openshift-windows-machine-config-operator").Output()
		defer restoreWMCODeployment(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Scale up the MachineSet")
		windowsMachineSetName := getWindowsMachineSetName(oc)
		scaleWindowsMachineSet(oc, windowsMachineSetName, 3)
		defer scaleWindowsMachineSet(oc, windowsMachineSetName, 2)

		g.By("Scale up WMCO")
		restoreWMCODeployment(oc)

		g.By("Check Windows machines created before WMCO starts are successfully reconciling and Windows nodes added")
		waitWindowsNodesReady(oc, 3, 60*time.Second, 1200*time.Second)
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Critical-25593-Prevent scheduling non Windows workloads on Windows nodes", func() {
		namespace := "winc-25593"
		g.By("Check Windows node have a taint 'os=Windows:NoSchedule'")
		msg, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath={.items[0].spec.taints[0].key}={.items[0].spec.taints[0].value}:{.items[0].spec.taints[0].effect}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if msg != "os=Windows:NoSchedule" {
			e2e.Failf("Failed to check Windows node have taint os=Windows:NoSchedule")
		}
		g.By("Check deployment without tolerations would not land on Windows nodes")
		defer deleteProject(oc, namespace)
		oc.WithoutNamespace().Run("new-project").Args(namespace).Output()
		windowsWebServerNoTaint := filepath.Join(exutil.FixturePath("testdata", "winc"), "windows_web_server_no_taint.yaml")
		oc.WithoutNamespace().Run("create").Args("-f", windowsWebServerNoTaint, "-n", namespace).Output()
		poolErr := wait.Poll(20*time.Second, 60*time.Second, func() (bool, error) {
			msg, err = oc.WithoutNamespace().Run("get").Args("pod", "--selector=app=win-webserver-no-taint", "-o=jsonpath={.items[].status.conditions[].message}", "-n", namespace).Output()
			if strings.Contains(msg, "didn't tolerate") {
				return true, nil
			}
			return false, nil
		})
		if poolErr != nil {
			e2e.Failf("Failed to check deployment without tolerations would not land on Windows nodes")
		}
		g.By("Check deployment with tolerations already covered in function createWindowsWorkload()")
		g.By("Check none of core/optional operators/operands would land on Windows nodes")
		for _, winHostName := range getWindowsHostNames(oc) {
			e2e.Logf("Check pods running on Windows node: " + winHostName)
			msg, err = oc.WithoutNamespace().Run("get").Args("pods", "--all-namespaces", "-o=jsonpath={.items[*].metadata.namespace}", "--field-selector", "spec.nodeName="+winHostName, "--no-headers").Output()
			for _, namespace := range strings.Split(msg, " ") {
				e2e.Logf("Found pods running in namespace: " + namespace)
				if namespace != "" && !strings.Contains(namespace, "winc") {
					e2e.Failf("Failed to check none of core/optional operators/operands would land on Windows nodes")
				}
			}
		}
	})
})

func restoreWMCODeployment(oc *exutil.CLI) {
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas=1", "deployment", "windows-machine-config-operator", "-n", "openshift-windows-machine-config-operator").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	// TODO: check WMCO is ready
}

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

func waitVersionAnnotationReady(oc *exutil.CLI, windowsNodeName string, interval time.Duration, timeout time.Duration) {
	pollErr := wait.Poll(interval, timeout, func() (bool, error) {
		if !checkVersionAnnotationReady(oc, windowsNodeName) {
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

func checkVersionAnnotationReady(oc *exutil.CLI, windowsNodeName string) bool {
	msg, err := oc.WithoutNamespace().Run("get").Args("nodes", windowsNodeName, "-o=jsonpath='{.metadata.annotations.windowsmachineconfig\\.openshift\\.io\\/version}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if msg == "" {
		return false
	}
	return true
}

func getWindowsMachineSetName(oc *exutil.CLI) string {
	getCMD := "oc get machineset -ojson -n openshift-machine-api | jq -r '.items[] | select(.spec.template.metadata.labels[]==\"Windows\") | .spec.template.metadata.labels.\"machine.openshift.io/cluster-api-machineset\"'"
	windowsMachineSetName, _ := exec.Command("bash", "-c", getCMD).Output()
	return strings.TrimSuffix(string(windowsMachineSetName), "\n")
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

func runPSCommand(bastionHost string, windowsHost string, command string, privateKey string, iaasPlatform string) (resut string, err error) {
	windowsUser := ""
	if iaasPlatform == "azure" {
		windowsUser = "capi"
	} else {
		windowsUser = "Administrator"
	}
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

func checkWindowsWorkloadCreated(oc *exutil.CLI, namespace string) bool {
	msg, _ := oc.WithoutNamespace().Run("get").Args("deployment", "win-webserver", "-o=jsonpath={.status.readyReplicas}", "-n", namespace).Output()
	if msg != "1" {
		e2e.Logf("Windows workload is not created yet")
		return false
	}
	return true
}

func checkWindowsWorkloadScaled(oc *exutil.CLI, namespace string, replicas int) bool {
	msg, _ := oc.WithoutNamespace().Run("get").Args("deployment", "win-webserver", "-o=jsonpath={.status.readyReplicas}", "-n", namespace).Output()
	workloads := strconv.Itoa(replicas)
	if msg != workloads {
		e2e.Logf("Windows workload did not scaled to " + workloads)
		return false
	}
	return true
}

func createWindowsWorkload(oc *exutil.CLI, namespace string) {
	oc.WithoutNamespace().Run("new-project").Args(namespace).Output()
	msg, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath={.items[0].metadata.labels.node\\.kubernetes\\.io\\/windows-build}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	windows_docker_image := "mcr.microsoft.com/windows/servercore:ltsc2019"
	if msg == "10.0.18363" {
		windows_docker_image = "mcr.microsoft.com/windows/servercore:1909"
	}
	windowsWebServer := getFileContent("winc", "windows_web_server.yaml")
	windowsWebServer = strings.ReplaceAll(windowsWebServer, "<windows_docker_image>", windows_docker_image)
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

// A private function to translate the workload/pod/deployment name
func getWorkloadName(os string) string {
	name := ""
	if os == "windows" {
		name = "win-webserver"
	} else {
		name = "linux-webserver"
	}
	return name
}

// Get an external IP of loadbalancer service
func getExternalIP(iaasPlatform string, oc *exutil.CLI, os string, namespace string) string {
	extIP := ""
	serviceName := getWorkloadName(os)
	if iaasPlatform == "azure" {
		extIP, _ = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.status.loadBalancer.ingress[0].ip}", "-n", namespace).Output()
	} else {
		extIP, _ = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.status.loadBalancer.ingress[0].hostname}", "-n", namespace).Output()
	}
	return extIP
}

// we retrieve the ClusterIP from a pod according to it's OS
func getServiceClusterIP(oc *exutil.CLI, os string, namespace string) string {
	clusterIP := ""
	serviceName := getWorkloadName(os)
	clusterIP, _ = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.spec.clusterIP}", "-n", namespace).Output()
	return clusterIP
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
func scaleDeployment(oc *exutil.CLI, os string, replicas int, namespace string) {
	deploymentName := getWorkloadName(os)
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), "deployment", deploymentName, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	poolErr := wait.Poll(60*time.Second, 300*time.Second, func() (bool, error) {
		return checkWindowsWorkloadScaled(oc, namespace, replicas), nil
	})
	if poolErr != nil {
		e2e.Failf("Windows workload did not scaled after waiting up to 5 minutes ...")
	}
}

func scaleWindowsMachineSet(oc *exutil.CLI, windowsMachineSetName string, replicas int) {
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), "machineset", windowsMachineSetName, "-n", "openshift-machine-api").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

}

// this function delete a workspace, we intend to do it after each test case run
func deleteProject(oc *exutil.CLI, namespace string) {
	_, err := oc.WithoutNamespace().Run("delete").Args("project", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}
func getWorkloadSelector(os string) string {
	selector := "app"
	if os == "linux" {
		selector = "run"
	}
	return selector
}

// this function returns an array of workloads names by their OS type
func getWorkloadsNames(oc *exutil.CLI, os string, namespace string) []string {
	workloadName := getWorkloadName(os)
	selector := getWorkloadSelector(os)

	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector="+selector+"="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].metadata.name}", "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	pods := strings.Split(workloads, " ")
	return pods
}

// this function returns an array of workloads IP's by their OS type
func getWorkloadsIP(oc *exutil.CLI, os string, namespace string) []string {
	workloadName := getWorkloadName(os)
	selector := getWorkloadSelector(os)

	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector="+selector+"="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].status.podIP}", "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	ips := strings.Split(workloads, " ")
	return ips
}

// this function returns an array of workloads IP's by their OS type
func getWorkloadsHostIP(oc *exutil.CLI, os string, namespace string) []string {
	workloadName := getWorkloadName(os)
	selector := getWorkloadSelector(os)

	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector="+selector+"="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].status.hostIP}", "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	ips := strings.Split(workloads, " ")
	return ips
}
