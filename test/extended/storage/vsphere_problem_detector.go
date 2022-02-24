package storage

import (
	//"path/filepath"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("vsphere-problem-detector-operator", exutil.KubeConfigPath())

	// vsphere-problem-detector test suite infrastructure check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "vsphere") {
			g.Skip("Skip for non-supported infrastructure!!!")
		}
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-44254-[vsphere-problem-detector] should check the node hardware version and report in metric for alerter raising by CSO", func() {

		g.By("# Check HW version from vsphere-problem-detector-operator log")
		vpd_podlog, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment/vsphere-problem-detector-operator", "-n", "openshift-cluster-storage-operator", "--limit-bytes", "50000").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(vpd_podlog).NotTo(o.BeEmpty())
		o.Expect(vpd_podlog).To(o.ContainSubstring("has HW version vmx"))

		g.By("# Get the node hardware versioni")
		re := regexp.MustCompile(`HW version vmx-([0-9][0-9])`)
		match_res := re.FindStringSubmatch(vpd_podlog)
		hw_version := match_res[1]
		e2e.Logf("The node hardware version is %v", hw_version)

		g.By("# Check HW version from metrics")
		token := getSAToken(oc)
		url := "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=vsphere_node_hw_version_total"
		metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("prometheus-k8s-0", "-c", "prometheus", "-n", "openshift-monitoring", "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), url).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metrics).NotTo(o.BeEmpty())
		o.Expect(metrics).To(o.ContainSubstring("\"hw_version\":\"vmx-" + hw_version))

		g.By("# Check alert for if there is unsupported HW version")
		if hw_version == "13" || hw_version == "14" {
			e2e.Logf("Checking the CSIWithOldVSphereHWVersion alert")
			checkAlertRaised(oc, "CSIWithOldVSphereHWVersion")
		}
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-Medium-44664-The vSphere cluster is marked as unupgradable if vcenter, esxi versions or HW versions are unsupported", func() {
		g.By("# Get log from vsphere-problem-detector-operator")
		podlog, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment/vsphere-problem-detector-operator", "-n", "openshift-cluster-storage-operator").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		mes := map[string]string{
			"HW version":      "Marking cluster un-upgradeable because one or more VMs are on hardware version",
			"esxi version":    "Marking cluster un-upgradeable because host .* is on esxi version",
			"vCenter version": "Marking cluster un-upgradeable because connected vcenter is on",
		}
		for kind, expected_mes := range mes {
			g.By("# Check upgradeable status and reason is expected from clusterversion")
			e2e.Logf("%s: Check upgradeable status and reason is expected from clusterversion if %s not support", kind, kind)
			matched, _ := regexp.MatchString(expected_mes, podlog)
			if matched {
				reason, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].status.conditions[?(.type=='Upgradeable')].reason}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(reason).To(o.Equal("VSphereProblemDetectorController_VSphereOlderVersionDetected"))
				status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].status.conditions[?(.type=='Upgradeable')].status}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(status).To(o.Equal("False"))
				e2e.Logf("The cluster is marked as unupgradeable due to %s", kind)
			} else {
				e2e.Logf("The %s is supported", kind)
			}

		}
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-45514-[vsphere-problem-detector] should report metric about vpshere env", func() {
		// Add 'vsphere_rwx_volumes_total' metric from ocp 4.10
		g.By("Check metric: vsphere_vcenter_info, vsphere_esxi_version_total, vsphere_node_hw_version_total, vsphere_datastore_total, vsphere_rwx_volumes_total")
		checkStorageMetricsContent(oc, "vsphere_vcenter_info", "api_version")
		checkStorageMetricsContent(oc, "vsphere_esxi_version_total", "api_version")
		checkStorageMetricsContent(oc, "vsphere_node_hw_version_total", "hw_version")
		checkStorageMetricsContent(oc, "vsphere_datastore_total", "instance")
		checkStorageMetricsContent(oc, "vsphere_rwx_volumes_total", "value")
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-37728-[vsphere-problem-detector] should report vsphere_cluster_check_total metric correctly", func() {
		g.By("Check metric vsphere_cluster_check_total should contain CheckDefaultDatastore, CheckFolderPermissions, CheckTaskPermissions, CheckStorageClasses, ClusterInfo check.")
		metric := getStorageMetrics(oc, "vsphere_cluster_check_total")
		cluster_check_list := []string{"CheckDefaultDatastore", "CheckFolderPermissions", "CheckTaskPermissions", "CheckStorageClasses", "ClusterInfo"}
		for i := range cluster_check_list {
			o.Expect(metric).To(o.ContainSubstring(cluster_check_list[i]))
		}
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-37729-[vsphere-problem-detector] should report vsphere_node_check_total metric correctly", func() {
		g.By("Check metric vsphere_node_check_total should contain CheckNodeDiskUUID, CheckNodePerf, CheckNodeProviderID, CollectNodeESXiVersion, CollectNodeHWVersion.")
		metric := getStorageMetrics(oc, "vsphere_node_check_total")
		node_check_list := []string{"CheckNodeDiskUUID", "CheckNodePerf", "CheckNodeProviderID", "CollectNodeESXiVersion", "CollectNodeHWVersion"}
		for i := range node_check_list {
			o.Expect(metric).To(o.ContainSubstring(node_check_list[i]))
		}
	})

	// author:pewang@redhat.com
	// Since it'll restart deployment/vsphere-problem-detector-operator maybe conflict with the other vsphere-problem-detector cases,so set it as [Serial]
	g.It("NonPreRelease-Author:pewang-High-48763-[vsphere-problem-detector] should report 'vsphere_rwx_volumes_total' metric correctly [Serial]", func() {
		g.By("# Get the value of 'vsphere_rwx_volumes_total' metric real init value")
		// Define the CSO vsphere-problem-detector-operator deployment object
		detector := newDeployment(setDeploymentName("vsphere-problem-detector-operator"), setDeploymentNamespace("openshift-cluster-storage-operator"), setDeploymentApplabel("name=vsphere-problem-detector-operator"))
		orignReplicasNum, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", detector.name, "-n", detector.namespace, "-o", "jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		detector.replicasno = orignReplicasNum
		mo := newMonitor(oc.AsAdmin())
		// Restart vsphere-problem-detector-operator and get the init value of 'vsphere_rwx_volumes_total' metric
		detector.scaleReplicas(oc.AsAdmin(), "0")
		// VSphereProblemDetectorController will automated recover the dector replicas number
		detector.replicasno = orignReplicasNum
		detector.waitReady(oc.AsAdmin())
		// Get the report metric pod's new name (Restart deployment the pod name will changed)
		newInstanceName := detector.getPodList(oc.AsAdmin())[0]
		debugLogf("newInstanceName:%s", newInstanceName)
		// When the metric update by restart the instance the metric's pod's `data.result.0.metric.pod` name will change to the newInstanceName
		mo.waitSpecifiedMetricValueAsExpected("vsphere_rwx_volumes_total", `data.result.0.metric.pod`, newInstanceName)
		initCount, err := mo.getSpecifiedMetricValue("vsphere_rwx_volumes_total", `data.result.0.value.1`)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create two manual fileshare persist volumes(vSphere CNS File Volume) and one manual general volume")
		// The backend service count the total number of 'fileshare persist volumes' by only count the pvs which volumeHandle prefix with `file:`
		// https://github.com/openshift/vsphere-problem-detector/pull/64/files
		// So I create 2 pvs volumeHandle prefix with `file:` with different accessModes to check the count logic's accurateness
		storageTeamBaseDir := exutil.FixturePath("testdata", "storage")
		pvTemplate := filepath.Join(storageTeamBaseDir, "csi-pv-template.yaml")
		rwxPersistVolume := newPersistentVolume(setPersistentVolumeAccessMode("ReadWriteMany"), setPersistentVolumeHandle("file:a7d6fcdd-1cbd-4e73-a54f-a3c7"+getRandomString()), setPersistentVolumeTemplate(pvTemplate))
		rwxPersistVolume.create(oc)
		defer rwxPersistVolume.deleteAsAdmin(oc)
		rwoPersistVolume := newPersistentVolume(setPersistentVolumeAccessMode("ReadWriteOnce"), setPersistentVolumeHandle("file:a7d6fcdd-1cbd-4e73-a54f-a3c7"+getRandomString()), setPersistentVolumeTemplate(pvTemplate))
		rwoPersistVolume.create(oc)
		defer rwoPersistVolume.deleteAsAdmin(oc)
		generalPersistVolume := newPersistentVolume(setPersistentVolumeHandle("a7d6fcdd-1cbd-4e73-a54f-a3c7qawkdl"+getRandomString()), setPersistentVolumeTemplate(pvTemplate))
		generalPersistVolume.create(oc)
		defer generalPersistVolume.deleteAsAdmin(oc)

		g.By("# Check the metric update correctly")
		// Since the vsphere-problem-detector update the metric every hour restart the deployment to trigger the update right now
		detector.scaleReplicas(oc.AsAdmin(), "0")
		// VSphereProblemDetectorController will automated recover the dector replicas number
		detector.replicasno = orignReplicasNum
		detector.waitReady(oc.AsAdmin())
		// Wait for 'vsphere_rwx_volumes_total' metric value update correctly
		initCountInt, err := strconv.Atoi(initCount)
		o.Expect(err).NotTo(o.HaveOccurred())
		mo.waitSpecifiedMetricValueAsExpected("vsphere_rwx_volumes_total", `data.result.0.value.1`, interfaceToString(initCountInt+2))

		g.By("# Delete one RWX pv and wait for it deleted successfully")
		rwxPersistVolume.deleteAsAdmin(oc)
		waitForPersistentVolumeStatusAsExpected(oc, rwxPersistVolume.name, "deleted")

		g.By("# Check the metric update correctly again")
		detector.scaleReplicas(oc.AsAdmin(), "0")
		detector.replicasno = orignReplicasNum
		detector.waitReady(oc.AsAdmin())
		mo.waitSpecifiedMetricValueAsExpected("vsphere_rwx_volumes_total", `data.result.0.value.1`, interfaceToString(initCountInt+1))
	})
})
