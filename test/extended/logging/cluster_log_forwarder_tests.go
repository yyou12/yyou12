package logging

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	})

})
