package clusterinfrastructure

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc                           = exutil.NewCLI("machine-api-operator", exutil.KubeConfigPath())
		clusterInfrastructureBaseDir = exutil.FixturePath("testdata", "clusterinfrastructure")
		iaasPlatform                 string
	)

	g.BeforeEach(func() {
		iaasPlatform = checkPlatform(oc)
	})

	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-42280-ClusterID should not be required to create a working machineSet", func() {
		g.By("Create a new machineset")
		checkInstallMethod(oc)
		ms := setMachineSetTemplate(oc, iaasPlatform, clusterInfrastructureBaseDir)
		ms.name = "machineset-42280"
		ms.replicas = 0
		ms.createMachineSet(oc)
		defer ms.deleteMachineSet(oc)
		g.By("Update machineset with empty clusterID")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/machineset-42280", "-n", "openshift-machine-api", "-p", `{"metadata":{"labels":{"machine.openshift.io/cluster-api-cluster": null}},"spec":{"replicas":1,"selector":{"matchLabels":{"machine.openshift.io/cluster-api-cluster": null}},"template":{"metadata":{"labels":{"machine.openshift.io/cluster-api-cluster":null}}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check machine could be created successful")
		// Creat a new machine taking roughly 5 minutes , set timeout as 7 minutes
		waitMachinesRunning(oc, 1, "machineset-42280")
	})
})
