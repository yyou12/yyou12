package securityandcompliance

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"path/filepath"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-isc] Security_and_Compliance The Compliance Operator automates compliance check for OpenShift and CoreOS", func() {
	defer g.GinkgoRecover()

	var (
		oc                         = exutil.NewCLI("compliance-"+getRandomString(), exutil.KubeConfigPath())
		dr                         = make(describerResrouce)
		buildPruningBaseDir        string
		ogCoTemplate               string
		catsrcCoTemplate           string
		subCoTemplate              string
		csuiteTemplate             string
		csuitetpcmTemplate         string
		csuitetaintTemplate        string
		csuitenodeTemplate         string
		csuiteSCTemplate           string
		cscanTemplate              string
		cscantaintTemplate         string
		cscantaintsTemplate        string
		cscanSCTemplate            string
		tprofileTemplate           string
		tprofileWithoutVarTemplate string
		scansettingTemplate        string
		scansettingbindingTemplate string
		pvextractpodYAML           string
		podModifyTemplate          string
		storageClassTemplate       string
		catSrc                     catalogSourceDescription
		ogD                        operatorGroupDescription
		subD                       subscriptionDescription
		podModifyD                 podModify
		storageClass               storageClassDescription
	)

	g.BeforeEach(func() {
		buildPruningBaseDir = exutil.FixturePath("testdata", "securityandcompliance")
		ogCoTemplate = filepath.Join(buildPruningBaseDir, "operator-group.yaml")
		catsrcCoTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subCoTemplate = filepath.Join(buildPruningBaseDir, "subscription.yaml")
		csuiteTemplate = filepath.Join(buildPruningBaseDir, "compliancesuite.yaml")
		csuitetpcmTemplate = filepath.Join(buildPruningBaseDir, "compliancesuitetpconfmap.yaml")
		csuitetaintTemplate = filepath.Join(buildPruningBaseDir, "compliancesuitetaint.yaml")
		csuitenodeTemplate = filepath.Join(buildPruningBaseDir, "compliancesuitenodes.yaml")
		csuiteSCTemplate = filepath.Join(buildPruningBaseDir, "compliancesuiteStorageClass.yaml")
		cscanTemplate = filepath.Join(buildPruningBaseDir, "compliancescan.yaml")
		cscantaintTemplate = filepath.Join(buildPruningBaseDir, "compliancescantaint.yaml")
		cscantaintsTemplate = filepath.Join(buildPruningBaseDir, "compliancescantaints.yaml")
		cscanSCTemplate = filepath.Join(buildPruningBaseDir, "compliancescanStorageClass.yaml")
		tprofileTemplate = filepath.Join(buildPruningBaseDir, "tailoredprofile.yaml")
		tprofileWithoutVarTemplate = filepath.Join(buildPruningBaseDir, "tailoredprofile-withoutvariable.yaml")
		scansettingTemplate = filepath.Join(buildPruningBaseDir, "scansetting.yaml")
		scansettingbindingTemplate = filepath.Join(buildPruningBaseDir, "scansettingbinding.yaml")
		pvextractpodYAML = filepath.Join(buildPruningBaseDir, "pv-extract-pod.yaml")
		podModifyTemplate = filepath.Join(buildPruningBaseDir, "pod_modify.yaml")
		storageClassTemplate = filepath.Join(buildPruningBaseDir, "storage_class.yaml")

		catSrc = catalogSourceDescription{
			name:        "compliance-operator",
			namespace:   "",
			displayName: "openshift-compliance-operator",
			publisher:   "Red Hat",
			sourceType:  "grpc",
			address:     "quay.io/openshift-qe-optional-operators/compliance-operator-index:v4.8",
			template:    catsrcCoTemplate,
		}
		ogD = operatorGroupDescription{
			name:      "openshift-compliance",
			namespace: "",
			template:  ogCoTemplate,
		}
		subD = subscriptionDescription{
			subName:                "compliance-operator",
			namespace:              "",
			channel:                "4.8",
			ipApproval:             "Automatic",
			operatorPackage:        "openshift-compliance-operator",
			catalogSourceName:      "compliance-operator",
			catalogSourceNamespace: "",
			startingCSV:            "",
			currentCSV:             "",
			installedCSV:           "",
			template:               subCoTemplate,
			singleNamespace:        true,
		}
		podModifyD = podModify{
			name:      "",
			namespace: "",
			nodeName:  "",
			args:      "",
			template:  podModifyTemplate,
		}
		storageClass = storageClassDescription{
			name:              "",
			provisioner:       "",
			reclaimPolicy:     "",
			volumeBindingMode: "",
			template:          storageClassTemplate,
		}

		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.getIr(itName).cleanup()
		dr.rmIr(itName)
	})

	// author: pdhamdhe@redhat.com
	g.It("Author:pdhamdhe-Critical-34378-Install the Compliance Operator through olm using CatalogSource and Subscription", func() {

		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catSrc.namespace = oc.Namespace()
		ogD.namespace = oc.Namespace()
		subD.namespace = oc.Namespace()
		subD.catalogSourceName = catSrc.name
		subD.catalogSourceNamespace = catSrc.namespace

		g.By("Create catalogSource !!!")
		e2e.Logf("Here catsrc namespace : %v\n", catSrc.namespace)
		catSrc.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", catSrc.name, "-n", catSrc.namespace,
			"-o=jsonpath={.status..lastObservedState}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "-n", catSrc.namespace,
			"-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Create operatorGroup !!!")
		ogD.create(oc, itName, dr)

		g.By("Create subscription for above catalogsource !!!")
		subD.create(oc, itName, dr)
		e2e.Logf("Here subscp namespace : %v\n", subD.namespace)
		newCheck("expect", asAdmin, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, []string{"sub", subD.subName, "-n",
			subD.namespace, "-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

		// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
		// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
		defer cleanupObjects(oc,
			objectTableRef{"profilebundle.compliance", subD.namespace, "ocp4"},
			objectTableRef{"profilebundle.compliance", subD.namespace, "rhcos4"},
			objectTableRef{"deployment", subD.namespace, "compliance-operator"})

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-e2e-test-compliance", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-e2e-test-compliance", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed !!! ")
	})

	g.Context("When the compliance-operator is installed", func() {

		var itName string

		g.BeforeEach(func() {
			oc.SetupProject()
			catSrc.namespace = oc.Namespace()
			ogD.namespace = oc.Namespace()
			subD.namespace = oc.Namespace()
			subD.catalogSourceName = catSrc.name
			subD.catalogSourceNamespace = catSrc.namespace
			itName = g.CurrentGinkgoTestDescription().TestText
			g.By("Create catalogSource !!!")
			e2e.Logf("Here catsrc namespace : %v\n", catSrc.namespace)
			catSrc.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", catSrc.name, "-n", catSrc.namespace,
				"-o=jsonpath={.status..lastObservedState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "-n", catSrc.namespace,
				"-o=jsonpath={.items[0].status.phase}"}).check(oc)

			g.By("Create operatorGroup !!!")
			ogD.create(oc, itName, dr)

			g.By("Create subscription for above catalogsource !!!")
			subD.create(oc, itName, dr)
			e2e.Logf("Here subscp namespace : %v\n", subD.namespace)
			newCheck("expect", asAdmin, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, []string{"sub", subD.subName, "-n",
				subD.namespace, "-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

			g.By("Check CSV is created sucessfully !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check Compliance Operator & profileParser pods are created !!!")
			newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
				"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-e2e-test-compliance", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-e2e-test-compliance", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check Compliance Operator & profileParser pods are in running state !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

			g.By("Compliance Operator sucessfully installed !!! ")
		})

		g.AfterEach(func() {
			g.By("Remove compliance-operator default objects")
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			cleanupObjects(oc,
				objectTableRef{"profilebundle.compliance", subD.namespace, "ocp4"},
				objectTableRef{"profilebundle.compliance", subD.namespace, "rhcos4"},
				objectTableRef{"deployment", subD.namespace, "compliance-operator"})
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-27649-The ComplianceSuite reports the scan result as Compliant or Non-Compliant", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}

				csuiteMD = complianceSuiteDescription{
					name:         "master-compliancesuite",
					namespace:    "",
					scanname:     "master-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"compliancesuite", subD.namespace, "master-compliancesuite"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			csuiteMD.namespace = subD.namespace
			g.By("Create master-compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker & master scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check master-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "master-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteMD.name, "NON-COMPLIANT")

			g.By("Check master-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check worker-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-27649 ComplianceScan has performed successfully... !!!!\n ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-32082-The ComplianceSuite shows the scan result NOT-APPLICABLE after all rules are skipped to scan", func() {

			var (
				csuite = complianceSuiteDescription{
					name:                "worker-compliancesuite",
					namespace:           "",
					scanname:            "worker-scan",
					profile:             "xccdf_org.ssgproject.content_profile_ncp",
					content:             "ssg-rhel7-ds.xml",
					contentImage:        "quay.io/complianceascode/ocp4:latest",
					noExternalResources: true,
					nodeSelector:        "wscan",
					template:            csuiteTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuite.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuite.create(oc, itName, dr)

			g.By("Check complianceSuite Status !!!\n")
			csuite.checkComplianceSuiteStatus(oc, "DONE")

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuite.name, "NOT-APPLICABLE")

			g.By("Check rule status through complianceCheckResult.. !!!\n")
			subD.getRuleStatus(oc, "SKIP")

			g.By("The ocp-32082 complianceScan has performed successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33398-The Compliance Operator supports to variables in tailored profile", func() {

			var (
				tprofileD = tailoredProfileDescription{
					name:         "rhcos-tailoredprofile",
					namespace:    "",
					extends:      "rhcos4-moderate",
					enrulename1:  "rhcos4-sshd-disable-root-login",
					disrulename1: "rhcos4-audit-rules-dac-modification-chmod",
					disrulename2: "rhcos4-audit-rules-etc-group-open",
					varname:      "rhcos4-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"tailoredprofile", subD.namespace, "rhcos-tailoredprofile"})

			tprofileD.namespace = subD.namespace
			g.By("Create tailoredprofile !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			tprofileD.create(oc, itName, dr)

			g.By("Check tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, "rhcos-tailoredprofile")

			g.By("Verify the tailoredprofile details through configmap ..!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "xccdf_org.ssgproject.content_rule_sshd_disable_root_login", ok,
				[]string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace, "-o=jsonpath={.data}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "xccdf_org.ssgproject.content_rule_audit_rules_dac_modification_chmod", ok,
				[]string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace, "-o=jsonpath={.data}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "xccdf_org.ssgproject.content_value_var_selinux_state", ok,
				[]string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace, "-o=jsonpath={.data}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "permissive", ok, []string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace,
				"-o=jsonpath={.data}"}).check(oc)

			g.By("ocp-33398 The Compliance Operator supported variables in tailored profile... !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-32840-The ComplianceSuite generates through ScanSetting CR", func() {

			var (
				tprofileD = tailoredProfileDescription{
					name:         "rhcos-tp",
					namespace:    "",
					extends:      "rhcos4-e8",
					enrulename1:  "rhcos4-sshd-disable-root-login",
					disrulename1: "rhcos4-no-empty-passwords",
					disrulename2: "rhcos4-audit-rules-dac-modification-chown",
					varname:      "rhcos4-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "myss",
					namespace:             "",
					roles1:                "master",
					roles2:                "worker",
					rotation:              10,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "co-requirement",
					namespace:       "",
					profilekind1:    "TailoredProfile",
					profilename1:    "rhcos-tp",
					profilename2:    "ocp4-moderate",
					scansettingname: "",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name},
				objectTableRef{"tailoredprofile", subD.namespace, ssb.profilename1})

			g.By("Check default profiles name rhcos4-e8 .. !!!\n")
			subD.getProfileName(oc, "rhcos4-e8")

			tprofileD.namespace = subD.namespace
			ssb.namespace = subD.namespace
			ss.namespace = subD.namespace
			ssb.scansettingname = ss.name

			g.By("Create tailoredprofile rhcos-tp !!!\n")
			tprofileD.create(oc, itName, dr)

			g.By("Verify tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, "rhcos-tp")

			g.By("Create scansetting !!!\n")

			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "myss", ok, []string{"scansetting", "-n", ss.namespace, ss.name,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "co-requirement", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker and master scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "co-requirement")
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify the disable rules are not available in compliancecheckresult.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-audit-rules-dac-modification-chown", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)

			g.By("ocp-32840 The ComplianceSuite generated successfully using scansetting... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33381-Verify the ComplianceSuite could be generated from Tailored profiles", func() {

			var (
				tprofileD = tailoredProfileDescription{
					name:         "rhcos-e8-tp",
					namespace:    "",
					extends:      "rhcos4-e8",
					enrulename1:  "rhcos4-sshd-disable-root-login",
					disrulename1: "rhcos4-no-empty-passwords",
					disrulename2: "rhcos4-audit-rules-dac-modification-chown",
					varname:      "rhcos4-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
				csuiteD = complianceSuiteDescription{
					name:               "rhcos-csuite",
					namespace:          "",
					scanname:           "rhcos-scan",
					profile:            "xccdf_compliance.openshift.io_profile_rhcos-e8-tp",
					content:            "ssg-rhcos4-ds.xml",
					contentImage:       "quay.io/complianceascode/ocp4:latest",
					nodeSelector:       "wscan",
					tailoringConfigMap: "rhcos-e8-tp-tp",
					template:           csuitetpcmTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "rhcos-csuite"},
				objectTableRef{"tailoredprofile", subD.namespace, "rhcos-e8-tp"})

			g.By("Check default profiles name rhcos4-e8 .. !!!\n")
			subD.getProfileName(oc, "rhcos4-e8")

			tprofileD.namespace = subD.namespace
			g.By("Create tailoredprofile !!!\n")
			tprofileD.create(oc, itName, dr)

			g.By("Check tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, "rhcos-e8-tp")

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check rhcos-csuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "rhcos-csuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check rhcos-csuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify the enable and disable rules.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-sshd-disable-root-login", ok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-audit-rules-dac-modification-chown", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)

			g.By("ocp-33381 The ComplianceSuite performed scan successfully using tailored profile... !!!\n")

		})
		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-33611-Verify the tolerations could work for compliancescan when there is more than one taint on node [Serial]", func() {
			var (
				cscanD = complianceScanDescription{
					name:         "example-compliancescan3",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "value1",
					operator:     "Equal",
					key1:         "key2",
					value1:       "value2",
					operator1:    "Equal",
					nodeSelector: "wscan",
					template:     cscantaintsTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Label and set taint value to one worker node.. !!!\n")
			nodeName := getOneWorkerNodeName(oc)
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule", "key2=value2:NoExecute")
			labelTaintNode(oc, "node", nodeName, "taint=true")
			defer func() {
				taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-", "key2=value2:NoExecute-")
				labelTaintNode(oc, "node", nodeName, "taint-")
			}()

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan.. !!!\n")
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceScan name and result.. !!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if scan pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, cscanD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove compliancescan object and recover tainted worker node.. !!!\n")
			cscanD.delete(itName, dr)

			g.By("ocp-33611 Verify the tolerations could work for compliancescan when there is more than one taints on node successfully.. !!!\n")

		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-High-37121-The ComplianceSuite generates through ScanSettingBinding CR with cis profile and default scansetting", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "cis-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-cis-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			g.By("Check default profiles name ocp4-cis .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			g.By("Check default profiles name ocp4-cis-node .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis-node")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker and master scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-37121 The ComplianceSuite generated successfully using scansetting CR and cis profile and default scansetting... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33713-The ComplianceSuite reports the scan result as Error", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_coreos-ncp",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "ERROR")

			g.By("Check complianceScan result through configmap exit-code and result from error-msg..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "1")
			subD.getScanResultFromConfigmap(oc, "No profile matching suffix \"xccdf_org.ssgproject.content_profile_coreos-ncp\" was found.")

			g.By("The ocp-33713 complianceScan has performed successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-27705-The ComplianceScan reports the scan result Compliant or Non-Compliant", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     cscanTemplate,
				}

				cscanMD = complianceScanDescription{
					name:         "master-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancescan", subD.namespace, "worker-scan"},
				objectTableRef{"compliancescan", subD.namespace, "master-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			cscanD.namespace = subD.namespace
			g.By("Create worker-scan !!!\n")
			cscanD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker & master scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, "master-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check master-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check worker-scan name and result..!!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check worker-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-27705 ComplianceScan has performed successfully... !!!! ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-27762-The ComplianceScan reports the scan result Error", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_coreos-ncp",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "wscan",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, "worker-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceScan name and result..!!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "ERROR")

			g.By("Check complianceScan result through configmap exit-code and result from error-msg..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "1")
			subD.getScanResultFromConfigmap(oc, "No profile matching suffix \"xccdf_org.ssgproject.content_profile_coreos-ncp\" was found.")

			g.By("The ocp-27762 complianceScan has performed successfully....!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-27968-Perform scan only on a subset of nodes using ComplianceScan object", func() {

			var (
				cscanMD = complianceScanDescription{
					name:         "master-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, "master-scan"})

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check master scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, "master-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check master-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-27968 ComplianceScan has performed successfully... !!!! ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33230-The compliance-operator raw result storage size is configurable", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					size:         "2Gi",
					template:     csuiteTemplate,
				}
				cscanMD = complianceScanDescription{
					name:         "master-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					size:         "3Gi",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"compliancescan", subD.namespace, "master-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker & master scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Check pvc name and storage size for worker-scan.. !!!\n")
			subD.getPVCName(oc, "worker-scan")
			subD.getPVCSize(oc, "2Gi")

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, "master-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check master-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check pvc name and storage size for master-scan ..!!!\n")
			subD.getPVCName(oc, "master-scan")
			subD.getPVCSize(oc, "3Gi")

			g.By("The ocp-33230 complianceScan has performed successfully and storage size verified ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33609-Verify the tolerations could work for compliancesuite [Serial]", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "value1",
					operator:     "Equal",
					nodeSelector: "wscan",
					template:     csuitetaintTemplate,
				}
				csuite = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "",
					operator:     "Exists",
					nodeSelector: "wscan",
					template:     csuitetaintTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Label and set taint to one worker node.. !!!\n")
			//	setTaintLabelToWorkerNode(oc)
			//	setTaintToWorkerNodeWithValue(oc)
			nodeName := getOneWorkerNodeName(oc)
			defer func() {
				output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints}").Output()
				if strings.Contains(output, "value1") {
					taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")
				}
				output1, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints[0].key}").Output()
				if strings.Contains(output1, "key1") {
					taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
				}
				output2, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.metadata.labels.taint}").Output()
				if strings.Contains(output2, "true") {
					labelTaintNode(oc, "node", nodeName, "taint-")
				}
			}()
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule")
			labelTaintNode(oc, "node", nodeName, "taint=true")

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite.. !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove csuite and taint label from worker node.. !!!\n")
			csuiteD.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")

			g.By("Taint worker node without value.. !!!\n")
			/*	defer func() {
				taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			}()*/
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule")

			csuite.namespace = subD.namespace
			g.By("Create compliancesuite.. !!!\n")
			csuite.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuite.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuite.name, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove csuite, taint label and key from worker node.. !!!\n")
			csuite.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			labelTaintNode(oc, "node", nodeName, "taint-")
			//	removeTaintKeyFromWorkerNode(oc)
			//	removeTaintLabelFromWorkerNode(oc)

			g.By("ocp-33609 The compliance scan performed on tained node successfully.. !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33610-Verify the tolerations could work for compliancescan [Serial]", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "value1",
					operator:     "Equal",
					nodeSelector: "wscan",
					template:     cscantaintTemplate,
				}
				cscan = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "",
					operator:     "Exists",
					nodeSelector: "wscan",
					template:     cscantaintTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Label and set taint value to one worker node.. !!!\n")
			nodeName := getOneWorkerNodeName(oc)
			defer func() {
				output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints}").Output()
				if strings.Contains(output, "value1") {
					taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")
				}
				output1, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints[0].key}").Output()
				if strings.Contains(output1, "key1") {
					taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
				}
				output2, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.metadata.labels.taint}").Output()
				if strings.Contains(output2, "true") {
					labelTaintNode(oc, "node", nodeName, "taint-")
				}
			}()
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule")
			labelTaintNode(oc, "node", nodeName, "taint=true")

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan.. !!!\n")
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceScan name and result.. !!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if scan pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove compliancescan object and recover tainted worker node.. !!!\n")
			cscanD.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")

			g.By("Set taint to worker node without value.. !!!\n")
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule")

			cscan.namespace = subD.namespace
			g.By("Create compliancescan.. !!!\n")
			cscan.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscan.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceScan name and result.. !!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if the scan pod generated for tainted node...!!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove compliancescan object and taint label and key from worker node.. !!!\n")
			cscan.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			labelTaintNode(oc, "node", nodeName, "taint-")

			g.By("ocp-33610 The compliance scan performed on tained node successfully.. !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-28949-The complianceSuite and ComplianeScan perform scan using Platform scan type", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "platform-compliancesuite",
					namespace:    "",
					scanType:     "platform",
					scanname:     "platform-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuiteTemplate,
				}
				cscanMD = complianceScanDescription{
					name:         "platform-new-scan",
					namespace:    "",
					scanType:     "platform",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     cscanTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "platform-compliancesuite"},
				objectTableRef{"compliancescan", subD.namespace, "platform-new-scan"})

			csuiteD.namespace = subD.namespace
			g.By("Create platform-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create platform-new-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-scan-api-checks-pod")

			g.By("Check platform-new-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-new-scan-api-checks-pod")

			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "platform-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check platform-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check platform-new-scan name and result..!!!\n")
			subD.complianceScanName(oc, "platform-new-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check platform-new-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-28949 complianceScan for platform has performed successfully ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-36988-The ComplianceScan could be triggered for cis profile for platform scanType", func() {

			var (
				cscanMD = complianceScanDescription{
					name:         "platform-new-scan",
					namespace:    "",
					scanType:     "platform",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     cscanTemplate,
				}
			)

			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, "platform-new-scan"})

			cscanMD.namespace = subD.namespace
			g.By("Create platform-new-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-new-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-new-scan-api-checks-pod")

			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-new-scan name and result..!!!\n")
			subD.complianceScanName(oc, "platform-new-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check platform-new-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-36988 complianceScan for platform has performed successfully ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-36990-The ComplianceSuite could be triggered for cis profiles for platform scanType", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "platform-compliancesuite",
					namespace:    "",
					scanType:     "platform",
					scanname:     "platform-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "platform-compliancesuite"},
			)

			csuiteD.namespace = subD.namespace
			g.By("Create platform-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-scan-api-checks-pod")
			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "platform-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check platform-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By(" ocp-36990 The complianceSuite object successfully performed platform scan for cis profile ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-37063-The ComplianceSuite could be triggered for cis profiles for node scanType", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis-node",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}

				csuiteMD = complianceSuiteDescription{
					name:         "master-compliancesuite",
					namespace:    "",
					scanname:     "master-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis-node",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "master",
					template:     csuiteTemplate,
				}

				csuiteRD = complianceSuiteDescription{
					name:         "rhcos-compliancesuite",
					namespace:    "",
					scanname:     "rhcos-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis-node",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuitenodeTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "rhcos-compliancesuite"},
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check worker-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-etcd-unique-ca", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=NOT-APPLICABLE", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "NOT-APPLICABLE", ok, []string{"compliancecheckresult",
				"worker-scan-etcd-unique-ca", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Remove worker-compliancesuite object.. !!!\n")
			csuiteD.delete(itName, dr)

			csuiteMD.namespace = subD.namespace
			g.By("Create master-compliancesuite !!!\n")
			csuiteMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check master-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "master-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteMD.name, "NON-COMPLIANT")

			g.By("Check master-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"master-scan-etcd-unique-ca", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Remove master-compliancesuite object.. !!!\n")
			csuiteMD.delete(itName, dr)

			csuiteRD.namespace = subD.namespace
			g.By("Create rhcos-compliancesuite !!!\n")
			csuiteRD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteRD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check master-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "rhcos-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteRD.name, "INCONSISTENT")

			g.By("Check master-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-etcd-unique-ca", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=INCONSISTENT", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "INCONSISTENT", ok, []string{"compliancecheckresult",
				"rhcos-scan-etcd-unique-ca", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By(" ocp-37063 The complianceSuite object successfully triggered scan for cis node profile.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-32120-The ComplianceSuite performs schedule scan for Platform scan type", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "platform-compliancesuite",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanType:     "platform",
					scanname:     "platform-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "platform-compliancesuite"},
			)

			csuiteD.namespace = subD.namespace
			g.By("Create platform-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-scan-api-checks-pod")

			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "platform-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check platform-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			newCheck("expect", asAdmin, withoutNamespace, contain, "platform-compliancesuite-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", "platform-compliancesuite-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l=workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "platform-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-32120 The complianceScan object performed Platform schedule scan successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33418-The ComplianceSuite performs the schedule scan through cron job", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-compliancesuite-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", "worker-compliancesuite-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-33418 The ComplianceSuite object performed schedule scan successfully.. !!!\n")
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-33456-The Compliance-Operator edits the scheduled cron job to scan from ComplianceSuite", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "example-compliancesuite1",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, csuiteD.name},
			)

			// adding label to rhcos worker node to skip non-rhcos worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, csuiteD.name)
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")
			newCheck("expect", asAdmin, withoutNamespace, contain, csuiteD.name+"-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", csuiteD.name + "-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			g.By("edit schedule by patch.. !!!\n")
			patch := fmt.Sprintf("{\"spec\":{\"schedule\":\"*/4 * * * *\"}}")
			patchResource(oc, asAdmin, withoutNamespace, "compliancesuites", csuiteD.name, "-n", csuiteD.namespace, "--type", "merge", "-p", patch)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/4 * * * *", ok, []string{"cronjob", csuiteD.name + "-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, csuiteD.name)
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-33456-The Compliance-Operator could edit scheduled cron job to scan from ComplianceSuite successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33453-The Compliance Operator rotates the raw scan results", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					rotation:     2,
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"pod", subD.namespace, "pv-extract"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")
			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			//Verifying rotation policy and cronjob
			newCheck("expect", asAdmin, withoutNamespace, contain, "2", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.spec.scans[0].rawResultStorage.rotation}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-compliancesuite-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", "worker-compliancesuite-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			//Second round of scan and check
			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")
			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			//Third round of scan and check
			newCheck("expect", asAdmin, withoutNamespace, contain, "2", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[1].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")
			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Create pv-extract pod and verify arfReport result directories.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", pvextractpodYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pod", "pv-extract", "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			commands := []string{"exec", "pod/pv-extract", "--", "ls", "/workers-scan-results"}
			arfReportDir, err := oc.AsAdmin().Run(commands...).Args().Output()
			e2e.Logf("The arfReport result dir:\n%v", arfReportDir)
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(arfReportDir, "0") && (strings.Contains(arfReportDir, "1") && strings.Contains(arfReportDir, "2")) {
				g.By("The ocp-33453 The ComplianceSuite object performed schedule scan and rotates the raw scan results successfully.. !!!\n")
			}
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33660-Verify the differences in nodes from the same role could be handled [Serial]", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_direct_root_logins",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			var pod = podModifyD
			pod.namespace = oc.Namespace()
			nodeName := getOneRhcosWorkerNodeName(oc)
			pod.name = "pod-modify"
			pod.nodeName = nodeName
			pod.args = "touch /hostroot/etc/securetty"
			defer func() {
				pod.name = "pod-recover"
				pod.nodeName = nodeName
				pod.args = "rm -rf /hostroot/etc/securetty"
				pod.doActionsOnNode(oc, "Succeeded", dr)
			}()
			pod.doActionsOnNode(oc, "Succeeded", dr)

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "INCONSISTENT")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-direct-root-logins", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/inconsistent-check", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "INCONSISTENT", ok, []string{"compliancecheckresult",
				"worker-scan-no-direct-root-logins", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-33660 The compliance scan successfully handled the differences from the same role nodes ...!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-32814-The compliance operator by default creates ProfileBundles", func() {
			g.By("Check default profilebundles name and status.. !!!\n")
			subD.getProfileBundleNameandStatus(oc, "ocp4")
			subD.getProfileBundleNameandStatus(oc, "rhcos4")

			g.By("Check default profiles name.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			subD.getProfileName(oc, "ocp4-cis-node")
			subD.getProfileName(oc, "ocp4-e8")
			subD.getProfileName(oc, "ocp4-moderate")
			subD.getProfileName(oc, "rhcos4-e8")
			subD.getProfileName(oc, "rhcos4-moderate")

			g.By("ocp-32814 The Compliance Operator by default created ProfileBundles and profiles are verified successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33431-Verify compliance check result shows in ComplianceCheckResult label for compliancesuite", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})
			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-severity=medium", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=PASS", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/scan-name=worker-scan", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/suite=worker-compliancesuite", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("ocp-33431 The compliance scan result verified through ComplianceCheckResult label successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33435-Verify the compliance scan result shows in ComplianceCheckResult label for compliancescan", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "rhcos-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "wscan",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, "rhcos-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan !!!\n")
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceScanName(oc, "rhcos-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-severity=high", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=FAIL", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/scan-name=rhcos-scan", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("ocp-33435 The compliance scan result verified through ComplianceCheckResult label successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33449-The compliance-operator raw results store in ARF format on a PVC", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"pod", subD.namespace, "pv-extract"})

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Create pv-extract pod and check status.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", pvextractpodYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pod", "pv-extract", "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check ARF report generates in xml format.. !!!\n")
			subD.getARFreportFromPVC(oc, ".xml.bzip2")

			g.By("The ocp-33449 complianceScan raw result successfully stored in ARF format on the PVC... !!!!\n")

		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-37171-Check compliancesuite status when there are multiple rhcos4 profiles added in scansettingbinding object", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "rhcos4",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "rhcos4-e8",
					profilename2:    "rhcos4-moderate",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			g.By("Check default profiles name rhcos4-e8 .. !!!\n")
			subD.getProfileName(oc, ssb.profilename1)
			g.By("Check default profiles name rhcos4-moderate .. !!!\n")
			subD.getProfileName(oc, ssb.profilename2)

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker and master scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-37171 Check compliancesuite status when there are multiple rhcos4 profiles added in scansettingbinding object successfully... !!!\n")
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-High-37084-The ComplianceSuite generates through ScanSettingBinding CR with tailored cis profile", func() {
			var (
				tp = tailoredProfileWithoutVarDescription{
					name:         "ocp4-cis-custom",
					namespace:    "",
					extends:      "ocp4-cis",
					enrulename1:  "ocp4-scc-limit-root-containers",
					enrulename2:  "ocp4-scheduler-no-bind-address",
					disrulename1: "ocp4-api-server-encryption-provider-cipher",
					disrulename2: "ocp4-scc-drop-container-capabilities",
					template:     tprofileWithoutVarTemplate,
				}
				ss = scanSettingDescription{
					autoapplyremediations: true,
					name:                  "myss",
					namespace:             "",
					roles1:                "master",
					roles2:                "worker",
					rotation:              5,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "my-companys-compliance-requirements",
					namespace:       "",
					profilekind1:    "TailoredProfile",
					profilename1:    "ocp4-cis-custom",
					profilename2:    "ocp4-cis-node",
					scansettingname: "myss",
					template:        scansettingbindingTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name},
				objectTableRef{"tailoredprofile", subD.namespace, tp.name})

			g.By("Check default profiles name ocp4-cis and ocp4-cis-node.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			subD.getProfileName(oc, "ocp4-cis-node")

			tp.namespace = subD.namespace
			g.By("Create tailoredprofile !!!\n")
			tp.create(oc, itName, dr)
			subD.getTailoredProfileNameandStatus(oc, tp.name)

			g.By("Create scansetting !!!\n")
			ss.namespace = subD.namespace
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "myss", ok, []string{"scansetting", ss.name, "-n", ss.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "my-companys-compliance-requirements", ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker and master scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-37084 The ComplianceSuite generates through ScanSettingBinding CR with tailored cis profile successfully... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-34928-Storage class and access modes are configurable through ComplianceSuite and ComplianceScan", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:             "worker-compliancesuite",
					namespace:        "",
					scanname:         "worker-scan",
					profile:          "xccdf_org.ssgproject.content_profile_moderate",
					content:          "ssg-rhcos4-ds.xml",
					contentImage:     "quay.io/complianceascode/ocp4:latest",
					rule:             "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector:     "worker",
					storageClassName: "gold",
					pvAccessModes:    "ReadWriteOnce",
					template:         csuiteSCTemplate,
				}
				cscanMD = complianceScanDescription{
					name:             "master-scan",
					namespace:        "",
					profile:          "xccdf_org.ssgproject.content_profile_e8",
					content:          "ssg-rhcos4-ds.xml",
					contentImage:     "quay.io/complianceascode/ocp4:latest",
					rule:             "xccdf_org.ssgproject.content_rule_accounts_no_uid_except_zero",
					nodeSelector:     "master",
					storageClassName: "gold",
					pvAccessModes:    "ReadWriteOnce",
					template:         cscanSCTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"compliancescan", subD.namespace, "master-scan"},
				objectTableRef{"storageclass", subD.namespace, "gold"})

			g.By("Get the default storageClass provisioner & volumeBindingMode from cluster .. !!!\n")
			storageClass.name = "gold"
			storageClass.provisioner = getStorageClassProvisioner(oc)
			storageClass.reclaimPolicy = "Delete"
			storageClass.volumeBindingMode = getStorageClassVolumeBindingMode(oc)
			storageClass.create(oc, itName, dr)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker & master scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT INCONSISTENT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Check pvc name and storage size for worker-scan.. !!!\n")
			subD.getPVCName(oc, "worker-scan")
			newCheck("expect", asAdmin, withoutNamespace, contain, "gold", ok, []string{"pvc", csuiteD.scanname, "-n",
				subD.namespace, "-o=jsonpath={.spec.storageClassName}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ReadWriteOnce", ok, []string{"pvc", csuiteD.scanname, "-n",
				subD.namespace, "-o=jsonpath={.status.accessModes[]}"}).check(oc)

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, "master-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check master-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Check pvc name and storage size for master-scan ..!!!\n")
			subD.getPVCName(oc, "master-scan")
			newCheck("expect", asAdmin, withoutNamespace, contain, "gold", ok, []string{"pvc", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.spec.storageClassName}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ReadWriteOnce", ok, []string{"pvc", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.accessModes[]}"}).check(oc)

			g.By("ocp-34928 Storage class and access modes are successfully configurable through ComplianceSuite and ComplianceScan ..!!!\n")
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-40372-Use a separate SA for resultserver", func() {
			var csuiteMD = complianceSuiteDescription{
				name:         "master-compliancesuite",
				namespace:    "",
				scanname:     "master-scan",
				profile:      "xccdf_org.ssgproject.content_profile_moderate",
				content:      "ssg-rhcos4-ds.xml",
				contentImage: "quay.io/complianceascode/ocp4:latest",
				rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
				nodeSelector: "master",
				template:     csuiteTemplate,
			}

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "master-compliancesuite"})

			g.By("check role resultserver")
			rsRoleName := getResourceNameWithKeyword(oc, "role", "resultserver")
			e2e.Logf("rs role name: %v\n", rsRoleName)
			newCheck("expect", asAdmin, withoutNamespace, contain, "resourceNames:[restricted] resources:[securitycontextconstraints] verbs:[use]]", ok, []string{"role", rsRoleName, "-n",
				subD.namespace, "-o=jsonpath={.rules}"}).check(oc)

			g.By("create compliancesuite")
			csuiteMD.namespace = subD.namespace
			csuiteMD.create(oc, itName, dr)

			g.By("check the scc and securityContext for the rs pod")
			rsPodName := getResourceNameWithKeywordFromResourceList(oc, "pod", "rs")
			//could not use newCheck as rs pod will be deleted soon
			checkKeyWordsForRspod(oc, rsPodName, [...]string{"restricted", "fsGroup", "resultserver"})

		})

	})

})
