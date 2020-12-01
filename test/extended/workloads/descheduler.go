package workloads

import (
	"path/filepath"
	"regexp"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-scheduling] Workloads", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: yinzhou@redhat.com
	g.It("High-36616-Descheduler NodeAffinity Strategy", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		operatorGroupT := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subscriptionT := filepath.Join(buildPruningBaseDir, "subscription.yaml")
		deschedulerT := filepath.Join(buildPruningBaseDir, "kubedescheduler.yaml")
		configFile := filepath.Join(buildPruningBaseDir, "policy.cfg")
		deployT := filepath.Join(buildPruningBaseDir, "deploy_nodeaffinity.yaml")

		var kubeNamespace = "openshift-kube-descheduler-operator"
		var tnamespace = "test-36616"
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)

		sub := subscription{
			name:        "cluster-kube-descheduler-operator",
			namespace:   kubeNamespace,
			channelName: "4.7",
			opsrcName:   "qe-app-registry",
			sourceName:  "openshift-marketplace",
			template:    subscriptionT,
		}

		testd := deploynodeaffinity{
			dName:          "d36616",
			namespace:      tnamespace,
			replicaNum:     1,
			labelKey:       "app36616",
			labelValue:     "d36616",
			affinityKey:    "e2e-az-NorthSouth",
			operatorPolicy: "In",
			affinityValue1: "e2e-az-North",
			affinityValue2: "e2e-az-South",
			template:       deployT,
		}

		og := operatorgroup{
			name:        "openshift-kube-descheduler-operator-",
			namespace:   kubeNamespace,
			providedApi: "KubeDescheduler.v1beta1.operator.openshift.io",
			template:    operatorGroupT,
		}

		g.By("Create the descheduler project")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()

		g.By("Create the operatorgroup")
		og.createOperatorGroup(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("operatorgroup", "--all", "-n", kubeNamespace).Execute()

		g.By("Create the subscription")
		sub.createSubscription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("subscription", "--all", "-n", kubeNamespace).Execute()

		g.By("Wait for the descheduler operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "descheduler-operator", kubeNamespace, "1"); ok {
			e2e.Logf("Kubedescheduler operator runnnig now\n")
		}

		g.By("Create the configmap")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", "descheduler-policy", "-n", kubeNamespace, "--from-file="+configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get the descheduler image from CSV")
		imageInfo := getImageFromCSV(oc, kubeNamespace)
		deschu := kubedescheduler{
			namespace:        kubeNamespace,
			interSeconds:     60,
			imageInfo:        imageInfo,
			logLevel:         "Normal",
			apiVersionInfo:   "descheduler/v1alpha1",
			operatorLogLevel: "Normal",
			policyName:       "descheduler-policy",
			template:         deschedulerT,
		}
		g.By("Create the descheduler cluster")
		deschu.createKubeDescheduler(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("KubeDescheduler", "--all", "-n", kubeNamespace).Execute()

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "cluster", kubeNamespace, "1")

		g.By("Create test project")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", tnamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", tnamespace).Execute()

		g.By("Create the test deploy")
		testd.createDeployNodeAffinity(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all the pods should be pending")
		if ok := checkPodsStatusByLabel(oc, tnamespace, testd.labelKey+"="+testd.labelValue, "Pending"); ok {
			e2e.Logf("All pods are in Pending status\n")
		}

		g.By("label the node1")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth", "e2e-az-North")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth")

		g.By("Check all the pods should running on node1")
		podNodeList := getPodNodeListByLabel(oc, tnamespace, testd.labelKey)

		g.By("Checking all the pods scheduled to node with testnode label")
		for _, nodeName := range podNodeList {
			e2e.ExpectEqual(nodeList.Items[0].Name, nodeName)
		}

		g.By("Remove the lalbe from node1 and label node2 ")
		e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "e2e-az-NorthSouth", "e2e-az-North")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "e2e-az-NorthSouth")

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "deploy", "cluster", regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason=" (NodeAffinity)"`))
	})
})

