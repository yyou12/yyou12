package securityandcompliance

import (
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
		oc                     = exutil.NewCLI("compliance-"+getRandomString(), exutil.KubeConfigPath())
		buildPruningBaseDir    = exutil.FixturePath("testdata", "securityandcompliance")
		ogCoTemplate           = filepath.Join(buildPruningBaseDir, "operator-group.yaml")
		catsrcCoTemplate       = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subCoTemplate          = filepath.Join(buildPruningBaseDir, "subscription.yaml")
		csuiteTemplate         = filepath.Join(buildPruningBaseDir, "compliancesuite.yaml")
		csuitetpcmTemplate     = filepath.Join(buildPruningBaseDir, "compliancesuitetpconfmap.yaml")
		csuitetaintTemplate    = filepath.Join(buildPruningBaseDir, "compliancesuitetaint.yaml")
		cscanTemplate          = filepath.Join(buildPruningBaseDir, "compliancescan.yaml")
		cscantaintTemplate     = filepath.Join(buildPruningBaseDir, "compliancescantaint.yaml")
		tprofileTemplate       = filepath.Join(buildPruningBaseDir, "tailoredprofile.yaml")
		scansettingYAML        = filepath.Join(buildPruningBaseDir, "scansetting.yaml")
		scansettingbindingYAML = filepath.Join(buildPruningBaseDir, "scansettingbinding.yaml")
		pvextractpodYAML       = filepath.Join(buildPruningBaseDir, "pv-extract-pod.yaml")
		podModifyTemplate      = filepath.Join(buildPruningBaseDir, "pod_modify.yaml")
		dr                     = make(describerResrouce)

		catSrc = catalogSourceDescription{
			name:        "compliance-operator",
			namespace:   "",
			displayName: "openshift-compliance-operator",
			publisher:   "Red Hat",
			sourceType:  "grpc",
			address:     "quay.io/openshift-qe-optional-operators/compliance-operator-index:latest",
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
			channel:                "4.6",
			ipApproval:             "Automatic",
			operatorPackage:        "compliance-operator",
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
	)

	g.BeforeEach(func() {

		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.getIr(itName).cleanup()
		dr.rmIr(itName)
	})

	// author: pdhamdhe@redhat.com
	g.It("Critical-34378-Install the Compliance Operator through OLM using CatalogSource and Subscription", func() {

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

	g.Context("When the compliance-operator is installed through OLM", func() {

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
		g.It("Critical-27649-The ComplianceSuite reports the scan result as Compliant or Non-Compliant", func() {

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
			subD.complianceSuiteResult(oc, "NON-COMPLIANT")

			g.By("Check master-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check worker-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-27649 ComplianceScan has performed successfully... !!!!\n ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Medium-32082-The ComplianceSuite shows the scan result NOT-APPLICABLE after all rules are skipped to scan", func() {

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
			subD.complianceSuiteResult(oc, "NOT-APPLICABLE")

			g.By("Check rule status through complianceCheckResult.. !!!\n")
			subD.getRuleStatus(oc, "SKIP")

			g.By("The ocp-32082 complianceScan has performed successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("High-33398-The Compliance Operator supports to variables in tailored profile", func() {

			var (
				tprofileD = tailoredProfileDescription{
					name:         "rhcos-tailoredprofile",
					namespace:    "",
					extends:      "rhcos4-ncp",
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
		g.It("High-32840-The ComplianceSuite generates through ScanSetting CR", func() {

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
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "co-requirement"},
				objectTableRef{"scansetting", subD.namespace, "co-setting"},
				objectTableRef{"tailoredprofile", subD.namespace, "rhcos-tp"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			g.By("Check default profiles name rhcos4-e8 .. !!!\n")
			subD.getProfileName(oc, "rhcos4-e8")

			tprofileD.namespace = subD.namespace
			g.By("Create tailoredprofile rhcos-tp !!!\n")
			tprofileD.create(oc, itName, dr)

			g.By("Verify tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, "rhcos-tp")

			g.By("Create scansetting !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", scansettingYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "co-setting", ok, []string{"scansetting", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			_, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", scansettingbindingYAML).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "co-requirement", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", "co-requirement", "-n", subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker and master scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "co-requirement")
			subD.complianceSuiteResult(oc, "NON-COMPLIANT")

			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify the disable rules are not available in compliancecheckresult.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-audit-rules-dac-modification-chown", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)

			g.By("ocp-32840 The ComplianceSuite generated successfully using scansetting... !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Medium-33381-Verify the ComplianceSuite could be generated from Tailored profiles", func() {

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
			subD.complianceSuiteResult(oc, "NON-COMPLIANT")

			g.By("Check rhcos-csuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify the enable and disable rules.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-sshd-disable-root-login", ok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-audit-rules-dac-modification-chown", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)

			g.By("ocp-33381 The ComplianceSuite performed scan successfully using tailored profile... !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Medium-33713-The ComplianceSuite reports the scan result as Error", func() {

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
			subD.complianceSuiteResult(oc, "ERROR")

			g.By("Check complianceScan result through configmap exit-code and result from error-msg..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "1")
			subD.getScanResultFromConfigmap(oc, "No profile matching suffix \"xccdf_org.ssgproject.content_profile_coreos-ncp\" was found.")

			g.By("The ocp-33713 complianceScan has performed successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Critical-27705-The ComplianceScan reports the scan result Compliant or Non-Compliant", func() {

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
		g.It("Medium-27762-The ComplianceScan reports the scan result Error", func() {

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
		g.It("Medium-27968-Perform scan only on a subset of nodes using ComplianceScan object", func() {

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
		g.It("High-33230-The compliance-operator raw result storage size is configurable", func() {

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
			subD.complianceSuiteResult(oc, "COMPLIANT")

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
		g.It("High-33609-Verify the tolerations could work for compliancesuite [Serial]", func() {

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
			subD.complianceSuiteResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if pod generated for tainted node.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, nodeName, ok, []string{"pods", "-n", subD.namespace, "--selector=workload=scanner",
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

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
			subD.complianceSuiteResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if the pod generated for tainted node...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, nodeName, ok, []string{"pods", "-n", subD.namespace, "--selector=workload=scanner",
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Remove csuite, taint label and key from worker node.. !!!\n")
			csuite.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			labelTaintNode(oc, "node", nodeName, "taint-")
			//	removeTaintKeyFromWorkerNode(oc)
			//	removeTaintLabelFromWorkerNode(oc)

			g.By("ocp-33609 The compliance scan performed on tained node successfully.. !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("High-33610-Verify the tolerations could work for compliancescan [Serial]", func() {

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
			newCheck("expect", asAdmin, withoutNamespace, contain, nodeName, ok, []string{"pods", "-n", subD.namespace, "--selector=workload=scanner",
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

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
			newCheck("expect", asAdmin, withoutNamespace, contain, nodeName, ok, []string{"pods", "-n", subD.namespace, "--selector=workload=scanner",
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Remove compliancescan object and taint label and key from worker node.. !!!\n")
			cscan.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			labelTaintNode(oc, "node", nodeName, "taint-")

			g.By("ocp-33610 The compliance scan performed on tained node successfully.. !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Critical-28949-The complianceSuite and ComplianeScan perform scan using Platform scan type", func() {

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
			subD.complianceSuiteResult(oc, "NON-COMPLIANT")

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
		g.It("High-32120-The ComplianceSuite performs schedule scan for Platform scan type", func() {

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
			subD.complianceSuiteResult(oc, "NON-COMPLIANT")

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
			subD.complianceSuiteResult(oc, "NON-COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-32120 The complianceScan object performed Platform schedule scan successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("High-33418-The ComplianceSuite performs the schedule scan through cron job", func() {

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
			subD.complianceSuiteResult(oc, "COMPLIANT")

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
			subD.complianceSuiteResult(oc, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-33418 The ComplianceSuite object performed schedule scan successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("High-33453-The Compliance Operator rotates the raw scan results", func() {

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
			subD.complianceSuiteResult(oc, "COMPLIANT")
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
			subD.complianceSuiteResult(oc, "COMPLIANT")
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
			subD.complianceSuiteResult(oc, "COMPLIANT")
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
		g.It("High-33660-Verify the differences in nodes from the same role could be handled [Serial]", func() {

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
			subD.complianceSuiteResult(oc, "INCONSISTENT")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-direct-root-logins", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/inconsistent-check", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "INCONSISTENT", ok, []string{"compliancecheckresult",
				"worker-scan-no-direct-root-logins", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-33660 The compliance scan successfully handled the differences from the same role nodes ...!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Medium-32814-The compliance operator by default creates ProfileBundles", func() {
			g.By("Check default profilebundles name and status.. !!!\n")
			subD.getProfileBundleNameandStatus(oc, "ocp4")
			subD.getProfileBundleNameandStatus(oc, "rhcos4")

			g.By("Check default profiles name.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			subD.getProfileName(oc, "ocp4-cis-node")
			subD.getProfileName(oc, "ocp4-e8")
			subD.getProfileName(oc, "ocp4-moderate")
			subD.getProfileName(oc, "ocp4-ncp")
			subD.getProfileName(oc, "rhcos4-e8")
			subD.getProfileName(oc, "rhcos4-moderate")
			subD.getProfileName(oc, "rhcos4-ncp")

			g.By("ocp-32814 The Compliance Operator by default created ProfileBundles and profiles are verified successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Medium-33431-Verify compliance check result shows in ComplianceCheckResult label for compliancesuite", func() {

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
			subD.complianceSuiteResult(oc, "COMPLIANT")

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
		g.It("Medium-33435-Verify the compliance scan result shows in ComplianceCheckResult label for compliancescan", func() {

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
		g.It("Medium-33449-The compliance-operator raw results store in ARF format on a PVC", func() {

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
			subD.complianceSuiteResult(oc, "NON-COMPLIANT")

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
	})

})
