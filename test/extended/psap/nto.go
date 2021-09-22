package psap

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] PSAP should", func() {
	defer g.GinkgoRecover()

	var (
		oc                  = exutil.NewCLI("nto-test", exutil.KubeConfigPath())
		machineNTONamespace = "openshift-cluster-node-tuning-operator"
		override_file       = exutil.FixturePath("testdata", "psap", "override.yaml")
		isNTO               bool
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
		tunedPod, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", machineNTONamespace, "--field-selector=spec.nodeName="+workerNodeName, "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Tuned Pod: %v", tunedPod)

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
		err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", machineNTONamespace, tunedPod, "--", "mount").Execute()
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
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", override_file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", workerNodeName, "tuned.openshift.io/override=").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check if new NTO profile was applied")
		err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			podLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", machineNTONamespace, "--tail=9", tunedPod).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			isApplied := strings.Contains(podLogs, "tuned.daemon.daemon: static tuning from profile 'override' applied")
			if !isApplied {
				e2e.Logf("Profile has not yet been applied to %v - retrying...", tunedPod)
				return false, nil
			}
			e2e.Logf("Profile has been applied to %v - continuing...", tunedPod)
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Profile was not applied to %v within timeout limit (30 seconds)", tunedPod))

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
})
