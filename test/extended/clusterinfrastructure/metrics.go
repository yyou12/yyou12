package clusterinfrastructure

import (
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("metrics", exutil.KubeConfigPath())
	)

	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-45499-mapi_current_pending_csr should reflect real pending CSR count", func() {
		g.By("Check the pending csr count")
		csrStatuses, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csr", "-o=jsonpath={.items[*].status.conditions[0].type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		csrStatusList := strings.Split(csrStatuses, " ")
		pending := 0
		for _, status := range csrStatusList {
			if status == "Pending" {
				pending++
			}
		}
		g.By("Get machine-approver-controller pod name")
		machineApproverPodName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", machineApproverNamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the value of mapi_current_pending_csr")
		token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(machineApproverPodName, "-c", "machine-approver-controller", "-n", machineApproverNamespace, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), "https://localhost:9192/metrics").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metrics).NotTo(o.BeEmpty())
		o.Expect(metrics).To(o.ContainSubstring("mapi_current_pending_csr " + strconv.Itoa(pending)))
	})
})
