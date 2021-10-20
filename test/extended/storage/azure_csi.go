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

	var oc = exutil.NewCLI("storage-azure-csi", exutil.KubeConfigPath())

	// azure-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider := getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "azure") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
	})

	// author: ropatil@redhat.com
	// [Azure-CSI] [Dynamic PV][ext4] volumes should store data and allow exec of files
	g.It("Author:ropatil-44903-Pod is running with dynamic created csi volume with ext4 paramters", func() {

		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate   = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
			storageClass         = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("disk.csi.azure.com"))
			pvc                  = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))

			dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			storageClassParameters = map[string]string{
				"fstype": "ext4",
			}
			extraParameters = map[string]interface{}{
				"parameters":           storageClassParameters,
				"allowVolumeExpansion": true,
			}
		)

		// Use the framework created project as default, if use your own, exec the follow code setupProject
		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project
		pvc.namespace = oc.Namespace()
		dep.namespace = pvc.namespace

		g.By("1. Create azure-csi storageclass")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.delete(oc) // ensure the storageclass is deleted whether the case exist normally or not.

		g.By("2. Create a pvc with the azure-csi storageclass")
		pvc.create(oc)
		defer pvc.delete(oc)

		g.By("3. Create deployment with the created pvc and wait for the pod ready")
		dep.create(oc)
		defer dep.delete(oc)

		g.By("4. Get the pod status by label Running")
		checkPodStatusByLabel(oc, dep.namespace, "app=mydep", "Running")
		nodeHostNames, _ := getNodeListForPodByLabel(oc, dep.namespace, "app=mydep")

		g.By("5. Check the pod and node sc type parameters")
		o.Expect(execCommandInSpecificPodWithLabel(oc, dep.namespace, "app=mydep", "mount | grep \"/mnt/storage\"")).To(o.ContainSubstring("ext4"))
		o.Expect(checkVolumeMountInNode(oc, dep.namespace, nodeHostNames, pvc.name, "mount | grep ")).To(o.ContainSubstring("ext4"))

		g.By("6. Check the pod volume can be read and write")
		execCommandInSpecificPodWithLabel(oc, dep.namespace, "app=mydep", "echo \"storage test\" > /mnt/storage/testfile")
		o.Expect(execCommandInSpecificPodWithLabel(oc, dep.namespace, "app=mydep", "cat /mnt/storage/testfile")).To(o.ContainSubstring("storage test"))

		g.By("7. Check the pod volume have the exec right")
		execCommandInSpecificPodWithLabel(oc, dep.namespace, "app=mydep", "cp /hello /mnt/storage/")
		o.Expect(execCommandInSpecificPodWithLabel(oc, dep.namespace, "app=mydep", "/mnt/storage/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))

		g.By("8. Scale down the replicas number to 0")
		dep.scaleReplicas(oc, "0")

		g.By("9. Wait untill all the pods are gone and check node has no mounted volume")
		WaitUntilPodsAreGoneByLabel(oc, dep.namespace, "app=mydep")
		msg, _ := checkVolumeMountInNode(oc, dep.namespace, nodeHostNames, pvc.name, "mount | grep ")
		o.Expect(strings.Contains(msg, "ext4")).To(o.BeFalse())

		g.By("10. Scale up the replicas number to 1")
		dep.scaleReplicas(oc, "1")

		g.By("11. Get the pod status by label Running")
		checkPodStatusByLabel(oc, dep.namespace, "app=mydep", "Running")

		g.By("12. Check the pod volume contents and exec right after scaling")
		o.Expect(execCommandInSpecificPodWithLabel(oc, dep.namespace, "app=mydep", "cat /mnt/storage/testfile")).To(o.ContainSubstring("storage test"))
		o.Expect(execCommandInSpecificPodWithLabel(oc, dep.namespace, "app=mydep", "/mnt/storage/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))
	})
})
