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

	var (
		oc            = exutil.NewCLI("storage-alibaba-csi", exutil.KubeConfigPath())
		cloudProvider string
	)

	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "alibabacloud") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
	})

	// author: ropatil@redhat.com
	// [Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: file system [ext4/ext3/xfs]
	g.It("Author:ropatil-Medium-47918-[Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: file system [ext4/ext3/xfs]", func() {
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project
		//Define the test scenario support fsTypes
		fsTypes := []string{"ext4", "ext3", "xfs"}
		for _, fsType := range fsTypes {
			// Set the resource template and definition for the scenario
			var (
				storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
				storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
				pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
				deploymentTemplate   = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
				storageClass         = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("diskplugin.csi.alibabacloud.com"))
				pvc                  = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name),
					setPersistentVolumeClaimCapacity("20Gi"))

				dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
				storageClassParameters = map[string]string{
					"csi.storage.k8s.io/fstype": fsType,
					"diskTags":                  "team:storage,user:Alitest",
				}
				extraParameters = map[string]interface{}{
					"parameters":           storageClassParameters,
					"allowVolumeExpansion": true,
				}
			)

			g.By("******" + cloudProvider + " csi driver: \"" + storageClass.provisioner + "\" for fsType: \"" + fsType + "\" test phase start" + "******")

			g.By("1. Create csi storageclass")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("2. Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("3. Create deployment with the created pvc and wait for the pod ready")
			dep.create(oc)
			defer dep.deleteAsAdmin(oc)

			g.By("4. Wait for the deployment ready")
			dep.waitReady(oc)

			g.By("5. Check volume have the diskTags attribute")
			volName := pvc.getVolumeName(oc)
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, "team:storage,user:Alitest")).To(o.BeTrue())

			g.By("6. Check the deployment's pod mounted volume can be read and write")
			dep.checkPodMountedVolumeCouldRW(oc)

			g.By("7. Check the deployment's pod mounted volume have the exec right")
			dep.checkPodMountedVolumeHaveExecRight(oc)

			g.By("******" + cloudProvider + " csi driver: \"" + storageClass.provisioner + "\" for fsType: \"" + fsType + "\" test phase finished" + "******")
		}
	})

	// author: ropatil@redhat.com
	// [Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: Block
	g.It("Author:ropatil-Medium-47919-[Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: Block", func() {
		// Set the resource template and definition for the scenario
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate   = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
			storageClass         = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("diskplugin.csi.alibabacloud.com"))
			pvc                  = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name),
				setPersistentVolumeClaimCapacity("20Gi"), setPersistentVolumeClaimVolumemode("Block"))

			dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name), setDeploymentVolumeType("volumeDevices"), setDeploymentVolumeTypePath("devicePath"), setDeploymentMountpath("/dev/dblock"))
			storageClassParameters = map[string]string{
				"diskTags": "team:storage,user:Alitest",
			}
			extraParameters = map[string]interface{}{
				"parameters":           storageClassParameters,
				"allowVolumeExpansion": true,
			}
		)
		// Set up a specified project share for all the phases
		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project

		g.By("******" + cloudProvider + " csi driver: \"" + storageClass.provisioner + "\"for Block volume mode test phase start" + "******")

		g.By("1. Create csi storageclass")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

		g.By("2. Create a pvc with the csi storageclass")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("3. Create deployment with the created pvc and wait for the pod ready")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)

		g.By("4. Wait for the deployment ready")
		dep.waitReady(oc)

		g.By("5 Check volume have the diskTags attribute")
		volName := pvc.getVolumeName(oc)
		o.Expect(checkVolumeCsiContainAttributes(oc, volName, "team:storage,user:Alitest")).To(o.BeTrue())

		g.By("6. Check the deployment's pod mounted volume can be read and write")
		dep.writeDataBlockType(oc)

		g.By("7. Check the deployment's pod mounted volume have the exec right")
		dep.checkDataBlockType(oc)

		g.By("******" + cloudProvider + " csi driver: \"" + storageClass.provisioner + "\" for Block volume mode test phase finished" + "******")
	})
})
