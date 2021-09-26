package logging

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"time"
)

var _ = g.Describe("[sig-openshift-logging] Logging", func() {
	defer g.GinkgoRecover()

	var (
		oc             = exutil.NewCLI("logging-es", exutil.KubeConfigPath())
		eo             = "elasticsearch-operator"
		clo            = "cluster-logging-operator"
		cloPackageName = "cluster-logging"
		eoPackageName  = "elasticsearch-operator"
	)

	g.Context("Cluster Logging Instance tests", func() {
		var (
			subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
			SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
			AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
			jsonLogFile       = exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
		)
		cloNS := "openshift-logging"
		eoNS := "openshift-operators-redhat"
		CLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, CatalogSourceObjects{}}
		EO := SubscriptionObjects{eo, eoNS, AllNamespaceOG, subTemplate, eoPackageName, CatalogSourceObjects{}}

		g.BeforeEach(func() {
			g.By("deploy CLO and EO")
			CLO.SubscribeLoggingOperators(oc)
			EO.SubscribeLoggingOperators(oc)
		})

		// author ikanse@redhat.com
		g.It("CPaasrunOnly-Author:ikanse-Medium-36368-Elasticsearch nodes can scale down[Serial][Slow]", func() {
			// create clusterlogging instance with elasticsearch node count set to 3
			g.By("deploy EFK pods")
			sc, err := getStorageClassName(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-storage-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "ES_NODE_COUNT=3", "-p", "REDUNDANCY_POLICY=SingleRedundancy")

			e2e.Logf("Start testing OCP-36368-Elasticsearch nodes can scale down")
			//Wait for EFK pods to be ready
			g.By("Waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			g.By("Check the elasticsearch node count")
			cl.assertResourceStatus(oc, "jsonpath={.status.logStore.elasticsearchStatus[0].cluster.numNodes}", "3")

			g.By("Check the elasticsearch cluster health")
			cl.assertResourceStatus(oc, "jsonpath={.status.logStore.elasticsearchStatus[0].cluster.status}", "green")

			g.By("Set elasticsearch node count to 2")
			er := oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterlogging/instance", "-n", "openshift-logging", "-p", "{\"spec\": {\"logStore\": {\"elasticsearch\": {\"nodeCount\":2}}}}", "--type=merge").Execute()
			o.Expect(er).NotTo(o.HaveOccurred())

			g.By("Check the elasticsearch node count")
			cl.assertResourceStatus(oc, "jsonpath={.status.logStore.elasticsearchStatus[0].cluster.numNodes}", "2")

			g.By("Check the elasticsearch cluster health")
			cl.assertResourceStatus(oc, "jsonpath={.status.logStore.elasticsearchStatus[0].cluster.status}", "green")
		})

		// author ikanse@redhat.com
		g.It("CPaasrunOnly-Author:ikanse-Medium-43065-Drop log messages after explicit time[Serial][Slow]", func() {

			g.By(" Create a Cluster Logging instance with Fluentd buffer retryTimeout set to 1 minute.")
			sc, err := getStorageClassName(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "43065.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "ES_NODE_COUNT=1", "-p", "REDUNDANCY_POLICY=ZeroRedundancy", "-p", "FLUENTD_BUFFER_RETRYTIMEOUT=1m")

			g.By("Waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			g.By("Make sure the Elasticsearch cluster is healthy")
			cl.assertResourceStatus(oc, "jsonpath={.status.logStore.elasticsearchStatus[0].cluster.status}", "green")
			prePodList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, prePodList.Items[0].Name, "infra-00", "")

			g.By("Set the Elasticsearch operator instance managementState to Unmanaged.")
			err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("es/elasticsearch", "-n", cloNS, "-p", "{\"spec\": {\"managementState\": \"Unmanaged\"}}", "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Scale down the Elasticsearch deployment to 0.")
			deployList := GetDeploymentsNameByLabel(oc, cloNS, "component=elasticsearch")
			for _, name := range deployList {
				err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", name, "--replicas=0", "-n", cloNS).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			WaitUntilPodsAreGone(oc, cloNS, "component=elasticsearch")

			g.By("Create an instance of the logtest app")
			oc.SetupProject()
			app_proj := oc.Namespace()
			cerr := oc.WithoutNamespace().Run("new-app").Args("-n", app_proj, "-f", jsonLogFile).Execute()
			o.Expect(cerr).NotTo(o.HaveOccurred())

			g.By("Make sure the logtest app has generated logs")
			appPodList, err := oc.AdminKubeClient().CoreV1().Pods(app_proj).List(metav1.ListOptions{LabelSelector: "run=centos-logtest"})
			o.Expect(err).NotTo(o.HaveOccurred())
			pl := resource{"pods", appPodList.Items[0].Name, app_proj}
			pl.assertResourceStatus(oc, "jsonpath={.status.phase}", "Running")
			pl.checkLogsFromRs(oc, "foobar")

			g.By("Delete the logtest app namespace")
			DeleteNamespace(oc, app_proj)

			g.By("Wait for 3 minutes for logtest app logs to be discarded")
			time.Sleep(180 * time.Second)

			g.By("Scale back the elasticsearch deployment to 1 replica")
			for _, name := range deployList {
				err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", name, "--replicas=1", "-n", cloNS).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				WaitForDeploymentPodsToBeReady(oc, cloNS, name)
			}
			WaitForEFKPodsToBeReady(oc, cloNS)

			g.By("Get the log count for logtest app namespace")
			postPodList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, postPodList.Items[0].Name, "infra-00", "")
			LogCount, err := getDocCountPerNamespace(oc, cloNS, postPodList.Items[0].Name, app_proj, "app")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Logcount for the logtest app in %s project is %d", app_proj, LogCount)

			g.By("Check if the logtest application logs are discarded")
			o.Expect(LogCount).To(o.Equal(0), "The log count for the %s namespace should be 0", app_proj)
		})
	})
})
