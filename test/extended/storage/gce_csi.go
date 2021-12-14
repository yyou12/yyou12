package storage

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("storage-gcp-csi", exutil.KubeConfigPath())

	// gcp-csi test suite cloud provider support check

	g.BeforeEach(func() {
		cloudProvider := getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "gcp") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
	})

	// author: chaoyang@redhat.com
	// [GKE-PD-CSI] [Dynamic Regional PV]Check provisioned region pv and drain node function
	g.It("Author:chaoyang-Critical-37490-[GKE-PD-CSI] Check provisioned region pv and drain node function [Disruptive]", func() {
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate   = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
			storageClass         = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("pd.csi.storage.gke.io"))

			pvc = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name),
				setPersistentVolumeClaimCapacity("200Gi"))

			dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			storageClassParameters = map[string]string{

				"type":             "pd-standard",
				"replication-type": "regional-pd"}
			extraParameters = map[string]interface{}{
				"parameters":           storageClassParameters,
				"allowVolumeExpansion": true,
			}
		)

		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project
		pvc.namespace = oc.Namespace()
		dep.namespace = pvc.namespace

		g.By("1. Create gcp-pd-csi storageclass")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.deleteAsAdmin(oc)

		g.By("2. Create a pvc with the gcp-pd-csi storageclass")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("3. Create deployment with the created pvc and wait for the pod ready")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)

		g.By("4. Wait for the deployment ready")
		dep.waitReady(oc)
		podLabel := "app=" + dep.applabel

		g.By("5. Drain the pod to other nodes")
		nodeName0 := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
		nodeZone0, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes/"+nodeName0, "-o=jsonpath={.metadata.labels.topology\\.gke\\.io\\/zone}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		errDrain := oc.AsAdmin().WithoutNamespace().Run("adm").Args("drain", "nodes/"+nodeName0, "--pod-selector", podLabel).Execute()
		o.Expect(errDrain).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("uncordon", "nodes/"+nodeName0).Execute()

		g.By("6. Wait for the pod ready again")
		dep.waitReady(oc)
		nodeName1 := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
		nodeZone1, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes/"+nodeName1, "-o=jsonpath={.metadata.labels.topology\\.gke\\.io\\/zone}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7. Check node0 and node1 are in different zones")
		o.Expect(nodeZone0).ShouldNot(o.Equal(nodeZone1))

	})

	// author: chaoyang@redhat.com
	// [GKE-PD-CSI] [Dynamic Regional PV]Provision region pv with allowedTopologies
	g.It("Author:chaoyang-High-37514-[GKE-PD-CSI] Check provisioned region pv with allowedTopologies", func() {
		zones := getZonesFromWorker(oc)
		if len(zones) < 2 {
			g.Skip("Have less than 2 zones - skipping test ... ")
		}
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate   = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
			storageClass         = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("pd.csi.storage.gke.io"))

			pvc = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name),
				setPersistentVolumeClaimCapacity("200Gi"))

			dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			storageClassParameters = map[string]string{
				"type":             "pd-standard",
				"replication-type": "regional-pd"}
			matchLabelExpressions = []map[string]interface{}{
				{"key": "topology.gke.io/zone",
					"values": zones[:2],
				},
			}
			allowedTopologies = []map[string]interface{}{
				{"matchLabelExpressions": matchLabelExpressions},
			}
			extraParameters = map[string]interface{}{
				"parameters":        storageClassParameters,
				"allowedTopologies": allowedTopologies,
			}
		)

		g.By("Create default namespace")
		oc.SetupProject() //create new project

		g.By("Create storage class with allowedTopologies")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.deleteAsAdmin(oc)

		g.By("Create pvc")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment with the created pvc and wait for the pod ready")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)

		g.By("Wait for the deployment ready")
		dep.waitReady(oc)

		g.By("Check pv nodeAffinity is two items")
		pvName := pvc.getVolumeName(oc)
		outPut, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.nodeAffinity.required.nodeSelectorTerms}").Output()
		o.Expect(outPut).To(o.ContainSubstring(zones[0]))
		o.Expect(outPut).To(o.ContainSubstring(zones[1]))

	})
})
