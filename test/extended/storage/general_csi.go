package storage

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE WITH-TREARDOWN-PROJECT", func() {
	defer g.GinkgoRecover()
	var (
		oc            = exutil.NewCLI("storage-general-csi", exutil.KubeConfigPath())
		cloudProvider string
	)

	// aws-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		generalCsiSupportCheck(cloudProvider)
	})

	// author: pewang@redhat.com
	// OCP-44903 [CSI Driver] [Dynamic PV] [ext4] volumes should store data and allow exec of files on the volume
	g.It("Author:pewang-High-44903-[CSI Driver] [Dynamic PV] [ext4] volumes should store data and allow exec of files on the volume", func() {
		// Define the test scenario support provisioners
		scenarioSupportProvisioners := []string{"ebs.csi.aws.com", "disk.csi.azure.com", "cinder.csi.openstack.org", "pd.csi.storage.gke.io", "csi.vsphere.vmware.com"}
		// Set the resource template for the scenario
		var (
			storageTeamBaseDir     = exutil.FixturePath("testdata", "storage")
			storageClassTemplate   = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate            = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate     = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
			storageClassParameters = map[string]string{
				"csi.storage.k8s.io/fstype": "ext4",
			}
			extraParameters = map[string]interface{}{
				"parameters":           storageClassParameters,
				"allowVolumeExpansion": true,
			}
			supportProvisioners = sliceIntersect(scenarioSupportProvisioners, getSupportProvisionersByCloudProvider(cloudProvider))
		)
		if len(supportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner!!!")
		}
		// Set up a specified project share for all the phases
		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project
		for _, provisioner := range supportProvisioners {
			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase start" + "******")
			// Set the resource definition for the scenario
			storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate))
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			pvc.namespace = oc.Namespace()
			dep.namespace = pvc.namespace

			g.By("1. Create csi storageclass")
			storageClass.provisioner = provisioner
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

			g.By("5. Check the deployment's pod mounted volume fstype is ext4 by exec mount cmd in the pod")
			dep.checkPodMountedVolumeContain(oc, "ext4")

			g.By("6. Check the deployment's pod mounted volume can be read and write")
			dep.checkPodMountedVolumeCouldRW(oc)

			g.By("7. Check the deployment's pod mounted volume have the exec right")
			dep.checkPodMountedVolumeHaveExecRight(oc)

			g.By("8. Check the volume mounted on the pod located node")
			volName := pvc.getVolumeName(oc)
			nodeName := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
			checkVolumeMountCmdContain(oc, volName, nodeName, "ext4")

			g.By("9. Scale down the replicas number to 0")
			dep.scaleReplicas(oc, "0")

			g.By("10. Wait for the deployment scale down completed and check nodes has no mounted volume")
			dep.waitReady(oc)
			checkVolumeNotMountOnNode(oc, volName, nodeName)

			g.By("11. Scale up the deployment replicas number to 1")
			dep.scaleReplicas(oc, "1")

			g.By("12. Wait for the deployment scale up completed")
			dep.waitReady(oc)

			g.By("13. After scaled check the deployment's pod mounted volume contents and exec right")
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "cat /mnt/storage/testfile*")).To(o.ContainSubstring("storage test"))
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "/mnt/storage/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))

			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase finished" + "******")
		}
	})
	// author: pewang@redhat.com
	// [CSI Driver] [Dynamic PV] [Filesystem default] volumes should store data and allow exec of files
	g.It("Author:pewang-Critical-24485-[CSI Driver] [Dynamic PV] [Filesystem default] volumes should store data and allow exec of files", func() {
		// Define the test scenario support provisioners
		scenarioSupportProvisioners := []string{"ebs.csi.aws.com", "disk.csi.azure.com", "cinder.csi.openstack.org", "pd.csi.storage.gke.io", "csi.vsphere.vmware.com"}
		// Set the resource template for the scenario
		var (
			storageTeamBaseDir  = exutil.FixturePath("testdata", "storage")
			pvcTemplate         = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			podTemplate         = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
			supportProvisioners = sliceIntersect(scenarioSupportProvisioners, getSupportProvisionersByCloudProvider(cloudProvider))
		)
		if len(supportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner!!!")
		}
		for _, provisioner := range supportProvisioners {
			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase start" + "******")
			// Set the resource definition for the scenario
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate))
			pod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))

			// Use the framework created project as default, if use your own, exec the follow code setupProject
			g.By("0. Create new project for the scenario")
			oc.SetupProject() //create new project
			pvc.namespace = oc.Namespace()
			pod.namespace = pvc.namespace

			g.By("1. Create a pvc with the preset csi storageclass")
			pvc.scname = getPresetStorageClassNameByProvisioner(cloudProvider, provisioner)
			e2e.Logf("%s", pvc.scname)
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("2. Create pod with the created pvc and wait for the pod ready")
			pod.create(oc)
			defer pod.deleteAsadmin(oc)
			pod.waitReady(oc)

			g.By("3. Check the pod volume can be read and write")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("4. Check the pod volume have the exec right")
			pod.checkMountedVolumeHaveExecRight(oc)

			g.By("5. TearDown the project after test phase")
			oc.TeardownProject()

			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase finished" + "******")
		}
	})

	// author: wduan@redhat.com
	// OCP-44910 - [CSI-Driver] [Dynamic PV] [Filesystem default] support mountOptions
	g.It("Author:wduan-High-44910-[CSI Driver] [Dynamic PV] [Filesystem default] support mountOptions", func() {
		// Define the test scenario support provisioners
		scenarioSupportProvisioners := []string{"ebs.csi.aws.com", "disk.csi.azure.com", "cinder.csi.openstack.org", "pd.csi.storage.gke.io", "csi.vsphere.vmware.com"}
		// Set the resource template for the scenario
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate   = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
			mountOption          = []string{"debug", "discard"}
			extraParameters      = map[string]interface{}{
				"allowVolumeExpansion": true,
				"mountOptions":         mountOption,
			}
			supportProvisioners = sliceIntersect(scenarioSupportProvisioners, getSupportProvisionersByCloudProvider(cloudProvider))
		)
		if len(supportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner in " + cloudProvider + "!!!")
		}
		// Set up a specified project share for all the phases
		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project
		for _, provisioner := range supportProvisioners {
			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase start" + "******")
			// Set the resource definition for the scenario
			storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate))
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			pvc.namespace = oc.Namespace()
			dep.namespace = pvc.namespace

			g.By("1. Create csi storageclass")
			storageClass.provisioner = provisioner
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("2. Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("3. Create deployment with the created pvc")
			dep.create(oc)
			defer dep.deleteAsAdmin(oc)

			g.By("4. Wait for the deployment ready")
			dep.waitReady(oc)

			g.By("5. Check the deployment's pod mounted volume contains the mount option by exec mount cmd in the pod")
			dep.checkPodMountedVolumeContain(oc, "debug")
			dep.checkPodMountedVolumeContain(oc, "discard")

			g.By("6. Check the deployment's pod mounted volume can be read and write")
			dep.checkPodMountedVolumeCouldRW(oc)

			g.By("7. Check the deployment's pod mounted volume have the exec right")
			dep.checkPodMountedVolumeHaveExecRight(oc)

			g.By("8. Check the volume mounted contains the mount option by exec mount cmd in the node")
			volName := pvc.getVolumeName(oc)
			nodeName := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
			checkVolumeMountCmdContain(oc, volName, nodeName, "debug")
			checkVolumeMountCmdContain(oc, volName, nodeName, "discard")

			g.By("9. Scale down the replicas number to 0")
			dep.scaleReplicas(oc, "0")

			g.By("10. Wait for the deployment scale down completed and check nodes has no mounted volume")
			dep.waitReady(oc)
			checkVolumeNotMountOnNode(oc, volName, nodeName)

			g.By("11. Scale up the deployment replicas number to 1")
			dep.scaleReplicas(oc, "1")

			g.By("12. Wait for the deployment scale up completed")
			dep.waitReady(oc)

			g.By("13. After scaled check the deployment's pod mounted volume contents and exec right")
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "cat /mnt/storage/testfile*")).To(o.ContainSubstring("storage test"))
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "/mnt/storage/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))

			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase finished" + "******")
		}
	})

	// author: pewang@redhat.com
	// OCP-44904 [CSI Driver] [Dynamic PV] [xfs] volumes should store data and allow exec of files on the volume
	g.It("Author:pewang-High-44904-[CSI Driver] [Dynamic PV] [xfs] volumes should store data and allow exec of files on the volume", func() {
		// Define the test scenario support provisioners
		scenarioSupportProvisioners := []string{"ebs.csi.aws.com", "disk.csi.azure.com", "cinder.csi.openstack.org", "pd.csi.storage.gke.io", "csi.vsphere.vmware.com"}
		// Set the resource template for the scenario
		var (
			storageTeamBaseDir     = exutil.FixturePath("testdata", "storage")
			storageClassTemplate   = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate            = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate     = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
			storageClassParameters = map[string]string{
				"csi.storage.k8s.io/fstype": "xfs",
			}
			extraParameters = map[string]interface{}{
				"parameters":           storageClassParameters,
				"allowVolumeExpansion": true,
			}
			supportProvisioners = sliceIntersect(scenarioSupportProvisioners, getSupportProvisionersByCloudProvider(cloudProvider))
		)
		if len(supportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner!!!")
		}
		// Set up a specified project share for all the phases
		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project
		for _, provisioner := range supportProvisioners {
			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase start" + "******")
			// Set the resource definition for the scenario
			storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate))
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			pvc.namespace = oc.Namespace()
			dep.namespace = pvc.namespace

			g.By("1. Create csi storageclass")
			storageClass.provisioner = provisioner
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

			g.By("5. Check the deployment's pod mounted volume fstype is xfs by exec mount cmd in the pod")
			dep.checkPodMountedVolumeContain(oc, "xfs")

			g.By("6. Check the deployment's pod mounted volume can be read and write")
			dep.checkPodMountedVolumeCouldRW(oc)

			g.By("7. Check the deployment's pod mounted volume have the exec right")
			dep.checkPodMountedVolumeHaveExecRight(oc)

			g.By("8. Check the volume mounted on the pod located node")
			volName := pvc.getVolumeName(oc)
			nodeName := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
			checkVolumeMountCmdContain(oc, volName, nodeName, "xfs")

			g.By("9. Scale down the replicas number to 0")
			dep.scaleReplicas(oc, "0")

			g.By("10. Wait for the deployment scale down completed and check nodes has no mounted volume")
			dep.waitReady(oc)
			checkVolumeNotMountOnNode(oc, volName, nodeName)

			g.By("11. Scale up the deployment replicas number to 1")
			dep.scaleReplicas(oc, "1")

			g.By("12. Wait for the deployment scale up completed")
			dep.waitReady(oc)

			g.By("13. After scaled check the deployment's pod mounted volume contents and exec right")
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "cat /mnt/storage/testfile*")).To(o.ContainSubstring("storage test"))
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "/mnt/storage/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))

			g.By("******" + cloudProvider + " csi driver: \"" + provisioner + "\" test phase finished" + "******")
		}
	})

})
