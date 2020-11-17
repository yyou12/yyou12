package securityandcompliance

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-isc] Security_and_Compliance an end user handle FIO within a namespace", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("fio-"+getRandomString(), exutil.KubeConfigPath())

		buildPruningBaseDir = exutil.FixturePath("testdata", "securityandcompliance")
		ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operator-group.yaml")
		catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subTemplate         = filepath.Join(buildPruningBaseDir, "subscription.yaml")
		fioTemplate         = filepath.Join(buildPruningBaseDir, "fileintegrity.yaml")
		podModifyTemplate   = filepath.Join(buildPruningBaseDir, "pod_modify.yaml")
		configFile          = filepath.Join(buildPruningBaseDir, "aide.conf.rhel8")
		configErrFile       = filepath.Join(buildPruningBaseDir, "aide.conf.rhel8.err")

		catsrc = catalogSourceDescription{
			name:        "file-integrity-operator",
			namespace:   "",
			displayName: "openshift-file-integrity-operator",
			publisher:   "Red Hat",
			sourceType:  "grpc",
			address:     "quay.io/openshift-qe-optional-operators/file-integrity-operator-index:latest",
			template:    catsrcImageTemplate,
		}
		og = operatorGroupDescription{
			name:      "openshift-file-integrity-qbcd",
			namespace: "",
			template:  ogSingleTemplate,
		}
		sub = subscriptionDescription{
			subName:                "file-integrity-operator",
			namespace:              "",
			channel:                "4.6",
			ipApproval:             "Automatic",
			operatorPackage:        "file-integrity-operator",
			catalogSourceName:      "file-integrity-operator",
			catalogSourceNamespace: "",
			startingCSV:            "",
			currentCSV:             "",
			installedCSV:           "",
			template:               subTemplate,
			singleNamespace:        true,
		}
		fi1 = fileintegrity{
			name:              "example-fileintegrity",
			namespace:         "",
			configname:        "",
			configkey:         "",
			graceperiod:       15,
			debug:             false,
			nodeselectorkey:   "kubernetes.io/os",
			nodeselectorvalue: "linux",
			template:          fioTemplate,
		}
		podModifyD = podModify{
			name:      "",
			namespace: "",
			nodeName:  "",
			args:      "",
			template:  podModifyTemplate,
		}
		dr = make(describerResrouce)
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

	// It will cover test case: OCP-34388 & OCP-27760 , author: xiyuan@redhat.com
	g.It("Critical-34388-High-27760-check file-integrity-operator could report failure and persist the failure logs on to a ConfigMap [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		sub.checkPodFioStatus(oc, "running")
		g.By("Create fileintegrity")
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")

		var pod = podModifyD
		pod.namespace = oc.Namespace()
		nodeName := getOneWorkerNodeName(oc)
		pod.name = "pod-modify"
		pod.nodeName = nodeName
		pod.args = "mkdir -p /hostroot/root/test"
		defer func() {
			pod.name = "pod-recover"
			pod.nodeName = nodeName
			pod.args = "rm -rf /hostroot/root/test"
			pod.doActionsOnNode(oc, "Succeeded", dr)
		}()
		pod.doActionsOnNode(oc, "Succeeded", dr)
		time.Sleep(time.Second * 30)
		cmName := fi1.getConfigmapFromFileintegritynodestatus(oc, nodeName)
		fi1.getDataFromConfigmap(oc, cmName, "/hostroot/root/test")
	})

	//author: xiyuan@redhat.com
	g.It("Medium-31979-the enabling debug flag of the logcollector should work [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity with debug=false")
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		var podName = fi1.getOneFioPodName(oc)
		fi1.checkArgsInPod(oc, "debug=false")
		fi1.checkKeywordNotExistInLog(oc, podName, "debug:")

		g.By("Configure fileintegrity with debug=true")
		fi1.debug = true
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		podName = fi1.getOneFioPodName(oc)
		fi1.checkArgsInPod(oc, "debug=true")
		fi1.checkKeywordExistInLog(oc, podName, "debug:")

	})

	//author: xiyuan@redhat.com
	g.It("Medium-31933-the disabling debug flag of the logcollector should work [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = true

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity with debug=true")
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		var podName = fi1.getOneFioPodName(oc)
		fi1.checkArgsInPod(oc, "debug=true")
		fi1.checkKeywordExistInLog(oc, podName, "debug:")

		g.By("Configure fileintegrity with debug=false")
		fi1.debug = false
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		podName = fi1.getOneFioPodName(oc)
		fi1.checkArgsInPod(oc, "debug=false")
		fi1.checkKeywordNotExistInLog(oc, podName, "debug:")

	})

	//author: xiyuan@redhat.com
	g.It("Medium-31873-check the gracePeriod is configurable [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity without gracePeriod")
		fi1.createFIOWithoutKeyword(oc, itName, dr, "gracePeriod")
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkArgsInPod(oc, "interval=900")

		g.By("create fileintegrity with configmap and gracePeriod")
		fi1.configname = "myconf"
		fi1.configkey = "aide-conf"
		fi1.graceperiod = 0
		fi1.createConfigmapFromFile(oc, itName, dr, fi1.configname, fi1.configkey, configFile, "created")
		fi1.checkConfigmapCreated(oc)
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkArgsInPod(oc, "interval=10")

		fi1.graceperiod = 11
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkArgsInPod(oc, "interval=11")

		fi1.graceperiod = 120
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkArgsInPod(oc, "interval=120")

		fi1.graceperiod = -10
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkArgsInPod(oc, "interval=10")
	})

	//author: xiyuan@redhat.com
	g.It("Medium-28524-adding invalid configuration should report failure", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity")
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		nodeName := getOneWorkerNodeName(oc)
		time.Sleep(time.Second * 30)
		fi1.checkFileintegritynodestatus(oc, nodeName, "Succeeded")

		g.By("Check fileintegritynodestatus becomes Errored")
		fi1.configname = "errfile"
		fi1.configkey = "aideerrconf"
		fi1.createConfigmapFromFile(oc, itName, dr, fi1.configname, fi1.configkey, configErrFile, "created")
		fi1.checkConfigmapCreated(oc)
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		var podName = fi1.getOneFioPodName(oc)
		fi1.checkKeywordExistInLog(oc, podName, "exit status 17")
		time.Sleep(time.Second * 30)
		fi1.checkFileintegritynodestatus(oc, nodeName, "Errored")
	})

	//author: xiyuan@redhat.com
	g.It("Medium-33177-only one long-running daemonset should be created by FIO", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity without aide config")
		fi1.createFIOWithoutKeyword(oc, itName, dr, "gracePeriod")
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkOnlyOneDaemonset(oc)

		g.By("Create fileintegrity with aide config")
		fi1.configname = "myconf"
		fi1.configkey = "aide-conf"
		fi1.createConfigmapFromFile(oc, itName, dr, fi1.configname, fi1.configkey, configFile, "created")
		fi1.checkConfigmapCreated(oc)
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkOnlyOneDaemonset(oc)
	})

	//author: xiyuan@redhat.com
	g.It("Medium-33853-check whether aide will not reinit when a fileintegrity recreated after deleted [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity without aide config")
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		var pod = podModifyD
		pod.namespace = oc.Namespace()
		nodeName := getOneWorkerNodeName(oc)
		pod.name = "pod-modify"
		pod.nodeName = nodeName
		pod.args = "mkdir -p /hostroot/root/test"
		defer func() {
			pod.name = "pod-recover"
			pod.nodeName = nodeName
			pod.args = "rm -rf /hostroot/root/test"
			pod.doActionsOnNode(oc, "Succeeded", dr)
		}()
		pod.doActionsOnNode(oc, "Succeeded", dr)
		time.Sleep(time.Second * 30)
		fi1.checkFileintegritynodestatus(oc, nodeName, "Failed")
		cmName := fi1.getConfigmapFromFileintegritynodestatus(oc, nodeName)
		fi1.getDataFromConfigmap(oc, cmName, "/hostroot/root/test")

		g.By("delete and recreate the fileintegrity")
		fi1.removeFileintegrity(oc, "deleted")
		fi1.createFIOWithoutConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		time.Sleep(time.Second * 30)
		fi1.checkFileintegritynodestatus(oc, nodeName, "Failed")
		fi1.getDataFromConfigmap(oc, cmName, "/hostroot/root/test")

		g.By("trigger reinit")
		fi1.reinitFileintegrity(oc, "annotated")
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkFileintegritynodestatus(oc, nodeName, "Succeeded")
	})

	//author: xiyuan@redhat.com
	g.It("Medium-33332-The fileintegritynodestatuses should show status summary for FIO [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity with aide config")
		fi1.configname = "myconf"
		fi1.configkey = "aide-conf"
		fi1.createConfigmapFromFile(oc, itName, dr, fi1.configname, fi1.configkey, configFile, "created")
		fi1.checkConfigmapCreated(oc)
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		time.Sleep(time.Second * 60)

		g.By("Check Data Details in CM and Fileintegritynodestatus Equal or not")
		nodeName := getOneWorkerNodeName(oc)
		fi1.checkFileintegritynodestatus(oc, nodeName, "Failed")
		cmName := fi1.getConfigmapFromFileintegritynodestatus(oc, nodeName)
		intFileAddedCM, intFileChangedCM, intFileRemovedCM := fi1.getDetailedDataFromConfigmap(oc, cmName)
		intFileAddedFins, intFileChangedFins, intFileRemovedFins := fi1.getDetailedDataFromFileintegritynodestatus(oc, nodeName)
		checkDataDetailsEqual(intFileAddedCM, intFileChangedCM, intFileRemovedCM, intFileAddedFins, intFileChangedFins, intFileRemovedFins)
	})

	//author: xiyuan@redhat.com
	g.It("High-33226-enable configuring tolerations in FileIntegrities [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false
		fi1.nodeselectorkey = "node-role.kubernetes.io/worker"
		fi1.nodeselectorvalue = ""

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create taint")
		nodeName := getOneWorkerNodeName(oc)
		defer func() {
			output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints}").Output()
			if strings.Contains(output, "value1") {
				taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")
			}
		}()
		taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule")

		g.By("Create fileintegrity with aide config and compare Aide-scan pod number and Node number")
		fi1.configname = "myconf"
		fi1.configkey = "aide-conf"
		fi1.createConfigmapFromFile(oc, itName, dr, fi1.configname, fi1.configkey, configFile, "created")
		fi1.checkConfigmapCreated(oc)
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerLessThanNodeNumber(oc, "worker")

		g.By("patch the tolerations and compare again")
		patch := fmt.Sprintf("{\"spec\":{\"tolerations\":[{\"effect\":\"NoSchedule\",\"key\":\"key1\",\"operator\":\"Equal\",\"value\":\"value1\"}]}}")
		patchResource(oc, asAdmin, withoutNamespace, "fileintegrity", fi1.name, "-n", fi1.namespace, "--type", "merge", "-p", patch)
		fi1.recreateFileintegrity(oc)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerEqualNodeNumber(oc, "node-role.kubernetes.io/worker=")

		taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")
		defer taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
		taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule")

		g.By("Create fileintegrity with aide config and compare Aide-scan pod number and Node number")
		fi1.removeFileintegrity(oc, "deleted")
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerLessThanNodeNumber(oc, "worker")

		g.By("patch the tolerations and compare again")
		patch = fmt.Sprintf("{\"spec\":{\"tolerations\":[{\"effect\":\"NoSchedule\",\"key\":\"key1\",\"operator\":\"Exists\"}]}}")
		patchResource(oc, asAdmin, withoutNamespace, "fileintegrity", fi1.name, "-n", fi1.namespace, "--type", "merge", "-p", patch)
		fi1.recreateFileintegrity(oc)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerEqualNodeNumber(oc, "node-role.kubernetes.io/worker=")
	})

	//author: xiyuan@redhat.com
	g.It("Medium-33254-enable configuring tolerations in FileIntegrities when there is more than one taint on one node [Serial]", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false
		fi1.nodeselectorkey = "node-role.kubernetes.io/worker"
		fi1.nodeselectorvalue = ""

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create taint")
		nodeName := getOneWorkerNodeName(oc)
		defer taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-", "key2=value2:NoExecute-")
		taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule", "key2=value2:NoExecute")

		g.By("Create fileintegrity with aide config and compare Aide-scan pod number and Node number")
		fi1.configname = "myconf"
		fi1.configkey = "aide-conf"
		fi1.createConfigmapFromFile(oc, itName, dr, fi1.configname, fi1.configkey, configFile, "created")
		fi1.checkConfigmapCreated(oc)
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerLessThanNodeNumber(oc, "worker")

		g.By("patch the tolerations and compare again")
		patch := fmt.Sprintf("{\"spec\":{\"tolerations\":[{\"effect\":\"NoSchedule\",\"key\":\"key1\",\"operator\":\"Equal\",\"value\":\"value1\"},{\"effect\":\"NoExecute\",\"key\":\"key2\",\"operator\":\"Equal\",\"value\":\"value2\"}]}}")
		patchResource(oc, asAdmin, withoutNamespace, "fileintegrity", fi1.name, "-n", fi1.namespace, "--type", "merge", "-p", patch)
		fi1.recreateFileintegrity(oc)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerEqualNodeNumber(oc, "node-role.kubernetes.io/worker=")
	})

	//author: xiyuan@redhat.com
	g.It("Medium-27755-check nodeSelector works for operator file-integrity-operator", func() {
		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catsrc.namespace = oc.Namespace()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace
		fi1.namespace = oc.Namespace()
		fi1.debug = false
		fi1.nodeselectorkey = "node-role.kubernetes.io/worker"
		fi1.nodeselectorvalue = ""

		g.By("Create catsrc")
		catsrc.create(oc, itName, dr)
		catsrc.checkPackagemanifest(oc, catsrc.displayName)
		g.By("Create og")
		og.create(oc, itName, dr)
		og.checkOperatorgroup(oc, og.name)
		g.By("Create subscription")
		sub.create(oc, itName, dr)
		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		sub.checkPodFioStatus(oc, "running")

		g.By("Create fileintegrity with aide config and compare Aide-scan pod number and Node number")
		fi1.configname = "myconf"
		fi1.configkey = "aide-conf"
		fi1.createConfigmapFromFile(oc, itName, dr, fi1.configname, fi1.configkey, configFile, "created")
		fi1.checkConfigmapCreated(oc)
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerEqualNodeNumber(oc, "node-role.kubernetes.io/worker=")

		g.By("Rereate fileintegrity with a new nodeselector and compare Aide-scan pod number and Node number")
		fi1.removeFileintegrity(oc, "deleted")
		fi1.nodeselectorkey = "node-role.kubernetes.io/worker"
		fi1.nodeselectorvalue = ""
		g.By("Create fileintegrity with aide config and compare Aide-scan pod number and Node number")
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerEqualNodeNumber(oc, "node-role.kubernetes.io/master=")

		g.By("Rereate fileintegrity with a new nodeselector and compare Aide-scan pod number and Node number")
		fi1.removeFileintegrity(oc, "deleted")
		fi1.nodeselectorkey = "node.openshift.io/os_id"
		fi1.nodeselectorvalue = "rhel"
		fi1.createFIOWithConfig(oc, itName, dr)
		if getNodeNumberPerLabel(oc, "node.openshift.io/os_id=rhel") != 0 {
			fi1.checkFileintegrityStatus(oc, "running")
			fi1.checkPodNumerEqualNodeNumber(oc, "node.openshift.io/os_id=rhel")
		}

		g.By("Label specific nodeName to node-role.kubernetes.io/test1= !!!\n")
		nodeName := getOneWorkerNodeName(oc)
		defer setLabelToSpecificNode(oc, nodeName, "node-role.kubernetes.io/test1-")
		setLabelToSpecificNode(oc, nodeName, "node-role.kubernetes.io/test1=")
		fi1.removeFileintegrity(oc, "deleted")
		g.By("Rereate fileintegrity with a new nodeselector and compare Aide-scan pod number and Node number")
		fi1.nodeselectorkey = "node-role.kubernetes.io/test1"
		fi1.nodeselectorvalue = ""
		fi1.createFIOWithConfig(oc, itName, dr)
		fi1.checkFileintegrityStatus(oc, "running")
		fi1.checkPodNumerEqualNodeNumber(oc, "node-role.kubernetes.io/test1=")
	})
})
