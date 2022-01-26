package storage

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("storage-azure-csi", exutil.KubeConfigPath())

	// azure-disk-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "azure") {
			g.Skip("Skip for non-supported cloud provider: *" + cloudProvider + "* !!!")
		}
	})

	// author: wduan@redhat.com
	// OCP-47001 - [Azure-Disk-CSI-Driver] support different skuName in storageclass
	g.It("Author:wduan-High-47001-[Azure-Disk-CSI-Driver] support different skuName in storageclass", func() {
		// Set the resource template for the scenario
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			podTemplate          = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
		)

		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Define the supported skuname
		// Currently, UltraSSD_LRS/StandardSSD_ZRS/Premium_ZRS are not supported in our CI/test env
		skunames := []string{"Premium_LRS", "StandardSSD_LRS", "Standard_LRS"}

		for _, skuname := range skunames {
			g.By("******" + " The skuname: " + skuname + " test phase start " + "******")

			// Set the resource definition for the scenario
			storageClassParameters := map[string]string{
				"skuname": skuname,
			}
			extraParameters := map[string]interface{}{
				"parameters": storageClassParameters,
			}

			storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("disk.csi.azure.com"))
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			pod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))

			g.By("Create csi storageclass with skuname")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("Create pod with the created pvc and wait for the pod ready")
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			pod.waitReady(oc)

			g.By("Check the pod volume can be read and write")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("Check the pv.spec.csi.volumeAttributes.skuname")
			pvName := pvc.getVolumeName(oc)
			skuname_pv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes.skuname}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The skuname in PV is: %v.", skuname_pv)
			o.Expect(skuname_pv).To(o.Equal(skuname))

		}
	})
})
