package hive

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	ci "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
)

// [Test Case Naming Rule Add-on]
// - For long duration run such as clusterpool/clusterdeployment, need to add "NonPreRelease"
// - platform specific case, need to add "[platform type]"
// - Add submodule like "ClusterPool", "ClusterDeployment" then we can run all cases for the submodule only

// [Test Resource Naming Rule]
// - Add test case Id into the resource name especially cluster-level resource to avoid name conflict in parallel run
// - Make the resource names in good correlation, the following is the rule example
//  	ClusterPool name:  poolName = pool-<test case Id>
//		Its linked ClusterImageSet name: imageSetName = poolName + "-imageset"
//		ClusterClaim name from the ClusterPool: claimName = poolName + "-claim" (This is to trim "-claim" directly to get the pool name and check if its claimed clusterdeployment delete done when deleting the clusterclaim)

var _ = g.Describe("[sig-hive] Cluster_Operator hive should", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("hive-"+getRandomString(), exutil.KubeConfigPath())
		ns           hiveNameSpace
		og           operatorGroup
		sub          subscription
		hc           hiveconfig
		testDataDir  string
		iaasPlatform string
	)
	g.BeforeEach(func() {
		testDataDir = exutil.FixturePath("testdata", "cluster_operator/hive")
		nsTemp := filepath.Join(testDataDir, "namespace.yaml")
		ogTemp := filepath.Join(testDataDir, "operatorgroup.yaml")
		subTemp := filepath.Join(testDataDir, "subscription.yaml")
		hcTemp := filepath.Join(testDataDir, "hiveconfig.yaml")

		ns = hiveNameSpace{
			name:     HIVE_NAMESPACE,
			template: nsTemp,
		}

		og = operatorGroup{
			name:      "hive-og",
			namespace: HIVE_NAMESPACE,
			template:  ogTemp,
		}

		sub = subscription{
			name:            "hive-sub",
			namespace:       HIVE_NAMESPACE,
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
			targetNamespace: HIVE_NAMESPACE,
			template:        hcTemp,
		}

		// get IaaS platform
		iaasPlatform = ci.CheckPlatform(oc)

		//Create Hive Resources if not exist
		g.By("Create Hive NameSpace...")
		ns.createIfNotExist(oc)
		g.By("Create OperatorGroup...")
		og.createIfNotExist(oc)
		g.By("Create Subscription...")
		sub.createIfNotExist(oc)
		g.By("Create hiveconfig !!!")
		hc.createIfNotExist(oc)

	})

	//author: lwan@redhat.com
	g.It("ConnectedOnly-Author:lwan-Critical-29670-install/uninstall hive operator from OperatorHub", func() {
		g.By("Check Subscription...")
		newCheck("expect", asAdmin, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, DEFAULT_TIMEOUT, []string{"sub", sub.name, "-n",
			sub.namespace, "-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

		g.By("Check Hive Operator pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hive-operator", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=control-plane=hive-operator",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check Hive Operator pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=control-plane=hive-operator", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Hive Operator sucessfully installed !!! ")

		g.By("Check hive-clustersync pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hive-clustersync", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=control-plane=clustersync",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hive-clustersync pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=control-plane=clustersync", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Check hive-controllers pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hive-controllers", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=control-plane=controller-manager",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hive-controllers pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=control-plane=controller-manager", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Check hiveadmission pods are created !!!")
		newCheck("expect", asAdmin, withoutNamespace, contain, "hiveadmission", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=app=hiveadmission",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hiveadmission pods are in running state !!!")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running Running", ok, DEFAULT_TIMEOUT, []string{"pod", "--selector=app=hiveadmission", "-n",
			sub.namespace, "-o=jsonpath={.items[*].status.phase}"}).check(oc)
		g.By("Hive controllers,clustersync and hiveadmission sucessfully installed !!! ")
	})

	//author: jshu@redhat.com
	//default duration is 15m for extended-platform-tests and 35m for jenkins job, need to reset for ClusterPool and ClusterDeployment cases
	//example: ./bin/extended-platform-tests run all --dry-run|grep "33832"|./bin/extended-platform-tests run --timeout 60m -f -
	g.It("NonPreRelease-ConnectedOnly-Author:jshu-Medium-33832-[aws]Hive supports ClusterPool", func() {
		if iaasPlatform != "aws" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 33832 is for AWS - skipping test ...")
		}
		testCaseId := "33832"
		poolName := "pool-" + testCaseId
		imageSetName := poolName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP_RELEASE_IMAGE,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{CLUSTER_IMAGE_SET, "", imageSetName})
		imageSet.create(oc)

		g.By("Check if ClusterImageSet was created successfully")
		newCheck("expect", asAdmin, withoutNamespace, contain, imageSetName, ok, DEFAULT_TIMEOUT, []string{CLUSTER_IMAGE_SET}).check(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and aws-creds to target namespace for the pool
		g.By("Copy AWS platform credentials...")
		createAWSCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create ClusterPool...")
		poolTemp := filepath.Join(testDataDir, "clusterpool.yaml")
		pool := clusterPool{
			name:          poolName,
			namespace:     oc.Namespace(),
			baseDomain:    AWS_BASE_DOMAIN,
			imageSetRef:   imageSetName,
			credRef:       AWS_CREDS,
			region:        AWS_REGION,
			pullSecretRef: PULL_SECRET,
			size:          1,
			maxSize:       1,
			template:      poolTemp,
		}
		defer cleanupObjects(oc, objectTableRef{CLUSTER_POOL, oc.Namespace(), poolName})
		pool.create(oc)
		g.By("Check if ClusterPool created successfully and become ready")
		newCheck("expect", asAdmin, withoutNamespace, contain, poolName, ok, DEFAULT_TIMEOUT, []string{CLUSTER_POOL, "-n", oc.Namespace()}).check(oc)
		poolReadyString := "\"ready\":1"
		newCheck("expect", asAdmin, withoutNamespace, contain, poolReadyString, ok, CLUSTER_INSTALL_TIMEOUT, []string{CLUSTER_POOL, poolName, "-n", oc.Namespace(), "-o=jsonpath={.status}"}).check(oc)

		g.By("Create ClusterClaim...")
		claimTemp := filepath.Join(testDataDir, "clusterclaim.yaml")
		claimName := poolName + "-claim"
		claim := clusterClaim{
			name:            claimName,
			namespace:       oc.Namespace(),
			clusterPoolName: poolName,
			template:        claimTemp,
		}
		defer cleanupObjects(oc, objectTableRef{CLUSTER_CLAIM, oc.Namespace(), claimName})
		claim.create(oc)
		g.By("Check if ClusterClaim created successfully and become running")
		newCheck("expect", asAdmin, withoutNamespace, contain, claimName, ok, DEFAULT_TIMEOUT, []string{CLUSTER_CLAIM, "-n", oc.Namespace()}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, CLUSTER_RESUME_TIMEOUT, []string{CLUSTER_CLAIM, "-n", oc.Namespace()}).check(oc)
	})
})
