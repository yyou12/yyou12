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

	var oc = exutil.NewCLI("storage-aws-csi", exutil.KubeConfigPath())

	// aws-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider := getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "aws") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
	})

	// author: pewang@redhat.com
	// [AWS-EBS-CSI] [Dynamic PV] volumes should store data and allow exec of files
	g.It("Author:pewang-Critical-24485-Pod is running with dynamic created csi volume", func() {
		var (
			exampleTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(exampleTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(exampleTeamBaseDir, "pvc-template.yaml")
			podTemplate          = filepath.Join(exampleTeamBaseDir, "pod-template.yaml")
			storageClass         = newStorageClass(setStorageClassTemplate(storageClassTemplate))
			pvc                  = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			pod                  = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		// Use the framework created project as default, if use your own, exec the follow code setupProject
		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project
		pvc.namespace = oc.Namespace()
		pod.namespace = pvc.namespace

		g.By("1. Create aws-ebs-csi storageclass")
		storageClass.create(oc)
		defer storageClass.delete(oc) // ensure the storageclass is deleted whether the case exist normally or not.

		g.By("2. Create a pvc with the aws-ebs-csi storageclass")
		pvc.create(oc)
		defer pvc.delete(oc)

		g.By("3. Check the pvc status is Pending")
		o.Expect(getPersistentVolumeClaimStatus(oc, pvc.namespace, pvc.name)).To(o.Equal("Pending"))

		g.By("4. Create pod with the created pvc and wait for the pod ready")
		pod.create(oc)
		defer pod.delete(oc)
		waitPodReady(oc, pod.namespace, pod.name)

		g.By("5. Check the pvc status to Bound")
		o.Expect(getPersistentVolumeClaimStatus(oc, pvc.namespace, pvc.name)).To(o.Equal("Bound"))

		g.By("6. Check the pvc volume can be read and write")
		execCommandInSpecificPod(oc, pod.namespace, pod.name, "echo \"storge test\" > /mnt/storage/testfile")
		o.Expect(execCommandInSpecificPod(oc, pod.namespace, pod.name, "cat /mnt/storage/testfile")).To(o.ContainSubstring("storge test"))

		g.By("7. Check the pvc volume have the exec right")
		execCommandInSpecificPod(oc, pod.namespace, pod.name, "cp hello /mnt/storage/")
		o.Expect(execCommandInSpecificPod(oc, pod.namespace, pod.name, "./mnt/storage/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))
	})
})
