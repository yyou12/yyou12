package etcd

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-etcd] Etcd", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("openshift-etcd")

	// author: jgeorge@redhat.com
	g.It("Author:jgeorge-High-44199-run etcdctl check perf [Exclusive]", func() {

		g.By("Discover all the etcd pods")
		etcdPodList := getPodListByLabel(oc, "etcd=true")

		g.By("Run etcdctl check perf with medium workload")
		output, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-c", "etcdctl", "-n", "openshift-etcd", etcdPodList[0], "etcdctl", "check", "perf", "--auto-compact", "--auto-defrag", "--load=m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Medium workload result:\n%s", output))
	})
})
