package mco

import (
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-mco] MCO", func() {

	var oc = exutil.NewCLI("mco", exutil.KubeConfigPath())

	g.It("Author:rioliu-Critical-42347-health check for machine-config-operator [Serial]", func() {
		g.By("checking mco status")

		coStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/machine-config", "-o", "jsonpath='{range .status.conditions[*]}{.type}{.status}{\"\\n\"}{end}'").Output()
		e2e.Logf(coStatus)

		o.Expect(coStatus).Should(o.ContainSubstring("ProgressingFalse"))
		o.Expect(coStatus).Should(o.ContainSubstring("UpgradeableTrue"))
		o.Expect(coStatus).Should(o.ContainSubstring("DegradedFalse"))
		o.Expect(coStatus).Should(o.ContainSubstring("AvailableTrue"))

		e2e.Logf("mco operator is healthy")

		g.By("checking mco pod status")

		podStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-machine-config-operator", "-o", "jsonpath='{.items[*].status.conditions[?(@.type==\"Ready\")].status}'").Output()
		e2e.Logf(podStatus)

		o.Expect(podStatus).ShouldNot(o.ContainSubstring("False"))

		e2e.Logf("mco pods are healthy")

		g.By("checking mcp status")

		mcpStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "-o", "jsonpath='{.items[*].status.conditions[?(@.type==\"Degraded\")].status}'").Output()
		e2e.Logf(mcpStatus)

		o.Expect(mcpStatus).ShouldNot(o.ContainSubstring("True"))

		e2e.Logf("mcps are not degraded")

	})

	g.It("Author:rioliu-Longduration-CPaasrunOnly-Critical-42361-add chrony systemd config [Disruptive]", func() {
		g.By("create new mc to apply chrony config on worker nodes")

		mcName := "change-workers-chrony-configuration"
		mcTemplate := generateTemplateAbsolutePath("change-workers-chrony-configuration.yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		defer mc.delete(oc)
		mc.create(oc)

		g.By("get one worker node to verify the config changes")
		nodeName, err := getFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		stdout, err := debugNodeWithChroot(oc, nodeName, "cat", "/etc/chrony.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf(stdout)
		o.Expect(stdout).Should(o.ContainSubstring("pool 0.rhel.pool.ntp.org iburst"))
		o.Expect(stdout).Should(o.ContainSubstring("driftfile /var/lib/chrony/drift"))
		o.Expect(stdout).Should(o.ContainSubstring("makestep 1.0 3"))
		o.Expect(stdout).Should(o.ContainSubstring("rtcsync"))
		o.Expect(stdout).Should(o.ContainSubstring("logdir /var/log/chrony"))

	})

	g.It("Author:rioliu-Longduration-CPaasrunOnly-High-42520-retrieve mc with large size from mcs [Disruptive]", func() {
		g.By("create new mc to add 100+ dummy files to /var/log")

		mcName := "bz1866117-add-dummy-files"
		mcTemplate := generateTemplateAbsolutePath("bz1866117-add-dummy-files.yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		defer mc.delete(oc)
		mc.create(oc)

		g.By("get one master node to do mc query")
		masterNode, err := getFirstMasterNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		stdout, err := debugNode(oc, masterNode, false, "curl", "-w", "'Total: %{time_total}'", "-k", "-s", "-o", "/dev/null", "https://localhost:22623/config/worker")
		o.Expect(err).NotTo(o.HaveOccurred())

		var timecost float64
		for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
			if strings.Contains(line, "Total") {
				substr := line[8 : len(line)-1]
				timecost, _ = strconv.ParseFloat(substr, 64)
				break
			}
		}
		e2e.Logf("api time cost is: %f", timecost)

		o.Expect(float64(timecost)).Should(o.BeNumerically("<", 10.0))

	})

	g.It("Author:mhanss-CPaasrunOnly-Critical-43043-Critical-43064-create/delete custom machine config pool [Disruptive]", func() {
		g.By("get worker node to change the label")
		workerNode, err := getFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Add label as infra to the existing node")
		labelOutput, err := addCustomLabelToNode(oc, workerNode, "infra=")
		defer deleteCustomLabelFromNode(oc, workerNode, "infra")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(labelOutput).Should(o.ContainSubstring(workerNode))
		nodeLabel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes/" + workerNode).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeLabel).Should(o.ContainSubstring("infra"))

		g.By("Create custom infra mcp")
		mcpName := "infra"
		mcpTemplate := generateTemplateAbsolutePath("custom-machine-config-pool.yaml")
		mcp := MachineConfigPool{name: mcpName, template: mcpTemplate}
		defer mcp.delete(oc)
		defer waitForNodeDoesNotContain(oc, workerNode, mcpName)
		defer deleteCustomLabelFromNode(oc, workerNode, mcpName)
		mcp.create(oc)
		e2e.Logf("Custom mcp is created successfully!")

		g.By("Remove custom label from the node")
		unlabeledOutput, err := deleteCustomLabelFromNode(oc, workerNode, mcpName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(unlabeledOutput).Should(o.ContainSubstring(workerNode))
		waitForNodeDoesNotContain(oc, workerNode, mcpName)

		g.By("Check custom infra label is removed from the node")
		nodeOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/infra").Output()
		o.Expect(nodeOut).Should(o.ContainSubstring("No resources found"))

		g.By("Remove custom infra mcp")
		mcp.delete(oc)

		g.By("Check custom infra mcp is deleted")
		mcpOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/" + mcpName).Output()
		o.Expect(mcpOut).Should(o.ContainSubstring("NotFound"))
		e2e.Logf("Custom mcp is deleted successfully!")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42365-add real time kernel argument [Disruptive]", func() {
		textToVerify := TextToVerify{
			textToVerifyForMC:   "realtime",
			textToVerifyForNode: "PREEMPT_RT",
			needBash:            true,
		}
		createMcAndVerifyMCValue(oc, "Kernel argument", "change-worker-kernel-argument", textToVerify, "uname -a")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42364-add selinux kernel argument [Disruptive]", func() {
		textToVerify := TextToVerify{
			textToVerifyForMC:   "enforcing=0",
			textToVerifyForNode: "enforcing=0",
		}
		createMcAndVerifyMCValue(oc, "Kernel argument", "change-worker-kernel-selinux", textToVerify, "cat", "/rootfs/proc/cmdline")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42367-add extension to RHCOS [Disruptive]", func() {
		textToVerify := TextToVerify{
			textToVerifyForMC:   "usbguard",
			textToVerifyForNode: "usbguard",
			needChroot:          true,
		}
		createMcAndVerifyMCValue(oc, "Usb Extension", "change-worker-extension-usbguard", textToVerify, "rpm", "-q", "usbguard")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42368-add max pods to the kubelet config [Disruptive]", func() {
		g.By("create kubelet config to add 500 max pods")
		kcName := "change-maxpods-kubelet-config"
		kcTemplate := generateTemplateAbsolutePath(kcName + ".yaml")
		kc := KubeletConfig{name: kcName, template: kcTemplate}
		defer kc.delete(oc)
		kc.create(oc)
		e2e.Logf("Kubelet config is created successfully!")

		g.By(fmt.Sprintf("Check max pods in the created kubelet config"))
		kcOut, err := getKubeletConfigDetails(oc, kc.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kcOut).Should(o.ContainSubstring("maxPods: 500"))
		e2e.Logf("Max pods are verified in the created kubelet config!")

		g.By("Check kubelet config in the worker node")
		workerNode, err := getFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		maxPods, err := debugNodeWithChroot(oc, workerNode, "cat", "/etc/kubernetes/kubelet.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(maxPods).Should(o.ContainSubstring("\"maxPods\": 500"))
		e2e.Logf("Max pods are verified in the worker node!")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42369-add container runtime config [Disruptive]", func() {
		g.By("Create container runtime config")
		crName := "change-ctr-cr-config"
		crTemplate := generateTemplateAbsolutePath(crName + ".yaml")
		cr := ContainerRuntimeConfig{name: crName, template: crTemplate}
		defer cr.delete(oc)
		cr.create(oc)
		e2e.Logf("Container runtime config is created successfully!")

		g.By("Check container runtime config values in the created config")
		crOut, err := getContainerRuntimeConfigDetails(oc, cr.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crOut).Should(
			o.And(
				o.ContainSubstring("logLevel: debug"),
				o.ContainSubstring("logSizeMax: \"-1\""),
				o.ContainSubstring("pidsLimit: 2048"),
				o.ContainSubstring("overlaySize: 8G")))
		e2e.Logf("Container runtime config values are verified in the created config!")

		g.By("Check container runtime config values in the worker node")
		workerNode, err := getFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		crStorageOut, err := debugNodeWithChroot(oc, workerNode, "head", "-n", "7", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crStorageOut).Should(o.ContainSubstring("size = \"8G\""))
		crConfigOut, err := debugNodeWithChroot(oc, workerNode, "crio", "config")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crConfigOut).Should(
			o.And(
				o.ContainSubstring("log_level = \"debug\""),
				o.ContainSubstring("pids_limit = 2048")))
		e2e.Logf("Container runtime config values are verified in the worker node!")
	})
})

