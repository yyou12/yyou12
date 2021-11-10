package clusterinfrastructure

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("cluster-machine-approver", exutil.KubeConfigPath())
	)

	// author: huliu@redhat.com
	g.It("Author:huliu-Medium-45420-Cluster Machine Approver should use leader election", func() {
		attemptAcquireLeaderLeaseStr := "attempting to acquire leader lease openshift-cluster-machine-approver/cluster-machine-approver-leader..."
		acquiredLeaseStr := "successfully acquired lease openshift-cluster-machine-approver/cluster-machine-approver-leader"

		g.By("Check default pod is leader")
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", "openshift-cluster-machine-approver").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(podName) == 0 {
			g.Skip("Skip for no pod!")
		}
		logsOfPod, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podName, "-n", "openshift-cluster-machine-approver", "-c", "machine-approver-controller").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logsOfPod).To(o.ContainSubstring(attemptAcquireLeaderLeaseStr))
		o.Expect(logsOfPod).To(o.ContainSubstring(acquiredLeaseStr))

		defer oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", "machine-approver", "--replicas=1", "-n", "openshift-cluster-machine-approver").Execute()

		g.By("Scale the replica of ClusterMachineApprover to 2")
		err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", "machine-approver", "--replicas=2", "-n", "openshift-cluster-machine-approver").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for ClusterMachineApprover to scale")
		err = wait.Poll(3*time.Second, 90*time.Second, func() (bool, error) {
			output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "machine-approver", "-o=jsonpath={.status.availableReplicas}", "-n", "openshift-cluster-machine-approver").Output()
			readyReplicas, _ := strconv.Atoi(output)
			if readyReplicas != 2 {
				e2e.Logf("The scaled pod is not ready yet and waiting up to 3 seconds ...")
				return false, nil
			}
			e2e.Logf("The deployment machine-approver is successfully scaled")
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Check pod failed"))

		g.By("Check only one pod is leader")
		podNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].metadata.name}", "-n", "openshift-cluster-machine-approver").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podNameList := strings.Split(podNames, " ")

		logsOfPod1, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameList[0], "-n", "openshift-cluster-machine-approver", "-c", "machine-approver-controller").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logsOfPod1).To(o.ContainSubstring(attemptAcquireLeaderLeaseStr))

		logsOfPod2, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameList[1], "-n", "openshift-cluster-machine-approver", "-c", "machine-approver-controller").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logsOfPod2).To(o.ContainSubstring(attemptAcquireLeaderLeaseStr))

		//check only one pod is leader
		o.Expect((strings.Contains(logsOfPod1, acquiredLeaseStr) && !strings.Contains(logsOfPod2, acquiredLeaseStr)) || (!strings.Contains(logsOfPod1, acquiredLeaseStr) && strings.Contains(logsOfPod2, acquiredLeaseStr))).To(o.BeTrue())
	})
})
