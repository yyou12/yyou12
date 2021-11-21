package clusterinfrastructure

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	clusterinfra "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc                        = exutil.NewCLI("cluster-autoscaler-operator", exutil.KubeConfigPath())
		autoscalerBaseDir         string
		clusterAutoscalerTemplate string
		machineAutoscalerTemplate string
		workLoadTemplate          string
		clusterAutoscaler         clusterAutoscalerDescription
		machineAutoscaler         machineAutoscalerDescription
		workLoad                  workLoadDescription
	)

	g.BeforeEach(func() {
		autoscalerBaseDir = exutil.FixturePath("testdata", "clusterinfrastructure", "autoscaler")
		clusterAutoscalerTemplate = filepath.Join(autoscalerBaseDir, "clusterautoscaler.yaml")
		machineAutoscalerTemplate = filepath.Join(autoscalerBaseDir, "machineautoscaler.yaml")
		workLoadTemplate = filepath.Join(autoscalerBaseDir, "workload.yaml")
		clusterAutoscaler = clusterAutoscalerDescription{
			maxNode:   100,
			minCore:   0,
			maxCore:   320000,
			minMemory: 0,
			maxMemory: 6400000,
			template:  clusterAutoscalerTemplate,
		}
		workLoad = workLoadDescription{
			name:      "workload",
			namespace: "openshift-machine-api",
			template:  workLoadTemplate,
		}
	})
	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-43174-ClusterAutoscaler CR could be deleted with foreground deletion", func() {
		g.By("Create clusterautoscaler")
		clusterAutoscaler.createClusterAutoscaler(oc)
		defer clusterAutoscaler.deleteClusterAutoscaler(oc)
		g.By("Delete clusterautoscaler with foreground deletion")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterautoscaler", "default", "--cascade=foreground").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterautoscaler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("default"))
	})

	//author: miyadav@redhat.com
	g.It("Longduration-NonPreRelease-Author:miyadav-Low-45430-MachineSet scaling from 0 should be evaluated correctly for the new or changed instance types [Serial][Slow][Disruptive]", func() {
		machineAutoscaler = machineAutoscalerDescription{
			name:           "machineautoscaler-45430",
			namespace:      "openshift-machine-api",
			maxReplicas:    1,
			minReplicas:    0,
			template:       machineAutoscalerTemplate,
			machineSetName: "machineset-45430",
		}

		g.By("Create machineset with instance type other than default in cluster")
		clusterinfra.SkipConditionally(oc)
		platform := clusterinfra.CheckPlatform(oc)
		if platform == "aws" {
			ms := clusterinfra.MachineSetDescription{"machineset-45430", 0}
			defer ms.DeleteMachineSet(oc)
			ms.CreateMachineSet(oc)
			g.By("Update machineset with instanceType")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/machineset-45430", "-n", "openshift-machine-api", "-p", `{"spec":{"template":{"spec":{"providerSpec":{"value":{"instanceType": "m5.4xlarge"}}}}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create MachineAutoscaler")
			defer machineAutoscaler.deleteMachineAutoscaler(oc)
			machineAutoscaler.createMachineAutoscaler(oc)

			g.By("Create clusterautoscaler")
			defer clusterAutoscaler.deleteClusterAutoscaler(oc)
			clusterAutoscaler.createClusterAutoscaler(oc)

			g.By("Create workload")
			defer workLoad.deleteWorkLoad(oc)
			workLoad.createWorkLoad(oc)

			g.By("Check machine could be created successful")
			// Creat a new machine taking roughly 5 minutes , set timeout as 7 minutes
			clusterinfra.WaitForMachinesRunning(oc, 1, "machineset-45430")
		}
	})
})
