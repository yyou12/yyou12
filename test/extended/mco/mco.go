package mco

import (
	"fmt"
	"os/exec"
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

	g.It("Author:rioliu-Longduration-Critical-42361-add chrony systemd config [Disruptive]", func() {
		g.By("create new mc to apply chrony config on worker nodes")

		mcName := "change-workers-chrony-configuration"
		mcTemplate := generateTemplateAbsolutePath("change-workers-chrony-configuration.yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		defer mc.delete(oc)
		mc.create(oc)

		g.By("get one worker node to verify the config changes")
		nodeName, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		stdout, err := exutil.DebugNodeWithChroot(oc, nodeName, "cat", "/etc/chrony.conf")
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
		masterNode, err := exutil.GetFirstMasterNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		stdout, err := exutil.DebugNode(oc, masterNode, "curl", "-w", "'Total: %{time_total}'", "-k", "-s", "-o", "/dev/null", "https://localhost:22623/config/worker")
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
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Add label as infra to the existing node")
		labelOutput, err := exutil.AddCustomLabelToNode(oc, workerNode, "infra")
		defer func() {
			// ignore output, just focus on error handling, if error is occurred, fail this case
			_, deletefailure := exutil.DeleteCustomLabelFromNode(oc, workerNode, "infra")
			o.Expect(deletefailure).NotTo(o.HaveOccurred())
		}()
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
		defer func() {
			// ignore output, just focus on error handling, if error is occurred, fail this case
			_, deletefailure := exutil.DeleteCustomLabelFromNode(oc, workerNode, mcpName)
			o.Expect(deletefailure).NotTo(o.HaveOccurred())
		}()
		mcp.create(oc)
		e2e.Logf("Custom mcp is created successfully!")

		g.By("Remove custom label from the node")
		unlabeledOutput, err := exutil.DeleteCustomLabelFromNode(oc, workerNode, mcpName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(unlabeledOutput).Should(o.ContainSubstring(workerNode))
		waitForNodeDoesNotContain(oc, workerNode, mcpName)

		g.By("Check custom infra label is removed from the node")
		nodeOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/infra").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeOut).Should(o.ContainSubstring("No resources found"))

		g.By("Remove custom infra mcp")
		mcp.delete(oc)

		g.By("Check custom infra mcp is deleted")
		mcpOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/" + mcpName).Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(mcpOut).Should(o.ContainSubstring("NotFound"))
		e2e.Logf("Custom mcp is deleted successfully!")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42365-add real time kernel argument [Disruptive]", func() {
		workerNode, err := skipTestIfOsIsNotCoreOs(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		textToVerify := TextToVerify{
			textToVerifyForMC:   "realtime",
			textToVerifyForNode: "PREEMPT_RT",
			needBash:            true,
		}
		createMcAndVerifyMCValue(oc, "Kernel argument", "change-worker-kernel-argument", workerNode, textToVerify, "uname -a")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42364-add selinux kernel argument [Disruptive]", func() {
		workerNode, err := skipTestIfOsIsNotCoreOs(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		textToVerify := TextToVerify{
			textToVerifyForMC:   "enforcing=0",
			textToVerifyForNode: "enforcing=0",
		}
		createMcAndVerifyMCValue(oc, "Kernel argument", "change-worker-kernel-selinux", workerNode, textToVerify, "cat", "/rootfs/proc/cmdline")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42367-add extension to RHCOS [Disruptive]", func() {
		workerNode, err := skipTestIfOsIsNotCoreOs(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		textToVerify := TextToVerify{
			textToVerifyForMC:   "usbguard",
			textToVerifyForNode: "usbguard",
			needChroot:          true,
		}
		createMcAndVerifyMCValue(oc, "Usb Extension", "change-worker-extension-usbguard", workerNode, textToVerify, "rpm", "-q", "usbguard")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-43310-add kernel arguments, kernel type and extension to the RHCOS and RHEL [Disruptive]", func() {
		rhelOs, err := exutil.GetFirstRhelWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		coreOs, err := exutil.GetFirstCoreOsWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if rhelOs == "" || coreOs == "" {
			g.Skip("Both Rhel and CoreOs are required to execute this test case!")
		}

		g.By("Create new MC to add the kernel arguments, kernel type and extension")
		mcName := "change-worker-karg-ktype-extension"
		mcTemplate := generateTemplateAbsolutePath(mcName + ".yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		defer mc.delete(oc)
		mc.create(oc)

		g.By("Check kernel arguments, kernel type and extension on the created machine config")
		mcOut, err := getMachineConfigDetails(oc, mc.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mcOut).Should(
			o.And(
				o.ContainSubstring("usbguard"),
				o.ContainSubstring("z=10"),
				o.ContainSubstring("realtime")))

		g.By("Check kernel arguments, kernel type and extension on the rhel worker node")
		rhelRpmOut, err := exutil.DebugNodeWithChroot(oc, rhelOs, "rpm", "-qa")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelRpmOut).Should(o.And(
			o.MatchRegexp(".*kernel-tools-[0-9-.]+el[0-9]+.x86_64.*"),
			o.MatchRegexp(".*kernel-tools-libs-[0-9-.]+el[0-9]+.x86_64.*"),
			o.MatchRegexp(".*kernel-[0-9-.]+el[0-9]+.x86_64.*")))
		rhelUnameOut, err := exutil.DebugNodeWithChroot(oc, rhelOs, "uname", "-a")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelUnameOut).Should(o.Not(o.ContainSubstring("PREEMPT_RT")))
		rhelCmdlineOut, err := exutil.DebugNodeWithChroot(oc, rhelOs, "cat", "/proc/cmdline")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelCmdlineOut).Should(o.Not(o.ContainSubstring("z=10")))

		g.By("Check kernel arguments, kernel type and extension on the rhcos worker node")
		coreOsRpmOut, err := exutil.DebugNodeWithChroot(oc, coreOs, "rpm", "-qa")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(coreOsRpmOut).Should(o.And(
			o.MatchRegexp(".*kernel-rt-kvm-[0-9-.]+rt[0-9.]+el[0-9_]+.x86_64.*"),
			o.MatchRegexp(".*kernel-rt-core-[0-9-.]+rt[0-9.]+el[0-9_]+.x86_64.*"),
			o.MatchRegexp(".*kernel-rt-modules-extra-[0-9-.]+rt[0-9.]+el[0-9_]+.x86_64.*"),
			o.MatchRegexp(".*kernel-rt-modules-[0-9-.]+rt[0-9.]+el[0-9_]+.x86_64.*")))
		coreOsUnameOut, err := exutil.DebugNodeWithChroot(oc, coreOs, "uname", "-a")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(coreOsUnameOut).Should(o.ContainSubstring("PREEMPT_RT"))
		coreOsCmdlineOut, err := exutil.DebugNodeWithChroot(oc, coreOs, "cat", "/proc/cmdline")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(coreOsCmdlineOut).Should(o.ContainSubstring("z=10"))
		e2e.Logf("Kernel argument, kernel type and extension changes are verified on both rhcos and rhel worker nodes!")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42368-add max pods to the kubelet config [Disruptive]", func() {
		g.By("create kubelet config to add 500 max pods")
		kcName := "change-maxpods-kubelet-config"
		kcTemplate := generateTemplateAbsolutePath(kcName + ".yaml")
		kc := KubeletConfig{name: kcName, template: kcTemplate}
		defer kc.delete(oc)
		kc.create(oc)
		e2e.Logf("Kubelet config is created successfully!")

		g.By("Check max pods in the created kubelet config")
		kcOut, err := getKubeletConfigDetails(oc, kc.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kcOut).Should(o.ContainSubstring("maxPods: 500"))
		e2e.Logf("Max pods are verified in the created kubelet config!")

		g.By("Check kubelet config in the worker node")
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		maxPods, err := exutil.DebugNodeWithChroot(oc, workerNode, "cat", "/etc/kubernetes/kubelet.conf")
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
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		crStorageOut, err := exutil.DebugNodeWithChroot(oc, workerNode, "head", "-n", "7", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crStorageOut).Should(o.ContainSubstring("size = \"8G\""))
		crConfigOut, err := exutil.DebugNodeWithChroot(oc, workerNode, "crio", "config")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crConfigOut).Should(
			o.And(
				o.ContainSubstring("log_level = \"debug\""),
				o.ContainSubstring("pids_limit = 2048")))
		e2e.Logf("Container runtime config values are verified in the worker node!")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Critical-42438-add journald systemd config [Disruptive]", func() {
		g.By("Create journald systemd config")
		encodedConf, err := exec.Command("bash", "-c", "cat "+generateTemplateAbsolutePath("journald.conf")+" | base64 | tr -d '\n'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		conf := string(encodedConf)
		jcName := "change-worker-jrnl-configuration"
		jcTemplate := generateTemplateAbsolutePath(jcName + ".yaml")
		journaldConf := []string{"CONFIGURATION=" + conf}
		jc := MachineConfig{name: jcName, template: jcTemplate, pool: "worker", parameters: journaldConf}
		defer jc.delete(oc)
		jc.create(oc)
		e2e.Logf("Journald systemd config is created successfully!")
		g.By("Check journald config value in the created machine config!")
		jcOut, err := getMachineConfigDetails(oc, jc.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(jcOut).Should(o.ContainSubstring(conf))
		e2e.Logf("Journald config is verified in the created machine config!")
		g.By("Check journald config values in the worker node")
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		journaldConfOut, err := exutil.DebugNodeWithChroot(oc, workerNode, "cat", "/etc/systemd/journald.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(journaldConfOut).Should(
			o.And(
				o.ContainSubstring("RateLimitInterval=1s"),
				o.ContainSubstring("RateLimitBurst=10000"),
				o.ContainSubstring("Storage=volatile"),
				o.ContainSubstring("Compress=no"),
				o.ContainSubstring("MaxRetentionSec=30s")))
		e2e.Logf("Journald config values are verified in the worker node!")
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-High-43405-node drain is not needed for mirror config change in container registry [Disruptive]", func() {
		g.By("Create image content source policy for mirror changes")
		icspName := "repository-mirror"
		icspTemplate := generateTemplateAbsolutePath(icspName + ".yaml")
		icsp := ImageContentSourcePolicy{name: icspName, template: icspTemplate}
		defer icsp.delete(oc)
		icsp.create(oc)

		g.By("Check registry changes in the worker node")
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		registryOut, err := exutil.DebugNodeWithChroot(oc, workerNode, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(registryOut).Should(
			o.And(
				o.ContainSubstring("mirror-by-digest-only = true"),
				o.ContainSubstring("example.com/example/ubi-minimal"),
				o.ContainSubstring("example.io/example/ubi-minimal")))

		g.By("Check MCD logs to make sure drain is skipped")
		podLogs, err := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, workerNode), "drain")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Pod logs to skip node drain :\n %v", podLogs)
		o.Expect(podLogs).Should(
			o.And(
				o.ContainSubstring("/etc/containers/registries.conf: changes made are safe to skip drain"),
				o.ContainSubstring("Changes do not require drain, skipping")))
	})

	g.It("Author:rioliu-CPaasrunOnly-High-42218-add machine config without ignition version [Serial]", func() {
		createMcAndVerifyIgnitionVersion(oc, "empty ign version", "change-worker-ign-version-to-empty", "")
	})

	g.It("Author:mhanss-CPaasrunOnly-High-43124-add machine config with invalid ignition version [Serial]", func() {
		createMcAndVerifyIgnitionVersion(oc, "invalid ign version", "change-worker-ign-version-to-invalid", "3.9.0")
	})

	g.It("Author:rioliu-CPaasrunOnly-High-42679-add new ssh authorized keys [Serial]", func() {
		g.By("Create new machine config with new authorized key")
		mcName := "change-worker-add-ssh-authorized-key"
		mcTemplate := generateTemplateAbsolutePath(mcName + ".yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		defer mc.delete(oc)
		mc.create(oc)
		g.By("Check content of file authorized_keys to verify whether new one is added successfully")
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		sshKeyOut, err := exutil.DebugNodeWithChroot(oc, workerNode, "cat", "/home/core/.ssh/authorized_keys")
		e2e.Logf("file content of authorized_keys: %v", sshKeyOut)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sshKeyOut).Should(o.ContainSubstring("mco_test@redhat.com"))
	})
})

func createMcAndVerifyMCValue(oc *exutil.CLI, stepText string, mcName string, workerNode string, textToVerify TextToVerify, cmd ...string) {
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

// skipTestIfOsIsNotCoreOs it will either skip the test case in case of worker node is not CoreOS or will return the CoreOS worker node
func skipTestIfOsIsNotCoreOs(oc *exutil.CLI) (string, error) {
	coreOs, err := exutil.GetFirstCoreOsWorkerNode(oc)
	if coreOs == "" {
		g.Skip("CoreOs is required to execute this test case!")
	}
	return coreOs, err
}

func createMcAndVerifyIgnitionVersion(oc *exutil.CLI, stepText string, mcName string, ignitionVersion string) {
	g.By(fmt.Sprintf("Create machine config with %s", stepText))
	mcTemplate := generateTemplateAbsolutePath("change-worker-ign-version.yaml")
	mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker", parameters: []string{"IGNITION_VERSION=" + ignitionVersion}}
	defer mc.delete(oc)
	mc.create(oc)

	g.By("Get mcp/worker status to check whether it is degraded")
	mcpDataMap, err := getStatusCondition(oc, "mcp/worker", "RenderDegraded")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(mcpDataMap).NotTo(o.BeNil())
	o.Expect(mcpDataMap["status"].(string)).Should(o.Equal("True"))
	o.Expect(mcpDataMap["message"].(string)).Should(o.ContainSubstring("parsing Ignition config failed: unknown version. Supported spec versions: 2.2, 3.0, 3.1, 3.2"))

	g.By("Get co machine config to verify status and reason for Upgradeable type")
	mcDataMap, err := getStatusCondition(oc, "co/machine-config", "Upgradeable")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(mcDataMap).NotTo(o.BeNil())
	o.Expect(mcDataMap["status"].(string)).Should(o.Equal("False"))
	o.Expect(mcDataMap["message"].(string)).Should(o.ContainSubstring("One or more machine config pools are degraded, please see `oc get mcp` for further details and resolve before upgrading"))
}