func createMcAndVerifyMCValue(oc *exutil.CLI, stepText string, mcName string, textToVerify TextToVerify, cmd ...string) {
	g.By(fmt.Sprintf("Create new MC to add the %s", stepText))
	mcTemplate := generateTemplateAbsolutePath(mcName + ".yaml")
	mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
	defer mc.delete(oc)
	mc.create(oc)
	e2e.Logf("Machine config is created successfully!")

	g.By(fmt.Sprintf("Check %s in the created machine config", stepText))
	mcOut, err := getMachineConfigDetails(oc, mc.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(mcOut).Should(o.ContainSubstring(textToVerify.textToVerifyForMC))
	e2e.Logf("%s is verified in the created machine config!", stepText)

	g.By(fmt.Sprintf("Check %s in the machine config daemon", stepText))
	workerNode, err := getFirstWorkerNode(oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	var podOut string
	if textToVerify.needBash {
		podOut, err = exutil.RemoteShPodWithBash(oc, "openshift-machine-config-operator", getMachineConfigDaemon(oc, workerNode), cmd...)
	} else if textToVerify.needChroot {
		podOut, err = exutil.RemoteShPodWithChroot(oc, "openshift-machine-config-operator", getMachineConfigDaemon(oc, workerNode), cmd...)
	} else {
		podOut, err = exutil.RemoteShPod(oc, "openshift-machine-config-operator", getMachineConfigDaemon(oc, workerNode), cmd...)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(podOut).Should(o.ContainSubstring(textToVerify.textToVerifyForNode))
	e2e.Logf("%s is verified in the machine config daemon!", stepText)
}
