package router

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("dns-operator", exutil.KubeConfigPath())
	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-Critical-41049-DNS controlls pod placement by node selector [Disruptive]", func() {
		var (
			dns_worker_nodeselector = "[{\"op\":\"add\", \"path\":\"/spec/nodePlacement/nodeSelector\", \"value\":{\"node-role.kubernetes.io/worker\":\"\"}}]"
			dns_master_nodeselector = "[{\"op\":\"replace\", \"path\":\"/spec/nodePlacement/nodeSelector\", \"value\":{\"node-role.kubernetes.io/master\":\"\"}}]"
		)

		g.By("check the default dns pod placement is present")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/os=linux"))

		g.By("Patch dns operator with worker as node selector in dns.operator default")
		dnspodname := getDNSPodName(oc)
		defer restoreDNSOperatorDefault(oc)
		patchGlobalResourceAsAdmin(oc, "dns.operator.openshift.io/default", dns_worker_nodeselector)
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/worker"))
		output_lcfg, err_lcfg := oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.nodeSelector}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`"node-role.kubernetes.io/worker":""`))
		err = waitForResourceToDisappear(oc, "openshift-dns", "pod/"+dnspodname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+dnspodname))

		g.By("Patch dns operator with master as node selector in dns.operator default")
		dnspodname1 := getDNSPodName(oc)
		patchGlobalResourceAsAdmin(oc, "dns.operator.openshift.io/default", dns_master_nodeselector)
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/master"))
		output_lcfg, err_lcfg = oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.nodeSelector}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`"node-role.kubernetes.io/master":""`))
		err = waitForResourceToDisappear(oc, "openshift-dns", "pod/"+dnspodname1)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+dnspodname1))
	})
})
