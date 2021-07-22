package workloads

import (
	"path/filepath"
        "regexp"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

        "strings"
        "time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
        "k8s.io/apimachinery/pkg/util/wait"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-scheduling] Workloads The Descheduler Operator automates pod evictions using different profiles", func() {
       defer g.GinkgoRecover()
       var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())
       var kubeNamespace  = "openshift-kube-descheduler-operator"

       buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
       operatorGroupT := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
       subscriptionT := filepath.Join(buildPruningBaseDir, "subscription.yaml")
       deschedulerT := filepath.Join(buildPruningBaseDir, "kubedescheduler.yaml")

       sub := subscription{
                        name:        "cluster-kube-descheduler-operator",
                        namespace:   kubeNamespace,
                        channelName: "4.9",
                        opsrcName:   "qe-app-registry",
                        sourceName:  "openshift-marketplace",
                        template:    subscriptionT,
       }

       og := operatorgroup{
                        name:        "openshift-kube-descheduler-operator",
                        namespace:   kubeNamespace,
                        template:    operatorGroupT,
       }

       deschu := kubedescheduler{
                        namespace:         kubeNamespace,
                        interSeconds:      60,
                        imageInfo:         "registry.redhat.io/openshift4/ose-descheduler:v4.9.0",
                        logLevel:          "Normal",
                        operatorLogLevel:  "Normal",
                        profile1:          "AffinityAndTaints",
                        profile2:          "TopologyAndDuplicates",
                        profile3:          "LifecycleAndUtilization",
                        template:          deschedulerT,
       }

       // author: knarra@redhat.com
       g.It("Author:knarra-High-21205-Install descheduler operator via a deployment [Serial]", func() {

            _, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)

            g.By("Create the descheduler namespace")
            err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Execute()
            o.Expect(err).NotTo(o.HaveOccurred())
            defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()

            g.By("Create the operatorgroup")
            og.createOperatorGroup(oc)
            o.Expect(err).NotTo(o.HaveOccurred())
            defer og.deleteOperatorGroup(oc)

            g.By("Create the subscription")
            sub.createSubscription(oc)
            o.Expect(err).NotTo(o.HaveOccurred())
            defer sub.deleteSubscription(oc)

            g.By("Wait for the descheduler operator pod running")
            if ok := waitForAvailableRsRunning(oc, "deploy", "descheduler-operator", kubeNamespace, "1"); ok {
                  e2e.Logf("Kubedescheduler operator runnnig now\n")
            }

            g.By("Create descheduler cluster")
            deschu.createKubeDescheduler(oc)
            o.Expect(err).NotTo(o.HaveOccurred())
            defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("KubeDescheduler", "--all", "-n", kubeNamespace).Execute()

            g.By("Check the kubedescheduler run well")
            checkAvailable(oc, "deploy", "cluster", kubeNamespace, "1")

            g.By("Get descheduler cluster pod name")
            podName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
            o.Expect(err).NotTo(o.HaveOccurred())


            g.By("Validate all profiles have been enabled checking descheduler cluster logs")
            profile_details := []string{"duplicates.go", "lownodeutilization.go", "pod_antiaffinity.go", "node_affinity.go", "node_taint.go", "toomanyrestarts.go", "pod_lifetime.go", "topologyspreadconstraint.go"}
            for _, pd := range profile_details {
                checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(pd))
            }

       })

      // author: knarra@redhat.com
      g.It("Author:knarra-High-37463-Descheduler-Validate AffinityAndTaints profile [Serial]", func() {
              deployT  := filepath.Join(buildPruningBaseDir, "deploy_nodeaffinity.yaml")
              deploynT := filepath.Join(buildPruningBaseDir, "deploy_nodetaint.yaml")
              deploypT := filepath.Join(buildPruningBaseDir, "deploy_interpodantiaffinity.yaml")

              nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)

              // Create test project
              g.By("Create test project")
              oc.SetupProject()

              testd := deploynodeaffinity{
			dName:          "d37463",
			namespace:      oc.Namespace(),
			replicaNum:     1,
			labelKey:       "app37463",
			labelValue:     "d37463",
			affinityKey:    "e2e-az-NorthSouth",
			operatorPolicy: "In",
			affinityValue1: "e2e-az-North",
			affinityValue2: "e2e-az-South",
			template:       deployT,
	      }

              testd2 := deploynodetaint{
                        dName:         "d374631",
                        namespace:     oc.Namespace(),
                        template:      deploynT,
              }

              testd3 := deployinterpodantiaffinity{
                        dName:          "d3746321",
                        namespace:      oc.Namespace(),
                        replicaNum:     1,
                        podAffinityKey: "key3746321",
                        operatorPolicy: "In",
                        podAffinityValue: "value3746321",
                        template:       deploypT,
              }

              testd4 := deployinterpodantiaffinity{
                        dName:          "d374632",
                        namespace:      oc.Namespace(),
                        replicaNum:     6,
                        podAffinityKey: "key374632",
                        operatorPolicy: "In",
                        podAffinityValue: "value374632",
                        template:       deploypT,
              }

              g.By("Create the descheduler namespace")
              err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Execute()
              o.Expect(err).NotTo(o.HaveOccurred())
              defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()

              g.By("Create the operatorgroup")
              og.createOperatorGroup(oc)
              o.Expect(err).NotTo(o.HaveOccurred())
              defer og.deleteOperatorGroup(oc)

              g.By("Create the subscription")
              sub.createSubscription(oc)
              o.Expect(err).NotTo(o.HaveOccurred())
              defer sub.deleteSubscription(oc)

              g.By("Wait for the descheduler operator pod running")
              if ok := waitForAvailableRsRunning(oc, "deploy", "descheduler-operator", kubeNamespace, "1"); ok {
                  e2e.Logf("Kubedescheduler operator runnnig now\n")
              }

              g.By("Create descheduler cluster")
              deschu.createKubeDescheduler(oc)
              o.Expect(err).NotTo(o.HaveOccurred())
              defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("KubeDescheduler", "--all", "-n", kubeNamespace).Execute()

              g.By("Check the kubedescheduler run well")
              checkAvailable(oc, "deploy", "cluster", kubeNamespace, "1")

              g.By("Get descheduler cluster pod name")
              podName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
              o.Expect(err).NotTo(o.HaveOccurred())


              // Test for RemovePodsViolatingNodeAffinity

              g.By("Create the test deploy")
	      testd.createDeployNodeAffinity(oc)
	      o.Expect(err).NotTo(o.HaveOccurred())

	      g.By("Check all the pods should be pending")
	      if ok := checkPodsStatusByLabel(oc, oc.Namespace(), testd.labelKey+"="+testd.labelValue, "Pending"); ok {
			e2e.Logf("All pods are in Pending status\n")
	      }

              g.By("label the node1")
	      e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth", "e2e-az-North")
	      defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth")

              g.By("Check all the pods should running on node1")
              waitErr := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
                        msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", testd.namespace).Output()
                        o.Expect(err).NotTo(o.HaveOccurred())
                        if strings.Contains(msg, "Running"){
                                return true, nil
                        }
                        return false, nil
                })
                o.Expect(waitErr).NotTo(o.HaveOccurred())

              testPodName, err := oc.AsAdmin().Run("get").Args("pods", "-l", testd.labelKey+"="+testd.labelValue, "-n", testd.namespace, "-o=jsonpath={.items..metadata.name}").Output()
              o.Expect(err).NotTo(o.HaveOccurred())
              pod37463nodename := getPodNodeName(oc, testd.namespace, testPodName)
              e2e.ExpectEqual(nodeList.Items[0].Name, pod37463nodename)

              g.By("Remove the label from node1 and label node2 ")
	      e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth")
              g.By("label removed from node1")
	      e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "e2e-az-NorthSouth", "e2e-az-South")
              g.By("label Added to node2")

              defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "e2e-az-NorthSouth")

              g.By("Check the descheduler deploy logs, should see evict logs")
	      checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="NodeAffinity"`))

              // Test for RemovePodsViolatingNodeTaints

              g.By("Create the test2 deploy")
              testd2.createDeployNodeTaint(oc)
              pod374631nodename := getPodNodeName(oc, testd2.namespace, "d374631")
              o.Expect(err).NotTo(o.HaveOccurred())

              g.By("Add taint to the node")
              err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod374631nodename, "dedicated=special-user:NoSchedule").Execute()
              o.Expect(err).NotTo(o.HaveOccurred())
              defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod374631nodename, "dedicated-").Execute()

              g.By("Check the descheduler deploy logs, should see evict logs")
              checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="NodeTaint"`))

             // Test for RemovePodsViolatingInterPodAntiAffinity

             g.By("Create the test3 deploy")
             testd3.createDeployInterPodAntiAffinity(oc)
             o.Expect(err).NotTo(o.HaveOccurred())

             g.By("Get pod name")
             podNameIpa, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=d3746321", "-n", testd3.namespace, "-o=jsonpath={.items..metadata.name}").Output()
             o.Expect(err).NotTo(o.HaveOccurred())

             g.By("Create the test4 deploy")
             testd4.createDeployInterPodAntiAffinity(oc)
             o.Expect(err).NotTo(o.HaveOccurred())

             g.By("Add label to the pod")
             err = oc.AsAdmin().WithoutNamespace().Run("label").Args("pod", podNameIpa, "key374632=value374632", "-n", testd3.namespace).Execute()
             o.Expect(err).NotTo(o.HaveOccurred())
             defer oc.AsAdmin().WithoutNamespace().Run("label").Args("pod", podNameIpa, "key374632-", "-n", testd3.namespace).Execute()

            g.By("Check the descheduler deploy logs, should see evict logs")
            checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="InterPodAntiAffinity"`))

         })

})
