package securityandcompliance

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-isc] Security_and_Compliance Pre-check and post-check for compliance operator upgrade", func() {
	defer g.GinkgoRecover()
	const (
		ns1 = "openshift-compliance"
		ns2 = "openshift-compliance2"
	)
	var (
		oc = exutil.NewCLI("compliance-"+getRandomString(), exutil.KubeConfigPath())
	)

	g.Context("When the compliance-operator is installed", func() {
		var (
			buildPruningBaseDir        = exutil.FixturePath("testdata", "securityandcompliance")
			scansettingbindingTemplate = filepath.Join(buildPruningBaseDir, "scansettingbinding.yaml")
			ssb                        = scanSettingBindingDescription{
				name:            "cossb1",
				namespace:       "",
				profilekind1:    "Profile",
				profilename1:    "rhcos4-moderate",
				profilename2:    "ocp4-moderate",
				scansettingname: "default",
				template:        scansettingbindingTemplate,
			}
			ssb2 = scanSettingBindingDescription{
				name:            "cossb2",
				namespace:       "",
				profilekind1:    "Profile",
				profilename1:    "ocp4-cis-node",
				profilename2:    "ocp4-cis",
				scansettingname: "default",
				template:        scansettingbindingTemplate,
			}
		)

		g.BeforeEach(func() {
			g.By("Check csv and pods for ns1 !!!")
			rsCsvName := getResourceNameWithKeywordForNamespace(oc, "csv", "compliance-operator", ns1)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", rsCsvName, "-n", ns1,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				ns1, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				ns1, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			g.By("Check Compliance Operator & profileParser pods are in running state !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
				ns1, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				ns1, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				ns1, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

			g.By("Check csv and pods for ns2 !!!")
			rsCsvName2 := getResourceNameWithKeywordForNamespace(oc, "csv", "compliance-operator", ns2)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", rsCsvName2, "-n", ns2,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				ns2, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				ns2, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			g.By("Check Compliance Operator & profileParser pods are in running state !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
				ns2, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				ns2, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				ns2, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-CPaasrunOnly-NonPreRelease-High-37721-High-37824-precheck for compliance operator", func() {
			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = ns1
			ssb2.namespace = ns2
			err2 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ssb.template, "-p", "NAME="+ssb.name, "NAMESPACE="+ssb.namespace,
				"PROFILENAME1="+ssb.profilename1, "PROFILEKIND1="+ssb.profilekind1, "PROFILENAME2="+ssb.profilename2, "SCANSETTINGNAME="+ssb.scansettingname)
			o.Expect(err2).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			err3 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ssb2.template, "-p", "NAME="+ssb2.name, "NAMESPACE="+ssb2.namespace,
				"PROFILENAME1="+ssb2.profilename1, "PROFILEKIND1="+ssb2.profilekind1, "PROFILENAME2="+ssb2.profilename2, "SCANSETTINGNAME="+ssb2.scansettingname)
			o.Expect(err3).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb2.name, ok, []string{"scansettingbinding", "-n", ssb2.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status, name and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "RUNNING", ok, []string{"compliancesuite", ssb.name, "-n", ns1,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "RUNNING", ok, []string{"compliancesuite", ssb2.name, "-n", ns2,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "AGGREGATING", ok, []string{"compliancesuite", ssb.name, "-n", ns1,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ns1,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb2.name, "-n", ns2,
				"-o=jsonpath={.status.phase}"}).check(oc)
			checkComplianceSuiteResult(oc, ns1, ssb.name, "NON-COMPLIANT INCONSISTENT")
			checkComplianceSuiteResult(oc, ns2, ssb2.name, "NON-COMPLIANT INCONSISTENT")
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-CPaasrunOnly-NonPreRelease-High-37721-High-37824-postcheck for compliance operator", func() {
			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", ns1, ssb.name},
				objectTableRef{"scansettingbinding", ns2, ssb2.name},
				objectTableRef{"profilebundle.compliance", ns1, "ocp4"},
				objectTableRef{"profilebundle.compliance", ns2, "ocp4"},
				objectTableRef{"profilebundle.compliance", ns1, "rhcos4"},
				objectTableRef{"profilebundle.compliance", ns2, "rhcos4"},
				objectTableRef{"project", ns1, ns1},
				objectTableRef{"project", ns2, ns2})

			g.By("Trigger rescan using oc-compliance plugin.. !!")
			_, err := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", ns1).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err2 := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb2.name, "-n", ns2).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())

			g.By("Check ComplianceSuite status, name and result after first rescan.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "RUNNING", ok, []string{"compliancesuite", ssb.name, "-n", ns1,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "RUNNING", ok, []string{"compliancesuite", ssb2.name, "-n", ns2,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "AGGREGATING", ok, []string{"compliancesuite", ssb.name, "-n", ns1,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ns1,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb2.name, "-n", ns2,
				"-o=jsonpath={.status.phase}"}).check(oc)
			checkComplianceSuiteResult(oc, ns1, ssb.name, "NON-COMPLIANT INCONSISTENT")
			checkComplianceSuiteResult(oc, ns2, ssb2.name, "NON-COMPLIANT INCONSISTENT")
		})
	})
})
