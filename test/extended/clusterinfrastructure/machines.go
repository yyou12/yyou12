package clusterinfrastructure

import (
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	clusterinfra "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("machine-api-operator", exutil.KubeConfigPath())
	)
	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-45772-MachineSet selector is immutable", func() {
		g.By("Create a new machineset")
		clusterinfra.SkipConditionally(oc)
		ms := clusterinfra.MachineSetDescription{"machineset-45772", 0}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)
		g.By("Update machineset with empty clusterID")
		out, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/machineset-45772", "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"selector":{"matchLabels":{"machine.openshift.io/cluster-api-cluster": null}}}}`, "--type=merge").Output()
		o.Expect(out).To(o.ContainSubstring("selector is immutable"))
	})

	// author: huliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:huliu-Medium-45377-Enable accelerated network via MachineSets on Azure [Serial]", func() {
		g.By("Create a new machineset with acceleratedNetworking: true")
		if clusterinfra.CheckPlatform(oc) == "azure" {
			machinesetName := "machineset-45377"
			ms := clusterinfra.MachineSetDescription{machinesetName, 0}
			defer ms.DeleteMachineSet(oc)
			ms.CreateMachineSet(oc)
			g.By("Update machineset with acceleratedNetworking: true")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/"+machinesetName, "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"acceleratedNetworking":true,"vmSize":"Standard_D4s_v3"}}}}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			//test when set acceleratedNetworking: true, machine running needs nearly 9 minutes. so change the method timeout as 10 minutes.
			clusterinfra.WaitForMachinesRunning(oc, 1, machinesetName)

			g.By("Check machine with acceleratedNetworking: true")
			out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName, "-o=jsonpath={.items[0].spec.providerSpec.value.acceleratedNetworking}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("out:%s", out)
			o.Expect(out).To(o.ContainSubstring("true"))
		}
		e2e.Logf("Only azure platform supported for the test")
	})

	// author: huliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:huliu-Medium-46967-Implement Ephemeral OS Disks - OS cache placement on Azure [Disruptive]", func() {
		g.By("Create a new machineset with Ephemeral OS Disks - OS cache placement")
		if clusterinfra.CheckPlatform(oc) == "azure" {
			machinesetName := "machineset-46967"
			ms := clusterinfra.MachineSetDescription{machinesetName, 0}
			defer ms.DeleteMachineSet(oc)
			ms.CreateMachineSet(oc)
			g.By("Update machineset with Ephemeral OS Disks - OS cache placement")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/"+machinesetName, "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"vmSize":"Standard_D4s_v3","osDisk":{"diskSizeGB":30,"cachingType":"ReadOnly","diskSettings":{"ephemeralStorageLocation":"Local"},"managedDisk":{"storageAccountType":""}}}}}}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			clusterinfra.WaitForMachinesRunning(oc, 1, machinesetName)

			g.By("Check machine with Ephemeral OS Disks - OS cache placement")
			out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName, "-o=jsonpath={.items[0].spec.providerSpec.value.osDisk.diskSettings.ephemeralStorageLocation}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("out:%s", out)
			o.Expect(out).To(o.ContainSubstring("Local"))
		}
		e2e.Logf("Only azure platform supported for the test")
	})

	// author: huliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:huliu-Medium-47177-[MDH] PreDrain hooks prevent the machine from being drained at the draining phases [Disruptive]", func() {
		g.By("Create a new machineset with preDrain lifecycle hook")
		machinesetName := "machineset-47177"
		ms := clusterinfra.MachineSetDescription{machinesetName, 0}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)
		g.By("Update machineset with preDrain lifecycle hook")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("machineset/"+machinesetName, "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"template":{"spec":{"lifecycleHooks":{"preDrain":[{"name":"MigrateImportantApp","owner":"etcd"}]}}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		clusterinfra.WaitForMachinesRunning(oc, 1, machinesetName)

		g.By("Delete newly created machine by scaling machineset-47177 to 0")
		err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("--replicas=0", "-n", "openshift-machine-api", "machineset", machinesetName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for machine to go into Deleting phase")
		err = wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
			output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName, "-o=jsonpath={.items[0].status.phase}").Output()
			if output != "Deleting" {
				e2e.Logf("machine is not in Deleting phase and waiting up to 2 seconds ...")
				return false, nil
			}
			e2e.Logf("machine is in Deleting phase")
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Check machine phase failed")

		g.By("Check machine stuck in Deleting phase because of preDrain lifecycle hook")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName, "-o=jsonpath={.items[0].status.conditions[0]}").Output()
		e2e.Logf("out:%s", out)
		o.Expect(strings.Contains(out, "\"message\":\"Drain operation currently blocked by: [{Name:MigrateImportantApp Owner:etcd}]\"") && strings.Contains(out, "\"reason\":\"HookPresent\"") && strings.Contains(out, "\"status\":\"False\"") && strings.Contains(out, "\"type\":\"Drainable\"")).To(o.BeTrue())

		g.By("Update machine without preDrain lifecycle hook")
		machineName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("machine/"+machineName, "-n", "openshift-machine-api", "-p", `[{"op": "remove", "path": "/spec/lifecycleHooks/preDrain","value":[{"name":"MigrateImportantApp","owner":"etcd"}]}]`, "--type=json").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
