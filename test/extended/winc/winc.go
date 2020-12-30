package winc

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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
		// privateKey = "~/.ssh/openshift-qe.pem"
		// publicKey  = "~/.ssh/openshift-qe.pub"
	)

	g.BeforeEach(func() {
		output, _ := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		iaasPlatform = strings.ToLower(output)

	})

	// author: sgao@redhat.com
	g.It("Critical-33612-Windows node basic check", func() {
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
		msg, err = oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath='{.items[0].metadata.annotations.windowsmachineconfig\\.openshift\\.io\\/version}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if msg == "" {
			e2e.Failf("Failed to check version annotation is applied to Windows node %s", msg)
		}

		g.By("Check kubelet service is running")
		bastionHost := getSSHBastionHost(oc)
		winInternalIP := getWindowsInternalIPs(oc)[0]
		msg, _ = runPSCommand(bastionHost, winInternalIP, "Get-Service kubelet", privateKey, iaasPlatform)
		if !strings.Contains(msg, "Running") {
			e2e.Failf("Failed to check kubelet service is running: %s", msg)
		}

		g.By("Check hybrid-overlay-node service is running")
		msg, _ = runPSCommand(bastionHost, winInternalIP, "Get-Service hybrid-overlay-node", privateKey, iaasPlatform)
		if !strings.Contains(msg, "Running") {
			e2e.Failf("Failed to check hybrid-overlay-node service is running: %s", msg)
		}
	})
	// author: sgao@redhat.com
	g.It("Critical-28423-Dockerfile prepare required binaries in operator image", func() {
		checkFolders := []struct {
			folder   string
			expected string
		}{
			{
				folder:   "/payload",
				expected: "cni hybrid-overlay-node.exe kube-node powershell wmcb.exe",
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
	g.It("Critical-32615-Generate userData secret [Serial]", func() {
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
	g.It("Low-32554-WMCO run in a pod with HostNetwork", func() {
		winInternalIP := getWindowsInternalIPs(oc)[0]
		curlDest := winInternalIP + ":22"
		command := []string{"exec", "-n", "openshift-windows-machine-config-operator", "deployment.apps/windows-machine-config-operator", "--", "curl", curlDest}
		msg, _ := oc.WithoutNamespace().Run(command...).Args().Output()
		if !strings.Contains(msg, "OpenSSH_for_Windows") {
			e2e.Failf("Failed to check WMCO run in a pod with HostNetwork: %s", msg)
		}
	})
	// author: sgao@redhat.com
	g.It("Critical-32856-WMCO watch machineset with Windows label", func() {
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
	g.It("Critical-29411-Reconcile Windows node [Serial]", func() {
		windowsMachineSetName := ""
		if iaasPlatform == "aws" {
			// TODO: Get Windows machineset via oc command
			infrastructureID, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
			zone := "us-east-2a"
			windowsMachineSetName = infrastructureID + "-windows-worker-" + zone
		} else if iaasPlatform == "azure" {
			windowsMachineSetName = "windows"
		} else {
			e2e.Failf("IAAS platform: %s is not automated yet", iaasPlatform)
		}
		g.By("Scale up the MachineSet")
		_, err := oc.WithoutNamespace().Run("scale").Args("--replicas=3", "machineset", windowsMachineSetName, "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Windows node is taking roughly 12 minutes to be shown up in the cluster, set timeout as 20 minutes
		pollErr := wait.Poll(60*time.Second, 1200*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "--no-headers").Output()
			nodes := strings.Split(msg, "\n")
			if len(nodes) != 3 {
				e2e.Logf("Expected 3 Windows nodes are not exist yet and waiting up to 20 minutes ...")
				return false, nil
			}
			if !(len(nodes) == 3 && strings.Contains(nodes[0], "Ready") && strings.Contains(nodes[1], "Ready")) {
				e2e.Logf("Expected 3 Windows nodes are exist but not ready yet and waiting up to 20 minutes ...")
				return false, nil
			}
			e2e.Logf("Expected 3 Windows nodes are ready")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Expected 3 Windows nodes are not ready after waiting up to 20 minutes ...")
		}

		g.By("Scale down the MachineSet")
		_, err = oc.WithoutNamespace().Run("scale").Args("--replicas=2", "machineset", windowsMachineSetName, "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pollErr = wait.Poll(10*time.Second, 150*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "--no-headers").Output()
			nodes := strings.Split(msg, "\n")
			if len(nodes) != 2 {
				e2e.Logf("Scale down to 2 Windows nodes is not ready yet and waiting up to 3 minutes ...")
				return false, nil
			}
			e2e.Logf("Scale down to 2 Windows nodes is ready")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Scale down to 2 Windows nodes is not ready after waiting up to 3 minutes ...")
		}
	})
	// author: sgao@redhat.com
	g.It("Critical-28632-Windows and Linux east-west network during a long time [Serial]", func() {
		// Note: Duplicate with Case 31276, run again in [Serial]
		if !checkWindowsWorkloadCreated(oc) {
			createWindowsWorkload(oc)
		}
		if !checkLinuxWorkloadCreated(oc) {
			createLinuxWorkload(oc)
		}
		g.By("Check communication: Windows pod <--> Linux pod")
		winpodNameIP, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector=app=win-webserver", "-o=jsonpath={.items[0].metadata.name}|{.items[0].status.podIP}", "-n", "winc-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxpodNameIP, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector=run=linux-webserver", "-o=jsonpath={.items[0].metadata.name}|{.items[0].status.podIP}", "-n", "winc-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		windows := strings.Split(winpodNameIP, "|")
		linux := strings.Split(linuxpodNameIP, "|")
		command := []string{"exec", "-n", "winc-test", windows[0], "--", "curl", linux[1] + ":8080"}
		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux web server from Windows pod")
		}
		command = []string{"exec", "-n", "winc-test", linux[0], "--", "curl", windows[1] + ":80"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Linux pod")
		}
	})
	// author: sgao@redhat.com
	g.It("Critical-32273-Configure kube-proxy and external networking check", func() {
		if !checkWindowsWorkloadCreated(oc) {
			createWindowsWorkload(oc)
		}
		externalIP := ""
		if iaasPlatform == "azure" {
			externalIP, _ = oc.WithoutNamespace().Run("get").Args("service", "win-webserver", "-o=jsonpath={.status.loadBalancer.ingress[0].ip}", "-n", "winc-test").Output()
		} else {
			externalIP, _ = oc.WithoutNamespace().Run("get").Args("service", "win-webserver", "-o=jsonpath={.status.loadBalancer.ingress[0].hostname}", "-n", "winc-test").Output()
		}
		externalIPURL := "http://" + externalIP + ":80"
		// Load balancer takes about 3 minutes to work, set timeout as 5 minutes
		pollErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
			msg, _ := exec.Command("bash", "-c", "curl "+externalIPURL).Output()
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
	g.It("Critical-31276-Configure CNI and internal networking check", func() {
		if !checkWindowsWorkloadCreated(oc) {
			createWindowsWorkload(oc)
		}
		if !checkLinuxWorkloadCreated(oc) {
			createLinuxWorkload(oc)
		}
		g.By("Check communication: Windows pod <--> Linux pod")
		winpodNameIP, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector=app=win-webserver", "-o=jsonpath={.items[0].metadata.name}|{.items[0].status.podIP}", "-n", "winc-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxpodNameIP, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector=run=linux-webserver", "-o=jsonpath={.items[0].metadata.name}|{.items[0].status.podIP}", "-n", "winc-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		windows := strings.Split(winpodNameIP, "|")
		linux := strings.Split(linuxpodNameIP, "|")
		command := []string{"exec", "-n", "winc-test", windows[0], "--", "curl", linux[1] + ":8080"}
		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux web server from Windows pod")
		}
		command = []string{"exec", "-n", "winc-test", linux[0], "--", "curl", windows[1] + ":80"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Linux pod")
		}

		g.By("Check communication: Windows pod <--> Windows pod in the same node")
		winpodNameIP, err = oc.WithoutNamespace().Run("get").Args("pod", "--selector=app=win-webserver", "--sort-by=.status.hostIP", "-o=jsonpath={.items[0].metadata.name}|{.items[0].status.podIP}|{.items[0].status.hostIP};{.items[1].metadata.name}|{.items[1].status.podIP}|{.items[1].status.hostIP}", "-n", "winc-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		winpod_1 := strings.Split(strings.Split(winpodNameIP, ";")[0], "|")
		winpod_2 := strings.Split(strings.Split(winpodNameIP, ";")[1], "|")
		if winpod_1[2] != winpod_2[2] {
			e2e.Failf("Failed to get Windows pod in the same node")
		}
		command = []string{"exec", "-n", "winc-test", winpod_1[0], "--", "curl", winpod_2[1] + ":80"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}
		command = []string{"exec", "-n", "winc-test", winpod_2[0], "--", "curl", winpod_1[1] + ":80"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}

		g.By("Check communication: Windows pod <--> Windows pod across different Windows nodes")
		winpodNameIP, err = oc.WithoutNamespace().Run("get").Args("pod", "--selector=app=win-webserver", "--sort-by=.status.hostIP", "-o=jsonpath={.items[0].metadata.name}|{.items[0].status.podIP}|{.items[0].status.hostIP};{.items[-1].metadata.name}|{.items[-1].status.podIP}|{.items[-1].status.hostIP}", "-n", "winc-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		windows_1 := strings.Split(strings.Split(winpodNameIP, ";")[0], "|")
		windows_2 := strings.Split(strings.Split(winpodNameIP, ";")[1], "|")
		if windows_1[2] == windows_2[2] {
			e2e.Failf("Failed to get Windows pod across different Windows nodes")
		}
		command = []string{"exec", "-n", "winc-test", windows_1[0], "--", "curl", windows_2[1] + ":80"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}
		command = []string{"exec", "-n", "winc-test", windows_2[0], "--", "curl", windows_1[1] + ":80"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}
	})
	g.It("Critical-25593-Prevent scheduling non-Windows workloads on Windows nodes", func() {
		g.By("Check Windows node have a taint 'os=Windows:NoSchedule'")
		msg, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath={.items[0].spec.taints[0].key}={.items[0].spec.taints[0].value}:{.items[0].spec.taints[0].effect}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if msg != "os=Windows:NoSchedule" {
			e2e.Failf("Failed to check Windows node have taint os=Windows:NoSchedule")
		}
		g.By("Check deployment without tolerations would not land on Windows nodes")
		defer oc.WithoutNamespace().Run("delete").Args("deployment", "win-webserver-no-taint", "-n", "winc-test").Output()
		oc.WithoutNamespace().Run("new-project").Args("winc-test").Output()
		windowsWebServerNoTaint := filepath.Join(exutil.FixturePath("testdata", "winc"), "windows_web_server_no_taint.yaml")
		oc.WithoutNamespace().Run("create").Args("-f", windowsWebServerNoTaint, "-n", "winc-test").Output()
		poolErr := wait.Poll(20*time.Second, 60*time.Second, func() (bool, error) {
			msg, err = oc.WithoutNamespace().Run("get").Args("pod", "--selector=app=win-webserver-no-taint", "-o=jsonpath={.items[].status.conditions[].message}", "-n", "winc-test").Output()
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
				if namespace != "" && namespace != "winc-test" {
					e2e.Failf("Failed to check none of core/optional operators/operands would land on Windows nodes")
				}
			}
		}
	})
})

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

func checkLinuxWorkloadCreated(oc *exutil.CLI) bool {
	msg, _ := oc.WithoutNamespace().Run("get").Args("deployment", "linux-webserver", "-o=jsonpath={.status.readyReplicas}", "-n", "winc-test").Output()
	if msg != "1" {
		e2e.Logf("Linux workload is not created yet")
		return false
	}
	return true
}

func createLinuxWorkload(oc *exutil.CLI) {
	oc.WithoutNamespace().Run("new-project").Args("winc-test").Output()
	linuxWebServer := filepath.Join(exutil.FixturePath("testdata", "winc"), "linux_web_server.yaml")
	// Wait up to 3 minutes for Linux workload ready
	oc.WithoutNamespace().Run("create").Args("-f", linuxWebServer, "-n", "winc-test").Output()
	poolErr := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		return checkLinuxWorkloadCreated(oc), nil
	})
	if poolErr != nil {
		e2e.Failf("Linux workload is not ready after waiting up to 3 minutes ...")
	}
}

func checkWindowsWorkloadCreated(oc *exutil.CLI) bool {
	msg, _ := oc.WithoutNamespace().Run("get").Args("deployment", "win-webserver", "-o=jsonpath={.status.readyReplicas}", "-n", "winc-test").Output()
	if msg != "5" {
		e2e.Logf("Windows workload is not created yet")
		return false
	}
	return true
}

func createWindowsWorkload(oc *exutil.CLI) {
	oc.WithoutNamespace().Run("new-project").Args("winc-test").Output()
	windowsWebServer := filepath.Join(exutil.FixturePath("testdata", "winc"), "windows_web_server.yaml")
	oc.WithoutNamespace().Run("create").Args("-f", windowsWebServer, "-n", "winc-test").Output()
	// Wait up to 5 minutes for Windows workload ready
	poolErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
		return checkWindowsWorkloadCreated(oc), nil
	})
	if poolErr != nil {
		e2e.Failf("Windows workload is not ready after waiting up to 5 minutes ...")
	}
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
