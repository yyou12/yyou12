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
	g.It("Author:zhsun-Medium-45772-MachineSet selector does is immutable", func() {
		g.By("Create a new machineset")
		ci.SkipConditionally(oc)
		ms := ci.MachineSetDescription{"machineset-45772", 0}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)
		g.By("Update machineset with empty clusterID")
		out, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/machineset-45772", "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"selector":{"matchLabels":{"machine.openshift.io/cluster-api-cluster": null}}}}`, "--type=merge").Output()
		o.Expect(out).To(o.ContainSubstring("selector is immutable"))
	})

	// author: huliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:huliu-Medium-45377-Enable accelerated network via MachineSets on Azure [Serial]", func() {
		g.By("Create a new machineset with acceleratedNetworking: true")
		machinesetName := "machineset-45377"
		ms := ci.MachineSetDescription{machinesetName, 0}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)
		g.By("Update machineset with acceleratedNetworking: true")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/"+machinesetName, "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"acceleratedNetworking":true,"vmSize":"Standard_D4s_v3"}}}}}}`, "--type=merge").Execute()
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
