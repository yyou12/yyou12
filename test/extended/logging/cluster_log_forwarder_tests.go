package logging

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-openshift-logging] Logging", func() {
	var oc = exutil.NewCLI("logfwd-namespace", exutil.KubeConfigPath())
	defer g.GinkgoRecover()

	var (
		eo             = "elasticsearch-operator"
		clo            = "cluster-logging-operator"
		cloPackageName = "cluster-logging"
		eoPackageName  = "elasticsearch-operator"
	)

	g.Context("Log Forward with namespace selector in the CLF", func() {
		var (
			subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
			SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
			AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
		)
		cloNS := "openshift-logging"
		eoNS := "openshift-operators-redhat"
		CLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, CatalogSourceObjects{}}
		EO := SubscriptionObjects{eo, eoNS, AllNamespaceOG, subTemplate, eoPackageName, CatalogSourceObjects{}}
		g.BeforeEach(func() {
			g.By("deploy CLO and EO")
			CLO.SubscribeLoggingOperators(oc)
			EO.SubscribeLoggingOperators(oc)
			oc.SetupProject()
		})

		g.It("CPaasrunOnly-Author:kbharti-High-41598-forward logs only from specific pods via a label selector inside the Log Forwarding API[Serial]", func() {

			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_non_json_log_template.json")
			)
			// Dev label - create a project and pod in the project to generate some logs
			g.By("create application for logs with dev label")
			app_proj_dev := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", app_proj_dev, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-dev").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// QA label - create a project and pod in the project to generate some logs
			g.By("create application for logs with qa label")
			oc.SetupProject()
			app_proj_qa := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", app_proj_qa, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-qa").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogForwarder instance
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "41598.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate)
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			//check app index in ES
			g.By("check indices in ES pod")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "app-000001", "")

			//Waiting for the app index to be populated
			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, app_proj_qa, "app-000001")

			// check data in ES for QA namespace
			g.By("check logs in ES pod for QA namespace in CLF")
			check_log := "es_util --query=app-*/_search?format=JSON -d '{\"size\": 1, \"sort\": [{\"@timestamp\": {\"order\":\"desc\"}}], \"query\": {\"match\": {\"kubernetes.namespace_name\": \"" + app_proj_qa + "\"}}}'"
			logs := searchInES(oc, cloNS, podList.Items[0].Name, check_log)
			o.Expect(logs.Hits.DataHits[0].Source.Kubernetes.NamespaceLabels.KubernetesIOMetadataName).Should(o.Equal(app_proj_qa))

			//check that no data exists for the other Dev namespace - Negative test
			g.By("check logs in ES pod for Dev namespace in CLF")
			count, err := getDocCountPerNamespace(oc, cloNS, podList.Items[0].Name, app_proj_dev, "app-000001")
			o.Expect(count).Should(o.Equal(0))

		})

		g.It("CPaasrunOnly-Author:kbharti-High-41599-Forward Logs from specified pods combining namespaces and label selectors[Serial]", func() {

			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_non_json_log_template.json")
			)

			g.By("create application for logs with dev1 label")
			app_proj_dev := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", app_proj_dev, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-dev-1", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-dev1", "-p", "CONFIGMAP=logtest-config-dev1").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application for logs with dev2 label")
			err = oc.WithoutNamespace().Run("new-app").Args("-n", app_proj_dev, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-dev-2", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-dev2", "-p", "CONFIGMAP=logtest-config-dev2").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application for logs with qa1 label")
			oc.SetupProject()
			app_proj_qa := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", app_proj_qa, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-qa-1", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-qa1", "-p", "CONFIGMAP=logtest-config-qa1").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application for logs with qa2 label")
			err = oc.WithoutNamespace().Run("new-app").Args("-n", app_proj_qa, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-qa-2", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-qa2", "-p", "CONFIGMAP=logtest-config-qa2").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogForwarder instance
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "41599.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "APP_NAMESPACE_QA="+app_proj_qa, "-p", "APP_NAMESPACE_DEV="+app_proj_dev)
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			//check app index in ES
			g.By("check indices in ES pod")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "app-00", "")

			//Waiting for the app index to be populated
			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, app_proj_qa, "app-00")
			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, app_proj_dev, "app-00")

			g.By("check doc count in ES pod for QA1 namespace in CLF")
			logCount, err := getDocCountByK8sLabel(oc, cloNS, podList.Items[0].Name, "app-00", "run=centos-logtest-qa-1")
			o.Expect(logCount).ShouldNot(o.Equal(0))

			g.By("check doc count in ES pod for QA2 namespace in CLF")
			logCount, err = getDocCountByK8sLabel(oc, cloNS, podList.Items[0].Name, "app-00", "run=centos-logtest-qa-2")
			o.Expect(logCount).Should(o.Equal(0))

			g.By("check doc count in ES pod for DEV1 namespace in CLF")
			logCount, err = getDocCountByK8sLabel(oc, cloNS, podList.Items[0].Name, "app-00", "run=centos-logtest-dev-1")
			o.Expect(logCount).ShouldNot(o.Equal(0))

			g.By("check doc count in ES pod for DEV2 namespace in CLF")
			logCount, err = getDocCountByK8sLabel(oc, cloNS, podList.Items[0].Name, "app-00", "run=centos-logtest-dev-2")
			o.Expect(logCount).Should(o.Equal(0))

		})

		g.It("CPaasrunOnly-Author:ikanse-High-42981-Collect OVN audit logs [Serial]", func() {

			g.By("Create clusterlogforwarder instance to forward OVN audit logs to default Elasticsearch instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "42981.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err := clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			g.By("Check audit index in ES pod")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "audit-00", "")

			g.By("Create a test project, enable OVN network log collection on it, add the OVN log app and network policies for the project")
			oc.SetupProject()
			ovn_proj := oc.Namespace()
			ovn := resource{"deployment", "ovn-app", ovn_proj}
			esTemplate := exutil.FixturePath("testdata", "logging", "generatelog", "42981.yaml")
			err = ovn.applyFromTemplate(oc, "-n", ovn.namespace, "-f", esTemplate, "-p", "NAMESPACE="+ovn.namespace)
			o.Expect(err).NotTo(o.HaveOccurred())
			WaitForDeploymentPodsToBeReady(oc, ovn_proj, ovn.name)

			g.By("Access the OVN app pod from another pod in the same project to generate OVN ACL messages")
			podList, err = oc.AdminKubeClient().CoreV1().Pods(ovn_proj).List(metav1.ListOptions{LabelSelector: "app=ovn-app"})
			o.Expect(err).NotTo(o.HaveOccurred())
			podIP := podList.Items[0].Status.PodIP
			e2e.Logf("Pod IP is %s ", podIP)
			ovn_curl := "curl " + podIP + ":8080"
			_, err = e2e.RunHostCmdWithRetries(ovn_proj, podList.Items[1].Name, ovn_curl, 3*time.Second, 30*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check for the generated OVN audit logs on the OpenShift cluster nodes")
			nodeLogs, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("-n", ovn_proj, "node-logs", "-l", "beta.kubernetes.io/os=linux", "--path=/ovn/acl-audit-log.log").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(nodeLogs).Should(o.ContainSubstring(ovn_proj), "The OVN logs doesn't contain logs from project %s", ovn_proj)

			g.By("Check for the generated OVN audit logs in Elasticsearch")
			podList, err = oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			check_log := "es_util --query=audit*/_search?format=JSON -d '{\"query\":{\"query_string\":{\"query\":\"verdict=allow AND severity=alert AND tcp,vlan_tci AND tcp_flags=ack\",\"default_field\":\"message\"}}}'"
			logs := searchInES(oc, cloNS, podList.Items[0].Name, check_log)
			o.Expect(logs.Hits.Total).Should(o.BeNumerically(">", 0))
		})
	})
})
