package logging

import (
	"fmt"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-openshift-logging] Logging", func() {
	defer g.GinkgoRecover()

	var (
		oc             = exutil.NewCLI("logging-es-"+getRandomString(), exutil.KubeConfigPath())
		eo             = "elasticsearch-operator"
		clo            = "cluster-logging-operator"
		cloPackageName = "cluster-logging"
		eoPackageName  = "elasticsearch-operator"
	)

	g.Context("Elasticsearch should", func() {
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

			// create clusterlogging instance
			g.By("deploy EFK pods")
			sc, err := getStorageClassName(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-storage-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			cl.applyFromTemplate(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc)
		})
		/*
			g.AfterEach(func() {
				cl := resource{"clusterlogging", "instance", cloNS}
				cl.deleteClusterLogging(oc)
			})
		*/

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-Medium-43444-Expose Index Level Metrics es_index_namespaces_total and es_index_document_count", func() {
			g.By("waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			g.By("check logs in ES pod")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "infra-00", "")

			g.By("check ES metric es_index_namespaces_total")
			err = wait.Poll(5*time.Second, 120*time.Second, func() (done bool, err error) {
				metricData1, err := queryPrometheus(oc, "", "/api/v1/query?", "es_index_namespaces_total", "GET")
				if err != nil {
					return false, err
				} else {
					if len(metricData1.Data.Result) == 0 {
						return false, nil
					} else {
						namespaceCount, _ := strconv.Atoi(metricData1.Data.Result[0].Value[1].(string))
						e2e.Logf("\nthe namespace count is: %d", namespaceCount)
						if namespaceCount > 0 {
							return true, nil
						} else {
							return false, nil
						}
					}
				}
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The value of metric %s isn't more than 0", "es_index_namespaces_total"))

			g.By("check ES metric es_index_document_count")
			metricData2, err := queryPrometheus(oc, "", "/api/v1/query?", "es_index_document_count", "GET")
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, content := range metricData2.Data.Result {
				metricValue, _ := strconv.Atoi(content.Value[1].(string))
				o.Expect(metricValue > 0).Should(o.BeTrue())
			}
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-Low-43081-remove JKS certificates", func() {
			g.By("waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			g.By("check certificates in ES")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			cmd := "ls /etc/elasticsearch/secret/"
			stdout, err := e2e.RunHostCmdWithRetries(cloNS, podList.Items[0].Name, cmd, 3*time.Second, 30*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(stdout).ShouldNot(o.ContainSubstring("admin.jks"))
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-Medium-42943-remove template org.ovirt.viaq-collectd.template.json", func() {
			g.By("waiting for the EFK pods to be ready...")
			WaitForEFKPodsToBeReady(oc, cloNS)

			g.By("check templates in ES")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			cmd := "ls /usr/share/elasticsearch/index_templates/"
			stdout, err := e2e.RunHostCmdWithRetries(cloNS, podList.Items[0].Name, cmd, 3*time.Second, 30*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(stdout).ShouldNot(o.ContainSubstring("org.ovirt.viaq-collectd.template.json"))
		})

	})

})
