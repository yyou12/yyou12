package mco

import (
	"encoding/json"
	"fmt"
	ci "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
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
		platform := ci.CheckPlatform(oc)
		if platform == "gcp" || platform == "aws" {
			workerNode, err := skipTestIfOsIsNotCoreOs(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			textToVerify := TextToVerify{
				textToVerifyForMC:   "realtime",
				textToVerifyForNode: "PREEMPT_RT",
				needBash:            true,
			}
			createMcAndVerifyMCValue(oc, "Kernel argument", "change-worker-kernel-argument", workerNode, textToVerify, "uname -a")
		} else {
			g.Skip("AWS or GCP platform is required to execute this test case as currently kernel real time argument is only supported by these platforms!")
		}
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
		defer func() {
			kc.delete(oc)
			mcp := MachineConfigPool{name: "worker"}
			mcp.waitForComplete(oc)
		}()
		kc.create(oc)
		kc.waitUntilSuccess(oc, "10s")
		mcp := MachineConfigPool{name: "worker"}
		mcp.waitForComplete(oc)
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

	g.It("Author:mhanss-CPaasrunOnly-Medium-43084-shutdown machine config daemon with SIGTERM [Disruptive]", func() {
		g.By("Create new machine config to add additional ssh key")
		mcName := "add-additional-ssh-authorized-key"
		mcTemplate := generateTemplateAbsolutePath(mcName + ".yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		defer mc.delete(oc)
		mc.create(oc)

		g.By("Check MCD logs to make sure shutdown machine config daemon with SIGTERM")
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		podLogs, err := exutil.WaitAndGetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, workerNode), "SIGTERM")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podLogs).Should(
			o.And(
				o.ContainSubstring("Adding SIGTERM protection"),
				o.ContainSubstring("Removing SIGTERM protection")))

		g.By("Kill MCD process")
		mcdKillLogs, err := exutil.DebugNodeWithChroot(oc, workerNode, "pgrep", "-f", "machine-config-daemon_")
		o.Expect(err).NotTo(o.HaveOccurred())
		mcpPid := regexp.MustCompile("(?m)^[0-9]+").FindString(mcdKillLogs)
		_, err = exutil.DebugNodeWithChroot(oc, workerNode, "kill", mcpPid)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check MCD logs to make sure machine config daemon without SIGTERM")
		mcDaemon := getMachineConfigDaemon(oc, workerNode)
		exutil.AssertPodToBeReady(oc, mcDaemon, "openshift-machine-config-operator")
		mcdLogs, err := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", mcDaemon, "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mcdLogs).ShouldNot(o.ContainSubstring("SIGTERM"))
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-High-42682-change container registry config on ocp 4.6 [Disruptive]", func() {
		clusterVersion, _, err := exutil.GetClusterVersion(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if clusterVersion != 4.6 {
			g.Skip("Cluster version 4.6 is required to execute this test case!")
		}

		g.By("Create new machine config to add quay.io to unqualified-search-registries list")
		mcName := "change-workers-container-reg"
		mcTemplate := generateTemplateAbsolutePath(mcName + ".yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		defer mc.delete(oc)
		mc.create(oc)

		g.By("Check content of registries file to verify quay.io added to unqualified-search-registries list")
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		regOut, err := exutil.DebugNodeWithChroot(oc, workerNode, "cat", "/etc/containers/registries.conf")
		e2e.Logf("File content of registries conf: %v", regOut)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(regOut).Should(o.ContainSubstring("quay.io"))

		g.By("Check MCD logs to make sure drain is successful and pods are evicted")
		podLogs, err := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, workerNode), "\"evicted\\|drain\"")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Pod logs for node drain and pods evicted :\n %v", podLogs)
		o.Expect(podLogs).Should(
			o.And(
				o.ContainSubstring("Update prepared; beginning drain"),
				o.ContainSubstring("Evicted pod openshift-image-registry/image-registry"),
				o.ContainSubstring("drain complete")))
	})

	g.It("Author:rioliu-Longduration-CPaasrunOnly-High-42704-disable auto reboot for mco [Disruptive]", func() {
		g.By("pause mcp worker")
		mcp := MachineConfigPool{name: "worker"}
		defer mcp.pause(oc, false)
		mcp.pause(oc, true)

		g.By("create new mc")
		mcName := "change-workers-chrony-configuration"
		mcTemplate := generateTemplateAbsolutePath("change-workers-chrony-configuration.yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker", skipWaitForMcp: true}
		defer mc.delete(oc)
		mc.create(oc)

		g.By("compare config name b/w spec.configuration.name and status.configuration.name, they're different")
		specConf, specErr := mcp.getConfigNameOfSpec(oc)
		o.Expect(specErr).NotTo(o.HaveOccurred())
		statusConf, statusErr := mcp.getConfigNameOfStatus(oc)
		o.Expect(statusErr).NotTo(o.HaveOccurred())
		o.Expect(specConf).ShouldNot(o.Equal(statusConf))

		g.By("check mcp status condition, expected: UPDATED=False && UPDATING=False")
		var updated, updating string
		pollerr := wait.Poll(5*time.Second, 10*time.Second, func() (bool, error) {
			stdouta, erra := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/worker", "-o", "jsonpath='{.status.conditions[?(@.type==\"Updated\")].status}'").Output()
			stdoutb, errb := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/worker", "-o", "jsonpath='{.status.conditions[?(@.type==\"Updating\")].status}'").Output()
			updated = strings.Trim(stdouta, "'")
			updating = strings.Trim(stdoutb, "'")
			if erra != nil || errb != nil {
				e2e.Logf("error occurred %v%v", erra, errb)
				return false, nil
			}
			if updated != "" && updating != "" {
				e2e.Logf("updated: %v, updating: %v", updated, updating)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(pollerr, "polling status conditions of mcp: [Updated,Updating] failed")
		o.Expect(updated).Should(o.Equal("False"))
		o.Expect(updating).Should(o.Equal("False"))

		g.By("unpause mcp worker, then verify whether the new mc can be applied on mcp/worker")
		mcp.pause(oc, false)
		mcp.waitForComplete(oc)
	})

	g.It("Author:rioliu-CPaasrunOnly-High-42681-rotate kubernetes certificate authority [Disruptive]", func() {
		g.By("patch secret to trigger CA rotation")
		patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret", "-p", `{"metadata": {"annotations": {"auth.openshift.io/certificate-not-after": null}}}`, "kube-apiserver-to-kubelet-signer", "-n", "openshift-kube-apiserver-operator").Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())

		g.By("monitor update progress of mcp master and worker, new configs should be applied successfully")
		mcpMaster := MachineConfigPool{name: "master"}
		mcpWorker := MachineConfigPool{name: "worker"}
		mcpMaster.waitForComplete(oc)
		mcpWorker.waitForComplete(oc)

		g.By("check new generated rendered configs for kuberlet CA")
		renderedConfs, renderedErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", "--sort-by=metadata.creationTimestamp", "-o", "jsonpath='{.items[-2:].metadata.name}'").Output()
		o.Expect(renderedErr).NotTo(o.HaveOccurred())
		o.Expect(renderedConfs).NotTo(o.BeEmpty())
		slices := strings.Split(strings.Trim(renderedConfs, "'"), " ")
		var renderedMasterConf, renderedWorkerConf string
		for _, conf := range slices {
			if strings.Contains(conf, "master") {
				renderedMasterConf = conf
			} else if strings.Contains(conf, "worker") {
				renderedWorkerConf = conf
			}
		}
		e2e.Logf("new rendered config generated for master: %s", renderedMasterConf)
		e2e.Logf("new rendered config generated for worker: %s", renderedWorkerConf)

		g.By("check logs of machine-config-daemon on master-n-worker nodes, make sure CA change is detected, drain and reboot are skipped")
		masterNode, masterErr := exutil.GetFirstMasterNode(oc)
		workerNode, workerErr := exutil.GetFirstWorkerNode(oc)
		o.Expect(masterErr).NotTo(o.HaveOccurred())
		o.Expect(workerErr).NotTo(o.HaveOccurred())
		commonExpectedStrings := []string{"File diff: detected change to /etc/kubernetes/kubelet-ca.crt", "Changes do not require drain, skipping"}
		expectedStringsForMaster := append(commonExpectedStrings, "Node has Desired Config "+renderedMasterConf+", skipping reboot")
		expectedStringsForWorker := append(commonExpectedStrings, "Node has Desired Config "+renderedWorkerConf+", skipping reboot")
		masterMcdLogs, masterMcdLogErr := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, masterNode), "")
		o.Expect(masterMcdLogErr).NotTo(o.HaveOccurred())
		workerMcdLogs, workerMcdLogErr := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, workerNode), "")
		o.Expect(workerMcdLogErr).NotTo(o.HaveOccurred())
		foundOnMaster := containsMultipleStrings(masterMcdLogs, expectedStringsForMaster)
		o.Expect(foundOnMaster).Should(o.BeTrue())
		e2e.Logf("mcd log on master node %s contains expected strings: %v", masterNode, expectedStringsForMaster)
		foundOnWorker := containsMultipleStrings(workerMcdLogs, expectedStringsForWorker)
		o.Expect(foundOnWorker).Should(o.BeTrue())
		e2e.Logf("mcd log on worker node %s contains expected strings: %v", workerNode, expectedStringsForWorker)
	})

	g.It("Author:rioliu-CPaasrunOnly-High-43085-check mcd crash-loop-back-off error in log [Serial]", func() {
		g.By("get master and worker nodes")
		masterNode, masterErr := exutil.GetFirstMasterNode(oc)
		workerNode, workerErr := exutil.GetFirstWorkerNode(oc)
		o.Expect(masterErr).NotTo(o.HaveOccurred())
		o.Expect(workerErr).NotTo(o.HaveOccurred())
		e2e.Logf("master node %s", masterNode)
		e2e.Logf("worker node %s", workerNode)

		g.By("check error messages in mcd logs for both master and worker nodes")
		expectedStrings := []string{"unable to update node", "cannot apply annotation for SSH access due to"}
		masterMcdLogs, masterMcdLogErr := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, masterNode), "")
		o.Expect(masterMcdLogErr).NotTo(o.HaveOccurred())
		workerMcdLogs, workerMcdLogErr := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, workerNode), "")
		o.Expect(workerMcdLogErr).NotTo(o.HaveOccurred())
		foundOnMaster := containsMultipleStrings(masterMcdLogs, expectedStrings)
		o.Expect(foundOnMaster).Should(o.BeFalse())
		e2e.Logf("mcd log on master node %s does not contain error messages: %v", masterNode, expectedStrings)
		foundOnWorker := containsMultipleStrings(workerMcdLogs, expectedStrings)
		o.Expect(foundOnWorker).Should(o.BeFalse())
		e2e.Logf("mcd log on worker node %s does not contain error messages: %v", workerNode, expectedStrings)
	})

	g.It("Author:mhanss-Longduration-CPaasrunOnly-Medium-43245-bump initial drain sleeps down to 1min [Disruptive]", func() {
		g.By("Create a pod disruption budget to set minAvailable to 1")
		oc.SetupProject()
		nsName := oc.Namespace()
		pdbName := "dont-evict-43245"
		pdbTemplate := generateTemplateAbsolutePath("pod-disruption-budget.yaml")
		pdb := PodDisruptionBudget{name: pdbName, namespace: nsName, template: pdbTemplate}
		defer pdb.delete(oc)
		pdb.create(oc)

		g.By("Create new pod for pod disruption budget")
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		hostname, err := exutil.GetNodeHostname(oc, workerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		podName := "dont-evict-43245"
		podTemplate := generateTemplateAbsolutePath("create-pod.yaml")
		pod := exutil.Pod{Name: podName, Namespace: nsName, Template: podTemplate, Parameters: []string{"HOSTNAME=" + hostname}}
		defer func() { o.Expect(pod.Delete(oc)).NotTo(o.HaveOccurred()) }()
		pod.Create(oc)

		g.By("Create new mc to add new file on the node and trigger node drain")
		mcName := "test-file"
		mcTemplate := generateTemplateAbsolutePath("add-mc-to-trigger-node-drain.yaml")
		mc := MachineConfig{name: mcName, template: mcTemplate, pool: "worker", skipWaitForMcp: true}
		defer mc.delete(oc)
		defer func() { o.Expect(pod.Delete(oc)).NotTo(o.HaveOccurred()) }()
		mc.create(oc)

		g.By("Check mcd logs to see the sleep interval b/w failed drains")
		podLogs := waitForNumberOfLinesInPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, workerNode), "Draining", 6)
		timestamps := filterTimestampFromLogs(podLogs, 6)
		o.Expect(getTimeDifferenceInMinute(timestamps[0], timestamps[1])).Should(o.BeNumerically("<=", 2.7))
		o.Expect(getTimeDifferenceInMinute(timestamps[4], timestamps[5])).Should(o.BeNumerically("<=", 6.7))
	})

	g.It("Author:rioliu-CPaasrunOnly-High-43278-security fix for unsafe cipher [Serial]", func() {
		g.By("check go version >= 1.15")
		_, clusterVersion, cvErr := exutil.GetClusterVersion(oc)
		o.Expect(cvErr).NotTo(o.HaveOccurred())
		o.Expect(clusterVersion).NotTo(o.BeEmpty())
		e2e.Logf("cluster version is %s", clusterVersion)
		commitId, commitErr := getCommitId(oc, "machine-config", clusterVersion)
		o.Expect(commitErr).NotTo(o.HaveOccurred())
		o.Expect(commitId).NotTo(o.BeEmpty())
		e2e.Logf("machine config commit id is %s", commitId)
		goVersion, verErr := getGoVersion("machine-config-operator", commitId)
		o.Expect(verErr).NotTo(o.HaveOccurred())
		e2e.Logf("go version is: %f", goVersion)
		o.Expect(float64(goVersion)).Should(o.BeNumerically(">", 1.15))

		g.By("verify TLS protocol version is 1.3")
		masterNode, nodeErr := exutil.GetFirstMasterNode(oc)
		o.Expect(nodeErr).NotTo(o.HaveOccurred())
		sslOutput, sslErr := exutil.DebugNodeWithChroot(oc, masterNode, "bash", "-c", "openssl s_client -connect localhost:6443 2>&1|grep -A3 SSL-Session")
		e2e.Logf("ssl protocol version is:\n %s", sslOutput)
		o.Expect(sslErr).NotTo(o.HaveOccurred())
		o.Expect(sslOutput).Should(o.ContainSubstring("TLSv1.3"))

		g.By("verify whether the unsafe cipher is disabled")
		cipherOutput, cipherErr := exutil.DebugNodeWithOptions(oc, masterNode, []string{"--image=drwetter/testssl.sh", "-n", "openshift-machine-config-operator"}, "testssl.sh", "--quiet", "--sweet32", "localhost:6443")
		e2e.Logf("test ssh script output:\n %s", cipherOutput)
		o.Expect(cipherErr).NotTo(o.HaveOccurred())
		o.Expect(cipherOutput).Should(o.ContainSubstring("not vulnerable (OK)"))
	})

	g.It("Author:sregidor-CPaasrunOnly-High-43151-add node label to service monitor [Serial]", func() {
		g.By("Get current mcd_ metrics from machine-config-daemon service")

		clusterIp := getServiceClusterIP(oc, "machine-config-daemon", "openshift-machine-config-operator")
		port := getServicePort(oc, "machine-config-daemon", "openshift-machine-config-operator", "metrics")
		token := getSATokenFromContainer(oc, "prometheus-k8s-0", "openshift-monitoring", "prometheus")

		statsCmd := fmt.Sprintf("curl -s -k  -H 'Authorization: Bearer %s' https://%s:%s/metrics | grep 'mcd_' | grep -v '#'", token, clusterIp, port)
		e2e.Logf("stats output:\n %s", statsCmd)
		statsOut, err := exutil.RemoteShPod(oc, "openshift-monitoring", "prometheus-k8s-0", "sh", "-c", statsCmd)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_drain_err"))
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_host_os_and_version"))
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_kubelet_state"))
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_pivot_err"))
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_reboot_err"))
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_state"))
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_update_state"))
		o.Expect(statsOut).Should(o.ContainSubstring("mcd_update_state"))

		g.By("Check relabeling section in machine-config-daemon")
		sourceLabels, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("servicemonitor/machine-config-daemon", "-n", "openshift-machine-config-operator",
			"-o", "jsonpath='{.spec.endpoints[*].relabelings[*].sourceLabels}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceLabels).Should(o.ContainSubstring("__meta_kubernetes_pod_node_name"))

		g.By("Check node label in mcd_state metrics")
		stateQuery := getPrometheusQueryResults(oc, "mcd_state")
		e2e.Logf("metrics:\n %s", stateQuery)
		firstMasterName, err := exutil.GetFirstMasterNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		firstWorkerName, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stateQuery).Should(o.ContainSubstring(`"node":"` + firstMasterName + `"`))
		o.Expect(stateQuery).Should(o.ContainSubstring(`"node":"` + firstWorkerName + `"`))
	})

	g.It("Author:sregidor-CPaasrunOnly-High-43726-Azure ControllerConfig Infrastructure does not match cluster Infrastructure resource [Serial]", func() {
		g.By("Get machine-config-controller platform status.")
		mccPlatformStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("controllerconfig/machine-config-controller", "-o", "jsonpath='{.spec.infra.status.platformStatus}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("test mccPlatformStatus:\n %s", mccPlatformStatus)

		if ci.CheckPlatform(oc) == "azure" {
			g.By("check cloudName field.")
			trimMccPlatformStatus := strings.Trim(mccPlatformStatus, "'")
			e2e.Logf("test trimMccPlatformStatus:\n %s", trimMccPlatformStatus)

			var jsonMccPlatformStatus map[string]interface{}
			errparseinfra := json.Unmarshal([]byte(trimMccPlatformStatus), &jsonMccPlatformStatus)
			o.Expect(errparseinfra).NotTo(o.HaveOccurred())
			o.Expect(jsonMccPlatformStatus).Should(o.HaveKey("azure"))

			azure := jsonMccPlatformStatus["azure"].(map[string]interface{})
			o.Expect(azure).Should(o.HaveKey("cloudName"))
		}

		g.By("Get infrastructure platform status.")
		infraPlatformStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructures/cluster", "-o", "jsonpath='{.status.platformStatus}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("infraPlatformStatus:\n %s", infraPlatformStatus)

		g.By("Check same status in infra and machine-config-controller.")
		o.Expect(mccPlatformStatus).To(o.Equal(infraPlatformStatus))
	})

	g.It("Author:mhanss-CPaasrunOnly-high-42680-change pull secret in the openshift-config namespace [Serial]", func() {
		g.By("Add a dummy credential in pull secret")
		secretFile, err := getPullSecret(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		newSecretFile := generateTmpFile(oc, "pull-secret.dockerconfigjson")
		_, copyErr := exec.Command("bash", "-c", "cp "+secretFile+" "+newSecretFile).Output()
		o.Expect(copyErr).NotTo(o.HaveOccurred())
		newPullSecret, err := oc.AsAdmin().WithoutNamespace().Run("registry").Args("login", `--registry="quay.io"`, `--auth-basic="mhans-redhat:redhat123"`, "--to="+newSecretFile, "--skip-check").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(newPullSecret).Should(o.Equal(`Saved credentials for "quay.io"`))
		setData, err := setDataForPullSecret(oc, newSecretFile)
		defer func() {
			_, err := setDataForPullSecret(oc, secretFile)
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(setData).Should(o.Equal("secret/pull-secret data updated"))

		g.By("Check new generated rendered configs for newly added pull secret")
		renderedConfs, renderedErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", "--sort-by=metadata.creationTimestamp", "-o", "jsonpath='{.items[-2:].metadata.name}'").Output()
		o.Expect(renderedErr).NotTo(o.HaveOccurred())
		o.Expect(renderedConfs).NotTo(o.BeEmpty())
		slices := strings.Split(strings.Trim(renderedConfs, "'"), " ")
		var renderedMasterConf, renderedWorkerConf string
		for _, conf := range slices {
			if strings.Contains(conf, "master") {
				renderedMasterConf = conf
			} else if strings.Contains(conf, "worker") {
				renderedWorkerConf = conf
			}
		}
		e2e.Logf("New rendered config generated for master: %s", renderedMasterConf)
		e2e.Logf("New rendered config generated for worker: %s", renderedWorkerConf)

		g.By("Check logs of machine-config-daemon on master-n-worker nodes, make sure pull secret changes are detected, drain and reboot are skipped")
		masterNode, masterErr := exutil.GetFirstMasterNode(oc)
		workerNode, workerErr := exutil.GetFirstWorkerNode(oc)
		o.Expect(masterErr).NotTo(o.HaveOccurred())
		o.Expect(workerErr).NotTo(o.HaveOccurred())
		commonExpectedStrings := []string{"File diff: detected change to /var/lib/kubelet/config.json", "Changes do not require drain, skipping"}
		expectedStringsForMaster := append(commonExpectedStrings, "Node has Desired Config "+renderedMasterConf+", skipping reboot")
		expectedStringsForWorker := append(commonExpectedStrings, "Node has Desired Config "+renderedWorkerConf+", skipping reboot")
		masterMcdLogs, masterMcdLogErr := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, masterNode), "")
		o.Expect(masterMcdLogErr).NotTo(o.HaveOccurred())
		workerMcdLogs, workerMcdLogErr := exutil.GetSpecificPodLogs(oc, "openshift-machine-config-operator", "machine-config-daemon", getMachineConfigDaemon(oc, workerNode), "")
		o.Expect(workerMcdLogErr).NotTo(o.HaveOccurred())
		foundOnMaster := containsMultipleStrings(masterMcdLogs, expectedStringsForMaster)
		o.Expect(foundOnMaster).Should(o.BeTrue())
		e2e.Logf("MCD log on master node %s contains expected strings: %v", masterNode, expectedStringsForMaster)
		foundOnWorker := containsMultipleStrings(workerMcdLogs, expectedStringsForWorker)
		o.Expect(foundOnWorker).Should(o.BeTrue())
		e2e.Logf("MCD log on worker node %s contains expected strings: %v", workerNode, expectedStringsForWorker)
	})

	g.It("Author:sregidor-CPaasrunOnly-High-45239-KubeletConfig has a limit of 10 per cluster [Disruptive]", func() {
		g.By("Pause mcp worker")
		mcp := MachineConfigPool{name: "worker"}
		defer mcp.pause(oc, false)
		mcp.pause(oc, true)

		g.By("Create 10 kubelet config to add 500 max pods")
		allKcs := []KubeletConfig{}
		kcTemplate := generateTemplateAbsolutePath("change-maxpods-kubelet-config.yaml")
		for n := 1; n <= 10; n++ {
			kcName := fmt.Sprintf("change-maxpods-kubelet-config-%d", n)
			kc := KubeletConfig{name: kcName, template: kcTemplate}
			defer kc.delete(oc)
			kc.create(oc)
			allKcs = append(allKcs, kc)
			e2e.Logf("Created:\n %s", kcName)
		}

		g.By("Created kubeletconfigs must be successful")
		for _, kcItem := range allKcs {
			kcItem.waitUntilSuccess(oc, "10s")
		}

		g.By("Check that 10 machine configs were created")
		verifyKcRenderedMcs(oc, allKcs)

		g.By("Create a new Kubeletconfig. The 11th one")
		kcName := "change-maxpods-kubelet-config-11"
		kc := KubeletConfig{name: kcName, template: kcTemplate}
		defer kc.delete(oc)
		kc.create(oc)

		g.By("Created kubeletconfigs over the limit must report a failure regarding the 10 configs limit")
		expectedMsg := "could not get kubelet config key: max number of supported kubelet config (10) has been reached. Please delete old kubelet configs before retrying"
		kc.waitUntilFailure(oc, expectedMsg, "10s")

		g.By("Created kubeletconfigs inside the limit must be successful")
		for _, kcItem := range allKcs {
			kcItem.waitUntilSuccess(oc, "10s")
		}

		g.By("Check that only the right machine configs were created")
		allMcs := verifyKcRenderedMcs(oc, allKcs)

		kcCounter := 0
		for _, line := range allMcs {
			if strings.Contains(line, "generated-kubelet") {
				kcCounter++
			}
		}
		o.Expect(kcCounter).Should(o.Equal(10), "Only 10 Kubeletconfig resources should be generated")

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

func verifyKcRenderedMcs(oc *exutil.CLI, allKcs []KubeletConfig) []string {
	renderedConfs, renderedErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc").Output()
	o.Expect(renderedErr).NotTo(o.HaveOccurred())
	o.Expect(renderedConfs).NotTo(o.BeEmpty())
	slices := strings.Split(strings.Trim(renderedConfs, "'"), " ")
	for index := range allKcs {
		suffix := ""
		if index > 0 {
			suffix = fmt.Sprintf("-%d", index)
		}

		mcSubstring := "worker-generated-kubelet" + suffix
		e2e.Logf("Machine config '%s' should exist", mcSubstring)
		o.Expect(slices).Should(o.ContainElement(o.ContainSubstring(mcSubstring)))
	}
	return slices
}
