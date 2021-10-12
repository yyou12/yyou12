package psap

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] PSAP should", func() {
	defer g.GinkgoRecover()

	var (
		oc                          = exutil.NewCLI("nto-test", exutil.KubeConfigPath())
		machineNTONamespace         = "openshift-cluster-node-tuning-operator"
		override_file               = exutil.FixturePath("testdata", "psap", "override.yaml")
		pod_test_file               = exutil.FixturePath("testdata", "psap", "pod_test.yaml")
		tuned_nf_conntrack_max_file = exutil.FixturePath("testdata", "psap", "tuned-nf-conntrack-max.yaml")
		isNTO                       bool
	)

	g.BeforeEach(func() {
		// ensure NTO operator is installed
		isNTO = isPodInstalled(oc, machineNTONamespace)
	})

	// author: nweinber@redhat.com
	g.It("Author:nweinber-Medium-29789-Sysctl parameters set by tuned can not be overwritten by parameters set via /etc/sysctl", func() {

		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		g.By("Pick one worker node and one tuned pod on said node")
		workerNodeName, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Worker Node: %v", workerNodeName)
		tunedPodName, err := getTunedPod(oc, machineNTONamespace, workerNodeName)
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
		err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", machineNTONamespace, tunedPodName, "--", "mount").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check sysctl kernel.pid_max on node and store the value")
		kernel, err := exutil.DebugNodeWithChroot(oc, workerNodeName, "sysctl", "kernel.pid_max")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kernel).To(o.ContainSubstring("kernel.pid_max"))
		pid_max_value := getKernelPidMaxValue(kernel)
		e2e.Logf("kernel.pid_max has value of: %v", pid_max_value)

		defer func() {
			g.By("Removed tuned override and label after test completion")
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", machineNTONamespace, "tuneds.tuned.openshift.io", "override").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", workerNodeName, "tuned.openshift.io/override-").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Create new CR and label the node")
		err = createFromCustomResource(oc, machineNTONamespace, override_file)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", workerNodeName, "tuned.openshift.io/override=").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check if new NTO profile was applied")
		assertIfTunedProfileApplied(oc, machineNTONamespace, tunedPodName, "override")

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
	g.It("Author:nweinber-Medium-33237-Test NTO support for operatorapi Unmanaged state", func() {

		// test requires NTO to be installed
		if !isNTO {
			g.Skip("NTO is not installed - skipping test ...")
		}

		defer func() {
			g.By("Remove custom profile (if not already removed) and patch default tuned back to Managed")
			_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", machineNTONamespace, "tuned", "nf-conntrack-max", "--ignore-not-found").Execute()
			_ = patchTunedState(oc, machineNTONamespace, "Managed")
		}()

		g.By("Create logging namespace")
		oc.SetupProject()
		loggingNamespace := oc.Namespace()

		g.By("Patch default tuned to 'Unmanaged'")
		err := patchTunedState(oc, machineNTONamespace, "Unmanaged")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err := getTunedState(oc, machineNTONamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.Equal("Unmanaged"))

		g.By("Create new pod from CR and label it")
		err = createFromCustomResource(oc, loggingNamespace, pod_test_file)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.LabelPod(oc, loggingNamespace, "web", "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get the tuned node and pod names")
		tunedNodeName, err := exutil.GetPodNodeName(oc, loggingNamespace, "web")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Node: %v", tunedNodeName)
		tunedPodName, err := getTunedPod(oc, machineNTONamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Pod: %v", tunedPodName)

		g.By("Create new profile from CR")
		err = createFromCustomResource(oc, machineNTONamespace, tuned_nf_conntrack_max_file)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check logs, profiles, and nodes (profile changes SHOULD NOT be applied since tuned is UNMANAGED)")
		render_check, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", machineNTONamespace, "tuned", "rendered", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(render_check).NotTo(o.ContainSubstring("nf-conntrack-max"))

		logs_check, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", machineNTONamespace, "--tail=9", tunedPodName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logs_check).NotTo(o.ContainSubstring("nf-conntrack-max"))

		profile_check, err := getTunedProfile(oc, machineNTONamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profile_check).To(o.Equal("openshift-node"))

		nodeList, err := exutil.GetAllNodes(oc)
		nodeListSize := len(nodeList)
		for i := 0; i < nodeListSize; i++ {
			output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", "net.netfilter.nf_conntrack_max")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048576"))
		}

		g.By("Remove custom profile and pod and patch default tuned back to Managed")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", machineNTONamespace, "tuned", "nf-conntrack-max").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", loggingNamespace, "pod", "web").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = patchTunedState(oc, machineNTONamespace, "Managed")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err = getTunedState(oc, machineNTONamespace)
		o.Expect(state).To(o.Equal("Managed"))

		g.By("Create new pod from CR and label it")
		err = createFromCustomResource(oc, loggingNamespace, pod_test_file)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.LabelPod(oc, loggingNamespace, "web", "tuned.openshift.io/elasticsearch=")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get the tuned node and pod names")
		tunedNodeName, err = exutil.GetPodNodeName(oc, loggingNamespace, "web")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Node: %v", tunedNodeName)
		tunedPodName, err = getTunedPod(oc, machineNTONamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Pod: %v", tunedPodName)

		g.By("Create new profile from CR")
		err = createFromCustomResource(oc, machineNTONamespace, tuned_nf_conntrack_max_file)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check logs, profiles, and nodes (profile changes SHOULD be applied since tuned is MANAGED)")
		render_check, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", machineNTONamespace, "tuned", "rendered", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(render_check).To(o.ContainSubstring("nf-conntrack-max"))

		assertIfTunedProfileApplied(oc, machineNTONamespace, tunedPodName, "nf-conntrack-max")

		profile_check, err = getTunedProfile(oc, machineNTONamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profile_check).To(o.Equal("nf-conntrack-max"))

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
		err = patchTunedState(oc, machineNTONamespace, "Unmanaged")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err = getTunedState(oc, machineNTONamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.Equal("Unmanaged"))
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", machineNTONamespace, "tuned", "nf-conntrack-max").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check logs, profiles, and nodes (profile changes SHOULD NOT be applied since tuned is UNMANAGED)")
		render_check, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", machineNTONamespace, "tuned", "rendered", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(render_check).To(o.ContainSubstring("nf-conntrack-max"))

		profile_check, err = getTunedProfile(oc, machineNTONamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profile_check).To(o.Equal("nf-conntrack-max"))

		logs_check, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", machineNTONamespace, "--tail=9", tunedPodName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logs_check).To(o.ContainSubstring("tuned.daemon.daemon: static tuning from profile 'nf-conntrack-max' applied"))

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
		err = patchTunedState(oc, machineNTONamespace, "Managed")
		o.Expect(err).NotTo(o.HaveOccurred())
		state, err = getTunedState(oc, machineNTONamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.Equal("Managed"))

		g.By("Check logs, profiles, and nodes (profile changes SHOULD be applied since tuned is MANAGED)")
		render_check, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", machineNTONamespace, "tuned", "rendered", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(render_check).NotTo(o.ContainSubstring("nf-conntrack-max"))

		assertIfTunedProfileApplied(oc, machineNTONamespace, tunedPodName, "openshift-node")

		profile_check, err = getTunedProfile(oc, machineNTONamespace, tunedNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profile_check).To(o.Equal("openshift-node"))

		for i := 0; i < nodeListSize; i++ {
			output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", "net.netfilter.nf_conntrack_max")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("net.netfilter.nf_conntrack_max = 1048576"))
		}
	})

})
