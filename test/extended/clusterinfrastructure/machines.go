package clusterinfrastructure

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	ci "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("machine-api-operator", exutil.KubeConfigPath())
	)
	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-42280-ClusterID should not be required to create a working machineSet", func() {
		g.By("Create a new machineset")
		ci.SkipConditionally(oc)
		ms := ci.MachineSetDescription{"machineset-42280", 0}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)
		g.By("Update machineset with empty clusterID")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/machineset-42280", "-n", "openshift-machine-api", "-p", `{"metadata":{"labels":{"machine.openshift.io/cluster-api-cluster": null}},"spec":{"replicas":1,"selector":{"matchLabels":{"machine.openshift.io/cluster-api-cluster": null}},"template":{"metadata":{"labels":{"machine.openshift.io/cluster-api-cluster":null}}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check machine could be created successful")
		// Creat a new machine taking roughly 5 minutes , set timeout as 7 minutes
		ci.WaitForMachinesRunning(oc, 1, "machineset-42280")
	})

	// author: huliu@redhat.com
	g.It("Longduration-CPaasrunOnly-Author:huliu-Medium-45377-Enable accelerated network via MachineSets on Azure [Serial]", func() {
		g.By("Create a new machineset with acceleratedNetworking: true")
		machinesetName := "machineset-45377"
		ms := ci.MachineSetDescription{machinesetName, 0}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)
		g.By("Update machineset with acceleratedNetworking: true")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/"+machinesetName, "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"acceleratedNetworking":true}}}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		//test when set acceleratedNetworking: true, machine running needs nearly 9 minutes. so change the method timeout as 10 minutes.
		ci.WaitForMachinesRunning(oc, 1, machinesetName)

		g.By("Check machine with acceleratedNetworking: true")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName, "-o=jsonpath={.items[0].spec.providerSpec.value.acceleratedNetworking}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("out:%s", out)
		o.Expect(out).To(o.ContainSubstring("true"))
	})
})
