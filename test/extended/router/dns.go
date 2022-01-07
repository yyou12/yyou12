package router

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("coredns-upstream-resolvers-log", exutil.KubeConfigPath())
	// author: shudili@redhat.com
	g.It("Author:shudili-NonPreRelease-Critical-46868-Configure forward policy for CoreDNS flag [Disruptive]", func() {
		var (
			resourceName           = "dns.operator.openshift.io/default"
			cfg_mul_ipv4_upstreams = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" + 
			                         "{\"address\":\"10.100.1.11\",\"port\":53,\"type\":\"Network\"}, " + 
			                         "{\"address\":\"10.100.1.12\",\"port\":53,\"type\":\"Network\"}, " + 
			                         "{\"address\":\"10.100.1.13\",\"port\":5353,\"type\":\"Network\"}]}]"
			cfg_default_upstreams  = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"port\":53,\"type\":\"SystemResolvConf\"}]}]"
			cfg_policy_random      = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"Random\"}]"
			cfg_policy_rr          = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"RoundRobin\"}]"
			cfg_policy_seq         = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"Sequential\"}]"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Check default values of forward policy for CoreDNS")
		podList       := getAllDNSPodsNames(oc)
		dnspodname    := getRandomDNSPodName(podList)
		policy_output := readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))

		g.By("Patch dns operator with multiple ipv4 upstreams")
		dnspodname  =  getRandomDNSPodName(podList)
		attrList   :=  getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_mul_ipv4_upstreams)
		attrList    =  waitCorefileUpdated(oc, attrList)
		g.By("Check multiple ipv4 forward upstreams in CoreDNS")
		upstreams  :=  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring("forward . 10.100.1.11:53 10.100.1.12:53 10.100.1.13:5353"))
		g.By("Check default forward policy in the CM after multiple ipv4 forward upstreams are configured")
		output_pcfg,err_pcfg := oc.AsAdmin().Run("get").Args("cm/dns-default", "-n", "openshift-dns", "-o=jsonpath={.data.Corefile}").Output()
		o.Expect(err_pcfg).NotTo(o.HaveOccurred())
		o.Expect(output_pcfg).To(o.ContainSubstring("policy sequential"))
		g.By("Check default forward policy in CoreDNS after multiple ipv4 forward upstreams are configured")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))

		g.By("Patch dns operator with policy random for upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_policy_random)
		attrList   = waitCorefileUpdated(oc, attrList)
		g.By("Check forward policy random in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy random"))

		g.By("Patch dns operator with policy roundrobin for upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_policy_rr)
		attrList   = waitCorefileUpdated(oc, attrList)
		g.By("Check forward policy roundrobin in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy round_robin"))

		g.By("Patch dns operator with policy sequential for upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_policy_seq)
		attrList   = waitCorefileUpdated(oc, attrList)
		g.By("Check forward policy sequential in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))

		g.By("Patch dns operator with default upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_default_upstreams)
		attrList   = waitCorefileUpdated(oc, attrList)
		g.By("Check upstreams is restored to default in CoreDNS")
		upstreams  =  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring("forward . /etc/resolv.conf"))
		g.By("Check forward policy sequential in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))
	})

	// author: shudili@redhat.com
	g.It("Author:shudili-Critical-46872-Configure logLevel for CoreDNS under DNS operator flag [Disruptive]", func() {
		var (
			resourceName        = "dns.operator.openshift.io/default"
			cfg_loglevel_debug  = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"Debug\"}]"
			cfg_loglevel_trace  = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"Trace\"}]"
			cfg_loglevel_normal = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"Normal\"}]"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Check default log level of CoreDNS")
		podList    := getAllDNSPodsNames(oc)
		dnspodname := getRandomDNSPodName(podList)
		log_output := readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class error"))

		g.By("Patch dns operator with logLevel Debug for CoreDNS")
		dnspodname =  getRandomDNSPodName(podList)
		attrList  :=  getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_loglevel_debug)
		attrList   =  waitCorefileUpdated(oc, attrList)
		output_lcfg,err_lcfg := oc.AsAdmin().Run("get").Args("cm/dns-default", "-n", "openshift-dns", "-o=jsonpath={.data.Corefile}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring("class denial error"))
		g.By("Check log class for logLevel Debug in Corefile of coredns")
		log_output = readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class denial error"))

		g.By("Patch dns operator with logLevel Trace for CoreDNS")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_loglevel_trace)
		attrList   = waitCorefileUpdated(oc, attrList)
		g.By("Check log class for logLevel Trace in Corefile of coredns")
		log_output = readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class all"))

		g.By("Patch dns operator with logLevel Normal for CoreDNS")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_loglevel_normal)
		attrList   = waitCorefileUpdated(oc, attrList)
		g.By("Check log class for logLevel Trace in Corefile of coredns")
		log_output = readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class error"))
	})
})
