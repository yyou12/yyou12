package securityandcompliance

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"path/filepath"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-isc] Security_and_Compliance The Compliance Operator automates compliance check for OpenShift and CoreOS", func() {
	defer g.GinkgoRecover()

	var (
		oc                  = exutil.NewCLI("compliance-"+getRandomString(), exutil.KubeConfigPath())
		buildPruningBaseDir = exutil.FixturePath("testdata", "securityandcompliance")
		ogCoTemplate        = filepath.Join(buildPruningBaseDir, "operator-group.yaml")
		catsrcCoTemplate    = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subCoTemplate       = filepath.Join(buildPruningBaseDir, "subscription.yaml")
		csuiteTemplate      = filepath.Join(buildPruningBaseDir, "compliancesuite.yaml")
		cscanTemplate       = filepath.Join(buildPruningBaseDir, "compliancescan.yaml")
		dr                  = make(describerResrouce)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
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
			itName = g.CurrentGinkgoTestDescription().TestText
		)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				{"compliancesuite", subD.namespace, "master-compliancesuite"},
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed !!! ")

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

		g.By("Check worker scan pods !!!\n")
		subD.scanPodName(oc, "worker-scan")

		g.By("Check master scan pods !!!\n")
		subD.scanPodName(oc, "master-scan")

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
			itName = g.CurrentGinkgoTestDescription().TestText
		)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed !!! ")

		// adding label to rhcos worker node to skip rhel worker node if any
		g.By("Label all rhcos worker nodes as wscan !!!\n")
		setLabelToNode(oc)

		csuiteD.namespace = subD.namespace
		g.By("Create compliancesuite !!!\n")
		e2e.Logf("Here namespace : %v\n", catSrc.namespace)
		csuiteD.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
			subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check worker scan pods !!!\n")
		subD.scanPodName(oc, "worker-scan")

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

			cscanMD = complianceSuiteDescription{
				name:         "master-scan",
				namespace:    "",
				profile:      "xccdf_org.ssgproject.content_profile_moderate",
				content:      "ssg-rhcos4-ds.xml",
				contentImage: "quay.io/complianceascode/ocp4:latest",
				rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
				nodeSelector: "master",
				template:     cscanTemplate,
			}
			itName = g.CurrentGinkgoTestDescription().TestText
		)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"compliancescan", subD.namespace, "worker-scan"},
				{"compliancescan", subD.namespace, "master-scan"},
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed !!! ")

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

		g.By("Check worker scan pods !!!\n")
		subD.scanPodName(oc, "worker-scan")

		g.By("Check master scan pods !!!\n")
		subD.scanPodName(oc, "master-scan")

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
			itName = g.CurrentGinkgoTestDescription().TestText
		)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"compliancescan", subD.namespace, "worker-scan"},
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed !!! ")

		// adding label to rhcos worker node to skip rhel worker node if any
		g.By("Label all rhcos worker nodes as wscan !!!\n")
		setLabelToNode(oc)

		cscanD.namespace = subD.namespace
		g.By("Create compliancescan !!!\n")
		e2e.Logf("Here namespace : %v\n", catSrc.namespace)
		cscanD.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
			subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check worker scan pods !!!\n")
		subD.scanPodName(oc, "worker-scan")

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
			cscanMD = complianceSuiteDescription{
				name:         "master-scan",
				namespace:    "",
				profile:      "xccdf_org.ssgproject.content_profile_moderate",
				content:      "ssg-rhcos4-ds.xml",
				contentImage: "quay.io/complianceascode/ocp4:latest",
				rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
				nodeSelector: "master",
				template:     cscanTemplate,
			}
			itName = g.CurrentGinkgoTestDescription().TestText
		)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"compliancescan", subD.namespace, "master-scan"},
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed !!! ")

		cscanMD.namespace = subD.namespace
		g.By("Create master-scan !!!\n")
		cscanMD.create(oc, itName, dr)

		newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
			subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check master scan pods !!!\n")
		subD.scanPodName(oc, "master-scan")

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

			itName = g.CurrentGinkgoTestDescription().TestText
		)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				{"compliancescan", subD.namespace, "master-scan"},
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully.. !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created.. !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state.. !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed.. !!! ")

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

		g.By("Check worker scan pods.. !!!\n")
		subD.scanPodName(oc, "worker-scan")

		g.By("Check master scan pods.. !!!\n")
		subD.scanPodName(oc, "master-scan")

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

			itName = g.CurrentGinkgoTestDescription().TestText
		)

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

		defer func() {
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			removeCmds := []struct {
				kind      string
				namespace string
				name      string
			}{
				{"compliancesuite", subD.namespace, "platform-compliancesuite"},
				{"compliancescan", subD.namespace, "platform-new-scan"},
				{"profilebundle.compliance", subD.namespace, "ocp4"},
				{"profilebundle.compliance", subD.namespace, "rhcos4"},
				{"deployment", subD.namespace, "compliance-operator"},
			}
			for _, v := range removeCmds {
				e2e.Logf("Start to remove: %v", v)
				_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Check CSV is created sucessfully.. !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are created.. !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
			"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-pp", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-pp", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

		g.By("Check Compliance Operator & profileParser pods are in running state.. !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
			subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

		g.By("Compliance Operator sucessfully installed.. !!! ")

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

})
