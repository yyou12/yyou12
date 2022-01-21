package nto

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] PSAP should", func() {
	defer g.GinkgoRecover()

	var (
		oc                               = exutil.NewCLI("nto-test", exutil.KubeConfigPath())
		ntoNamespace                     = "openshift-cluster-node-tuning-operator"
		override_file                    = exutil.FixturePath("testdata", "psap", "nto", "override.yaml")
		pod_test_file                    = exutil.FixturePath("testdata", "psap", "nto", "pod_test.yaml")
		pod_nginx_file                   = exutil.FixturePath("testdata", "psap", "nto", "pod-nginx.yaml")
		tuned_nf_conntrack_max_file      = exutil.FixturePath("testdata", "psap", "nto", "tuned-nf-conntrack-max.yaml")
		hp_performanceprofile_file       = exutil.FixturePath("testdata", "psap", "nto", "hp-performanceprofile.yaml")
		hp_performanceprofile_patch_file = exutil.FixturePath("testdata", "psap", "nto", "hp-performanceprofile-patch.yaml")
		custom_tuned_profile_file        = exutil.FixturePath("testdata", "psap", "nto", "custom-tuned-profiles.yaml")
		affine_default_cpuset_file       = exutil.FixturePath("testdata", "psap", "nto", "affine-default-cpuset.yaml")
		nto_tuned_debug_file             = exutil.FixturePath("testdata", "psap", "nto", "nto-tuned-debug.yaml")
		isNTO                            bool
		isAllInOne                       bool
	)

	g.BeforeEach(func() {
		// ensure NTO operator is installed
		isNTO = isPodInstalled(oc, ntoNamespace)
		isAllInOne = isAllInOneCluster(oc)
	})

	// author: nweinber@redhat.com
	g.It("Author:nweinber-Medium-29789-Sysctl parameters set by tuned can not be overwritten by parameters set via /etc/sysctl [Flaky]", func() {

		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		g.By("Pick one worker node and one tuned pod on said node")
		workerNodeName, err := exutil.GetFirstLinuxWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Worker Node: %v", workerNodeName)
		tunedPodName, err := exutil.GetPodName(oc, ntoNamespace, "", workerNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Pod: %v", tunedPodName)

		g.By("Check values set by /etc/sysctl on node and store the values")
		inotify, err := exutil.DebugNodeWithChroot(oc, workerNodeName, "cat", "/etc/sysctl.d/inotify.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(inotify).To(o.And(
			o.ContainSubstring("fs.inotify.max_user_watches"),
			o.ContainSubstring("fs.inotify.max_user_instances")))
		max_user_watches_value := getMaxUserWatchesValue(inotify)
		max_user_instances_value := getMaxUserInstancesValue(inotify)
		e2e.Logf("fs.inotify.max_user_watches has value of: %v", max_user_watches_value)
		e2e.Logf("fs.inotify.max_user_instances has value of: %v", max_user_instances_value)

		g.By("Mount /etc/sysctl on node")
		_, err = exutil.RemoteShPod(oc, ntoNamespace, tunedPodName, "mount")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check sysctl kernel.pid_max on node and store the value")
		kernel, err := exutil.DebugNodeWithChroot(oc, workerNodeName, "sysctl", "kernel.pid_max")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kernel).To(o.ContainSubstring("kernel.pid_max"))
		pid_max_value := getKernelPidMaxValue(kernel)
		e2e.Logf("kernel.pid_max has value of: %v", pid_max_value)

		defer func() {
			g.By("Removed tuned override and label after test completion")
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ntoNamespace, "tuneds.tuned.openshift.io", "override").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", workerNodeName, "tuned.openshift.io/override-").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Create new CR and label the node")
		exutil.CreateNsResourceFromTemplate(oc, ntoNamespace, "--ignore-unknown-parameters=true", "-f", override_file)
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", workerNodeName, "tuned.openshift.io/override=").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check if new NTO profile was applied")
		assertIfTunedProfileApplied(oc, ntoNamespace, tunedPodName, "override")

		g.By("Check value of fs.inotify.max_user_instances on node (set by sysctl, should be the same as before)")
		instanceCheck, err := exutil.DebugNodeWithChroot(oc, workerNodeName, "sysctl", "fs.inotify.max_user_instances")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(instanceCheck).To(o.ContainSubstring(max_user_instances_value))

		g.By("Check value of fs.inotify.max_user_watches on node (set by sysctl, should be the same as before)")
		watchesCheck, err := exutil.DebugNodeWithChroot(oc, workerNodeName, "sysctl", "fs.inotify.max_user_watches")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(watchesCheck).To(o.ContainSubstring(max_user_watches_value))

		g.By("Check value of kernel.pid_max on node (set by override tuned, should be different than before)")
		pidCheck, err := exutil.DebugNodeWithChroot(oc, workerNodeName, "sysctl", "kernel.pid_max")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pidCheck).To(o.ContainSubstring("kernel.pid_max = 1048576"))

	})

	// author: nweinber@redhat.com
	g.It("Author:nweinber-Medium-33237-Test NTO support for operatorapi Unmanaged state [Flaky]", func() {

		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		defer func() {
			g.By("Remove custom profile (if not already removed) and patch default tuned back to Managed")
			_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ntoNamespace, "tuned", "nf-conntrack-max", "--ignore-not-found").Execute()
			_ = patchTunedState(oc, ntoNamespace, "default", "Managed")
		}()

		g.By("Create logging namespace")
		oc.SetupProject()
		loggingNamespace := oc.Namespace()

		g.By("Patch default tuned to 'Unmanaged'")
		err := patchTunedState(oc, ntoNamespace, "default", "Unmanaged")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err := getTunedState(oc, ntoNamespace, "default")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.Equal("Unmanaged"))

		g.By("Create new pod from CR and label it")
		exutil.CreateNsResourceFromTemplate(oc, loggingNamespace, "--ignore-unknown-parameters=true", "-f", pod_test_file)
		err = exutil.LabelPod(oc, loggingNamespace, "web", "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get the tuned node and pod names")
		tunedNodeName, err := exutil.GetPodNodeName(oc, loggingNamespace, "web")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Node: %v", tunedNodeName)
		tunedPodName, err := exutil.GetPodName(oc, ntoNamespace, "", tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Pod: %v", tunedPodName)

		g.By("Create new profile from CR")
		exutil.CreateNsResourceFromTemplate(oc, ntoNamespace, "--ignore-unknown-parameters=true", "-f", tuned_nf_conntrack_max_file)

		g.By("Check logs, profiles, and nodes (profile changes SHOULD NOT be applied since tuned is UNMANAGED)")
		renderCheck, err := getTunedRender(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(renderCheck).NotTo(o.ContainSubstring("nf-conntrack-max"))

		logsCheck, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", ntoNamespace, "--tail=9", tunedPodName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logsCheck).NotTo(o.ContainSubstring("nf-conntrack-max"))

		profileCheck, err := getTunedProfile(oc, ntoNamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileCheck).To(o.Equal("openshift-node"))

		nodeList, err := exutil.GetAllNodesbyOSType(oc, "linux")
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeListSize := len(nodeList)
		for i := 0; i < nodeListSize; i++ {
			output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", "net.netfilter.nf_conntrack_max")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048576"))
		}

		g.By("Remove custom profile and pod and patch default tuned back to Managed")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ntoNamespace, "tuned", "nf-conntrack-max").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", loggingNamespace, "pod", "web").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = patchTunedState(oc, ntoNamespace, "default", "Managed")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err = getTunedState(oc, ntoNamespace, "default")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.Equal("Managed"))

		g.By("Create new pod from CR and label it")
		exutil.CreateNsResourceFromTemplate(oc, loggingNamespace, "--ignore-unknown-parameters=true", "-f", pod_test_file)
		err = exutil.LabelPod(oc, loggingNamespace, "web", "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get the tuned node and pod names")
		tunedNodeName, err = exutil.GetPodNodeName(oc, loggingNamespace, "web")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Node: %v", tunedNodeName)
		tunedPodName, err = exutil.GetPodName(oc, ntoNamespace, "", tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Pod: %v", tunedPodName)

		g.By("Create new profile from CR")
		exutil.CreateNsResourceFromTemplate(oc, ntoNamespace, "--ignore-unknown-parameters=true", "-f", tuned_nf_conntrack_max_file)

		g.By("Check logs, profiles, and nodes (profile changes SHOULD be applied since tuned is MANAGED)")
		renderCheck, err = getTunedRender(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(renderCheck).To(o.ContainSubstring("nf-conntrack-max"))

		assertIfTunedProfileApplied(oc, ntoNamespace, tunedPodName, "nf-conntrack-max")

		profileCheck, err = getTunedProfile(oc, ntoNamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileCheck).To(o.Equal("nf-conntrack-max"))

		// tuned nodes should have value of 1048578, others should be 1048576
		for i := 0; i < nodeListSize; i++ {
			output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", "net.netfilter.nf_conntrack_max")
			o.Expect(err).NotTo(o.HaveOccurred())
			if nodeList[i] == tunedNodeName {
				o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048578"))
			} else {
				o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048576"))
			}
		}

		g.By("Change tuned state back to Unmanaged and delete custom tuned")
		err = patchTunedState(oc, ntoNamespace, "default", "Unmanaged")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err = getTunedState(oc, ntoNamespace, "default")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.Equal("Unmanaged"))
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ntoNamespace, "tuned", "nf-conntrack-max").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check logs, profiles, and nodes (profile changes SHOULD NOT be applied since tuned is UNMANAGED)")
		renderCheck, err = getTunedRender(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(renderCheck).To(o.ContainSubstring("nf-conntrack-max"))

		profileCheck, err = getTunedProfile(oc, ntoNamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileCheck).To(o.Equal("nf-conntrack-max"))

		logsCheck, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", ntoNamespace, "--tail=9", tunedPodName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logsCheck).To(o.ContainSubstring("tuned.daemon.daemon: static tuning from profile 'nf-conntrack-max' applied"))

		// tuned nodes should have value of 1048578, others should be 1048576
		for i := 0; i < nodeListSize; i++ {
			output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", "net.netfilter.nf_conntrack_max")
			o.Expect(err).NotTo(o.HaveOccurred())
			if nodeList[i] == tunedNodeName {
				o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048578"))
			} else {
				o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048576"))
			}
		}

		g.By("Changed tuned state back to Managed")
		err = patchTunedState(oc, ntoNamespace, "default", "Managed")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err = getTunedState(oc, ntoNamespace, "default")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.Equal("Managed"))

		g.By("Check logs, profiles, and nodes (profile changes SHOULD be applied since tuned is MANAGED)")
		renderCheck, err = getTunedRender(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(renderCheck).NotTo(o.ContainSubstring("nf-conntrack-max"))

		assertIfTunedProfileApplied(oc, ntoNamespace, tunedPodName, "openshift-node")

		profileCheck, err = getTunedProfile(oc, ntoNamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileCheck).To(o.Equal("openshift-node"))

		for i := 0; i < nodeListSize; i++ {
			output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", "net.netfilter.nf_conntrack_max")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048576"))
		}
	})

	// author: nweinber@redhat.com
	g.It("Longduration-NonPreRelease-Author:nweinber-Medium-36881-Node Tuning Operator will provide machine config for the master machine config pool [Disruptive] [Slow]", func() {

		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		if !isAllInOne {
			g.Skip("It's not all in one cluster - skipping test ...")
		}

		defer func() {
			g.By("Remove new tuning profile after test completion")
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ntoNamespace, "tuneds.tuned.openshift.io", "openshift-node-performance-hp-performanceprofile").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Add new tuning profile from CR")
		exutil.CreateNsResourceFromTemplate(oc, ntoNamespace, "--ignore-unknown-parameters=true", "-f", hp_performanceprofile_file)

		g.By("Verify new tuned profile was created")
		profiles, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", "-n", ntoNamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profiles).To(o.ContainSubstring("openshift-node-performance-hp-performanceprofile"))

		g.By("Get NTO pod name and check logs for priority warning")
		ntoPodName, err := getNTOPodName(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("NTO pod name: %v", ntoPodName)
		ntoPodLogs, err := exutil.GetSpecificPodLogs(oc, ntoNamespace, "", ntoPodName, "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ntoPodLogs).To(o.ContainSubstring("profiles openshift-control-plane/openshift-node-performance-hp-performanceprofile have the same priority 30, please use a different priority for your custom profiles!"))

		g.By("Patch priority for openshift-node-performance-hp-performanceprofile tuned to 20")
		err = patchTunedPriority(oc, ntoNamespace, "openshift-node-performance-hp-performanceprofile", hp_performanceprofile_patch_file)
		o.Expect(err).NotTo(o.HaveOccurred())
		tunedPriority, err := getTunedPriority(oc, ntoNamespace, "openshift-node-performance-hp-performanceprofile")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tunedPriority).To(o.Equal("20"))

		g.By("Check Nodes for expected changes")
		assertIfNodeSchedulingDisabled(oc)

		g.By("Ensure the settings took effect on the master nodes")
		assertIfMasterNodeChangesApplied(oc)

		g.By("Check MachineConfig kernel arguments for expected changes")
		mcCheck, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mcCheck).To(o.ContainSubstring("50-nto-master"))
		mcKernelArgCheck, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("mc/50-nto-master").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mcKernelArgCheck).To(o.ContainSubstring("default_hugepagesz=2M"))

		g.By("Check MachineConfigPool for expected changes")
		assertIfMCPChangesApplied(oc)

	})

	g.It("Author:liqcui-Medium-23959-Test NTO for remove pod in daemon mode [Disruptive]", func() {

		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		ntoRes := ntoResource{
			name:        "user-max-cgroup-namespaces",
			namespace:   ntoNamespace,
			template:    custom_tuned_profile_file,
			sysctlparm:  "user.max_cgroup_namespaces",
			sysctlvalue: "128888",
		}
		defer func() {
			g.By("Remove custom profile (if not already removed) and patch default tuned back to Managed")
			ntoRes.delete(oc)
			_ = patchTunedState(oc, ntoNamespace, "default", "Managed")
		}()
		//Get the tuned pod name that run on first worker node
		tunedNodeName, err := exutil.GetFirstLinuxWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		tunedPodName := getTunedPodNamebyNodeName(oc, tunedNodeName, ntoNamespace)

		defer func() {
			g.By("Forcily delete labeled pod on first worker node after test case executed in case compareSysctlDifferentFromSpecifiedValueByName step failure")
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", tunedPodName, "-n", ntoNamespace, "--ignore-not-found").Execute()
		}()

		g.By("Apply new profile from CR")
		ntoRes.createTunedProfileIfNotExist(oc)

		g.By("Check current profile for each node")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "profile").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Current profile for each node: \n%v", output)

		g.By("Check all nodes for user.max_cgroup_namespaces value, all node should different from 128888")
		compareSysctlDifferentFromSpecifiedValueByName(oc, "user.max_cgroup_namespaces", "128888")

		g.By("Label tuned pod as tuned.openshift.io/elasticsearch=")
		err = exutil.LabelPod(oc, ntoNamespace, tunedPodName, "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check current profile for each node")
		ntoRes.assertTunedProfileApplied(oc)

		g.By("Compare if the value user.max_cgroup_namespaces in on node with labeled pod, should be 128888")
		compareSysctlValueOnSepcifiedNodeByName(oc, tunedNodeName, "user.max_cgroup_namespaces", "", "128888")

		g.By("Delete labeled tuned pod by name")
		oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", tunedPodName, "-n", ntoNamespace).Execute()

		g.By("Check all nodes for user.max_cgroup_namespaces value, all node should different from 128888")
		compareSysctlDifferentFromSpecifiedValueByName(oc, "user.max_cgroup_namespaces", "128888")

	})

	g.It("NonPreRelease-Author:liqcui-Medium-23958-Test NTO for label pod in daemon mode [Disruptive]", func() {

		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		ntoRes := ntoResource{
			name:        "user-max-ipc-namespaces",
			namespace:   ntoNamespace,
			template:    custom_tuned_profile_file,
			sysctlparm:  "user.max_ipc_namespaces",
			sysctlvalue: "121112",
		}
		defer func() {
			g.By("Remove custom profile (if not already removed) and patch default tuned back to Managed")
			ntoRes.delete(oc)
		}()
		//Get the tuned pod name that run on first worker node
		tunedNodeName, err := exutil.GetFirstLinuxWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		tunedPodName := getTunedPodNamebyNodeName(oc, tunedNodeName, ntoNamespace)

		defer func() {
			g.By("Forcily remove label from the pod on first worker node in case compareSysctlDifferentFromSpecifiedValueByName step failure")
			err = exutil.LabelPod(oc, ntoNamespace, tunedPodName, "tuned.openshift.io/elasticsearch-")
		}()

		g.By("Apply new profile from CR")
		ntoRes.createTunedProfileIfNotExist(oc)

		g.By("Check current profile for each node")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "profile").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Current profile for each node: \n%v", output)

		g.By("Check all nodes for user.max_ipc_namespaces value, all node should different from 121112")
		compareSysctlDifferentFromSpecifiedValueByName(oc, "user.max_ipc_namespaces", "121112")

		g.By("Label tuned pod as tuned.openshift.io/elasticsearch=")
		err = exutil.LabelPod(oc, ntoNamespace, tunedPodName, "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check current profile for each node")
		ntoRes.assertTunedProfileApplied(oc)

		g.By("Compare if the value user.max_ipc_namespaces in on node with labeled pod, should be 121112")
		compareSysctlValueOnSepcifiedNodeByName(oc, tunedNodeName, "user.max_ipc_namespaces", "", "121112")

		g.By("Remove label from tuned pod as tuned.openshift.io/elasticsearch-")
		err = exutil.LabelPod(oc, ntoNamespace, tunedPodName, "tuned.openshift.io/elasticsearch-")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all nodes for user.max_ipc_namespaces value, all node should different from 121112")
		compareSysctlDifferentFromSpecifiedValueByName(oc, "user.max_ipc_namespaces", "121112")

	})

	g.It("NonPreRelease-Author:liqcui-Medium-43173-POD should be affined to the default cpuset [Disruptive]", func() {
		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		//Get the tuned pod name that run on first worker node
		tunedNodeName, err := exutil.GetFirstLinuxWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		tunedPodName := getTunedPodNamebyNodeName(oc, tunedNodeName, ntoNamespace)

		g.By("Remove custom profile (if not already removed) and remove node label")
		defer exutil.CleanupOperatorResourceByYaml(oc, ntoNamespace, affine_default_cpuset_file)

		defer func() {
			err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", tunedNodeName, "affine-default-cpuset-").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Label the node with affine-default-cpuset ")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", tunedNodeName, "affine-default-cpuset=", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create new NTO profile")
		exutil.ApplyOperatorResourceByYaml(oc, ntoNamespace, affine_default_cpuset_file)

		g.By("Check if new NTO profile was applied")
		assertIfTunedProfileApplied(oc, ntoNamespace, tunedPodName, "affine-default-cpuset-profile")

		g.By("Check current profile for each node")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "profile").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Current profile for each node: \n%v", output)

		g.By("Verified test case results ...")
		finalResult := assertAffineDefaultCPUSets(oc, tunedPodName, ntoNamespace)
		o.Expect(finalResult).To(o.Equal(true))

	})

	g.It("NonPreRelease-Author:liqcui-Medium-27491-Add own custom profile to tuned operator [Disruptive]", func() {
		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		ntoRes := ntoResource{
			name:        "user-max-mnt-namespaces",
			namespace:   ntoNamespace,
			template:    custom_tuned_profile_file,
			sysctlparm:  "user.max_mnt_namespaces",
			sysctlvalue: "142214",
		}

		oc.SetupProject()
		ntoTestNS := oc.Namespace()
		//Clean up the custom profile user-max-mnt-namespaces and unlabel the nginx pod
		defer ntoRes.delete(oc)

		//Create a nginx web application pod
		g.By("Create a nginx web pod in nto temp namespace")
		exutil.ApplyOperatorResourceByYaml(oc, ntoTestNS, pod_nginx_file)

		//Check if nginx pod is ready
		exutil.AssertPodToBeReady(oc, "nginx", ntoTestNS)

		//Get the node name in the same node as nginx app
		tunedNodeName, err := exutil.GetPodNodeName(oc, ntoTestNS, "nginx")
		o.Expect(err).NotTo(o.HaveOccurred())

		//Get the tuned pod name in the same node as nginx app
		tunedPodName := getTunedPodNamebyNodeName(oc, tunedNodeName, ntoNamespace)

		//Get NTO operator pod name
		ntoOperatorPod, err := getNTOPodName(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		//Label pod nginx with tuned.openshift.io/elasticsearch=
		g.By("Label nginx pod as tuned.openshift.io/elasticsearch=")
		err = exutil.LabelPod(oc, ntoTestNS, "nginx", "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		//Apply new profile that match label tuned.openshift.io/elasticsearch=
		g.By("Apply new profile from CR")
		ntoRes.createTunedProfileIfNotExist(oc)

		g.By("Check if new profile in in rendered tuned")
		renderCheck, err := getTunedRender(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(renderCheck).To(o.ContainSubstring("user-max-mnt-namespaces"))

		//Verify if the new profile is applied
		assertIfTunedProfileApplied(oc, ntoNamespace, tunedPodName, "user-max-mnt-namespaces")
		profileCheck, err := getTunedProfile(oc, ntoNamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileCheck).To(o.Equal("user-max-mnt-namespaces"))

		//Verify nto operator logs
		assertNTOOperatorLogs(oc, ntoNamespace, ntoOperatorPod, "user-max-mnt-namespaces")

		g.By("Check current profile for each node")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "profile").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Current profile for each node: \n%v", output)

		g.By("Compare if the value user.max_mnt_namespaces in on node with labeled pod, should be 142214")
		compareSysctlValueOnSepcifiedNodeByName(oc, tunedNodeName, "user.max_mnt_namespaces", "", "142214")

		g.By("Delete custom profile")
		ntoRes.delete(oc)

		//Check if restore to default profile.
		isSNO := isSNOCluster(oc)
		if isSNO {
			assertIfTunedProfileApplied(oc, ntoNamespace, tunedPodName, "openshift-control-plane")
			assertNTOOperatorLogs(oc, ntoNamespace, ntoOperatorPod, "openshift-control-plane")
			profileCheck, err := getTunedProfile(oc, ntoNamespace, tunedNodeName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(profileCheck).To(o.Equal("openshift-control-plane"))
		} else {
			assertIfTunedProfileApplied(oc, ntoNamespace, tunedPodName, "openshift-node")
			assertNTOOperatorLogs(oc, ntoNamespace, ntoOperatorPod, "openshift-node")
			profileCheck, err := getTunedProfile(oc, ntoNamespace, tunedNodeName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(profileCheck).To(o.Equal("openshift-node"))
		}

		g.By("Check all nodes for user.max_mnt_namespaces value, all node should different from 142214")
		compareSysctlDifferentFromSpecifiedValueByName(oc, "user.max_mnt_namespaces", "142214")
	})

	g.It("NonPreRelease-Author:liqcui-Medium-37125-Turning on debugging for tuned containers.[Disruptive]", func() {
		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		ntoRes := ntoResource{
			name:        "user-max-net-namespaces",
			namespace:   ntoNamespace,
			template:    nto_tuned_debug_file,
			sysctlparm:  "user.max_net_namespaces",
			sysctlvalue: "101010",
		}

		var (
			isEnableDebug bool
			isDebugInLog  bool
		)

		//Clean up the custom profile user-max-mnt-namespaces
		defer ntoRes.delete(oc)

		//Create a temp namespace to deploy nginx pod
		oc.SetupProject()
		ntoTestNS := oc.Namespace()

		//Create a nginx web application pod
		g.By("Create a nginx web pod in nto temp namespace")
		exutil.ApplyOperatorResourceByYaml(oc, ntoTestNS, pod_nginx_file)

		//Check if nginx pod is ready
		exutil.AssertPodToBeReady(oc, "nginx", ntoTestNS)

		//Get the node name in the same node as nginx app
		tunedNodeName, err := exutil.GetPodNodeName(oc, ntoTestNS, "nginx")
		o.Expect(err).NotTo(o.HaveOccurred())

		//Get the tuned pod name in the same node as nginx app
		tunedPodName := getTunedPodNamebyNodeName(oc, tunedNodeName, ntoNamespace)

		//To reset tuned pod log, forcily to delete tuned pod
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", tunedPodName, "-n", ntoNamespace, "--ignore-not-found=true").Execute()

		//Get NTO operator pod name
		ntoOperatorPod, err := getNTOPodName(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		//Label pod nginx with tuned.openshift.io/elasticsearch=
		g.By("Label nginx pod as tuned.openshift.io/elasticsearch=")
		err = exutil.LabelPod(oc, ntoTestNS, "nginx", "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		//Verify if debug was disabled by default
		g.By("Check node profile debug settings, it should be debug: false")
		isEnableDebug = assertDebugSettings(oc, tunedNodeName, ntoNamespace, "false")
		o.Expect(isEnableDebug).To(o.Equal(true))

		//Apply new profile that match label tuned.openshift.io/elasticsearch=
		g.By("Apply new profile from CR with debug setting is false")
		ntoRes.createDebugTunedProfileIfNotExist(oc, false)

		//Verify if the new profile is applied
		ntoRes.assertTunedProfileApplied(oc)
		profileCheck, err := getTunedProfile(oc, ntoNamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileCheck).To(o.Equal("user-max-net-namespaces"))

		g.By("Check if new profile in rendered tuned")
		renderCheck, err := getTunedRender(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(renderCheck).To(o.ContainSubstring("user-max-net-namespaces"))

		//Verify nto operator logs
		assertNTOOperatorLogs(oc, ntoNamespace, ntoOperatorPod, "user-max-net-namespaces")

		//Verify if debug is false by CR setting
		g.By("Check node profile debug settings, it should be debug: false")
		isEnableDebug = assertDebugSettings(oc, tunedNodeName, ntoNamespace, "false")
		o.Expect(isEnableDebug).To(o.Equal(true))

		//Check if the log contain debug, the expected result should be none
		g.By("Check if tuned pod log contains debug key word, the expected result should be no DEBUG")
		isDebugInLog = exutil.AssertOprPodLogsbyFilter(oc, tunedPodName, ntoNamespace, "DEBUG", 2)
		o.Expect(isDebugInLog).To(o.Equal(false))

		g.By("Delete custom profile and will apply a new one ...")
		ntoRes.delete(oc)

		g.By("Apply new profile from CR with debug setting is true")
		ntoRes.createDebugTunedProfileIfNotExist(oc, true)

		//Verify if the new profile is applied
		ntoRes.assertTunedProfileApplied(oc)
		profileCheck, err = getTunedProfile(oc, ntoNamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileCheck).To(o.Equal("user-max-net-namespaces"))

		g.By("Check if new profile in rendered tuned")
		renderCheck, err = getTunedRender(oc, ntoNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(renderCheck).To(o.ContainSubstring("user-max-net-namespaces"))

		//Verify nto operator logs
		assertNTOOperatorLogs(oc, ntoNamespace, ntoOperatorPod, "user-max-net-namespaces")

		//Verify if debug was enabled by CR setting
		g.By("Check if the debug is true in node profile, the expected result should be true")
		isEnableDebug = assertDebugSettings(oc, tunedNodeName, ntoNamespace, "true")
		o.Expect(isEnableDebug).To(o.Equal(true))

		//The log shouldn't contain debug in log
		g.By("Check if tuned pod log contains debug key word, the log should contain DEBUG")
		exutil.AssertOprPodLogsbyFilterWithDuration(oc, tunedPodName, ntoNamespace, "DEBUG", 60, 2)
	})
})
