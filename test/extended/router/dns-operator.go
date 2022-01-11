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
	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-Critical-41050-DNS controll pod placement by tolerations [Disruptive]", func() {
		var (
			dns_master_toleration = "[{\"op\":\"replace\", \"path\":\"/spec/nodePlacement\", \"value\":{\"tolerations\":[" +
				"{\"effect\":\"NoExecute\",\"key\":\"my-dns-test\", \"operators\":\"Equal\", \"value\":\"abc\"}]}}]"
		)
		g.By("check the dns pod placement to confirm it is running on default mode")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/os=linux"))

		g.By("check dns pod placement to confirm it is running on default tolerations")
		output_lcfg, err_lcfg := oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.tolerations}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`{"key":"node-role.kubernetes.io/master","operator":"Exists"}`))

		g.By("Patch dns operator config with custom tolerations of dns pod, not to tolerate master node taints)")
		dnspodname := getDNSPodName(oc)
		defer restoreDNSOperatorDefault(oc)
		patchGlobalResourceAsAdmin(oc, "dns.operator.openshift.io/default", dns_master_toleration)
		output_lcfg, err_lcfg = oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.tolerations}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`{"effect":"NoExecute","key":"my-dns-test","value":"abc"}`))
		err = waitForResourceToDisappear(oc, "openshift-dns", "pod/"+dnspodname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+dnspodname))

		g.By("check dns.operator status to see any error messages")
		output_lcfg, err_lcfg = oc.AsAdmin().Run("get").Args("dns.operator/default", "-o=jsonpath={.status}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).NotTo(o.ContainSubstring("error"))
	})
})
