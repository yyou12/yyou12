package hive

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-hive] Cluster_Operator hive should", func() {
	defer g.GinkgoRecover()

	var (
		oc  = exutil.NewCLI("hive-"+getRandomString(), exutil.KubeConfigPath())
		og  operatorGroup
		sub subscription
		hc  hiveconfig
	)
	g.BeforeEach(func() {
		testDataDir := exutil.FixturePath("testdata", "cluster_operator/hive")
		ogTemp := filepath.Join(testDataDir, "operatorgroup.yaml")
		subTemp := filepath.Join(testDataDir, "subscription.yaml")
		hcTemp := filepath.Join(testDataDir, "hiveconfig.yaml")

		og = operatorGroup{
			name:      "hive-og",
			namespace: "",
			template:  ogTemp,
		}

		sub = subscription{
			name:            "hive-sub",
			namespace:       "",
			channel:         "alpha",
			approval:        "Automatic",
			operatorName:    "hive-operator",
			sourceName:      "community-operators",
			sourceNamespace: "openshift-marketplace",
			startingCSV:     "",
			currentCSV:      "",
			installedCSV:    "",
			template:        subTemp,
		}

		hc = hiveconfig{
			logLevel:        "debug",
			targetNamespace: "",
			template:        hcTemp,
		}
	})

	//author: lwan@redhat.com
	g.It("ConnectedOnly-Author:lwan-Critical-29670-install/uninstall hive operator from OperatorHub", func() {
		oc.SetupProject()
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		hc.targetNamespace = oc.Namespace()
		g.By("Create OperatorGroup...")
		og.create(oc)

		g.By("Create Subscription...")
		sub.create(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, []string{"sub", sub.name, "-n",
			sub.namespace, "-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

		defer cleanupObjects(oc,
			objectTableRef{"subscription", sub.namespace, sub.name},
			objectTableRef{"operatorgroup", og.namespace, og.name},
			objectTableRef{"csv", sub.namespace, sub.installedCSV})

		g.By("Check CSV is created sucessfully !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace,
			"-o=jsonpath={.status.phase}"}).check(oc)
		g.By("Check Hive Operator pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hive-operator", ok, []string{"pod", "--selector=control-plane=hive-operator",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check Hive Operator pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=control-plane=hive-operator", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Hive Operator sucessfully installed !!! ")

		g.By("Create hiveconfig !!!")
		hc.create(oc)

		defer cleanupObjects(oc, objectTableRef{"hiveconfig", "", "hive"})

		g.By("Check hive-clustersync pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hive-clustersync", ok, []string{"pod", "--selector=control-plane=clustersync",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hive-clustersync pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=control-plane=clustersync", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Check hive-controllers pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hive-controllers", ok, []string{"pod", "--selector=control-plane=controller-manager",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hive-controllers pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=control-plane=controller-manager", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Check hiveadmission pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hiveadmission", ok, []string{"pod", "--selector=app=hiveadmission",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hiveadmission pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running Running", ok, []string{"pod", "--selector=app=hiveadmission", "-n",
			sub.namespace, "-o=jsonpath={.items[*].status.phase}"}).check(oc)
		g.By("Hive controllers,clustersync and hiveadmission sucessfully installed !!! ")
	})
})
