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

	var oc = exutil.NewCLI("storage-vsphere-csi", exutil.KubeConfigPath())

	// vsphere-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider := getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "vsphere") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
	})

	// author: wduan@redhat.com
	g.It("Author:wduan-High-44257-[vSphere CSI Driver Operator] Create StorageClass along with a vSphere Storage Policy", func() {
		var (
			storageTeamBaseDir = exutil.FixturePath("testdata", "storage")
			pvcTemplate        = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			podTemplate        = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
			pvc                = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName("thin-csi"))
			pod                = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		// The storageclass/thin-csi should contain the .parameters.StoragePolicyName, and its value should be like "openshift-storage-policy-*"
		g.By("1. Check StoragePolicyName exist in storageclass/thin-csi")
		spn, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass/thin-csi", "-o=jsonpath={.parameters.StoragePolicyName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(spn).To(o.ContainSubstring("openshift-storage-policy"))

		// Basic check the provisioning with the storageclass/thin-csi
		g.By("2. Create new project for the scenario")
		oc.SetupProject() //create new project
		pvc.namespace = oc.Namespace()
		pod.namespace = pvc.namespace

		g.By("3. Create a pvc with the thin-csi storageclass")
		pvc.create(oc)
		defer pvc.delete(oc)

		g.By("4. Create pod with the created pvc and wait for the pod ready")
		pod.create(oc)
		defer pod.delete(oc)
		waitPodReady(oc, pod.namespace, pod.name)

		g.By("5. Check the pvc status to Bound")
		o.Expect(getPersistentVolumeClaimStatus(oc, pvc.namespace, pvc.name)).To(o.Equal("Bound"))
	})
})
