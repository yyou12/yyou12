package storage

import (
	//"path/filepath"
	"fmt"
	"regexp"
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
		cloudProvider := getCloudProvider(oc)
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
})
