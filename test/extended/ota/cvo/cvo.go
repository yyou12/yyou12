package cvo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-updates] OTA cvo should", func() {
	defer g.GinkgoRecover()

	project_name := "openshift-cluster-version"

	oc := exutil.NewCLIWithoutNamespace(project_name)

	//author: jialiu@redhat.com
	g.It("Author:jialiu-Medium-41391-cvo serves metrics over only https not http", func() {
		g.By("Check cvo delopyment config file...")
		cvo_deployment_yaml, err := GetDeploymentsYaml(oc, "cluster-version-operator", project_name)
		o.Expect(err).NotTo(o.HaveOccurred())
		var keywords = []string{"--listen=0.0.0.0:9099", "--serving-cert-file=/etc/tls/serving-cert/tls.crt", "--serving-key-file=/etc/tls/serving-cert/tls.key"}
		for _, v := range keywords {
			o.Expect(cvo_deployment_yaml).Should(o.ContainSubstring(v))
		}

		g.By("Check cluster-version-operator binary help")
		cvo_pods_list, err := exutil.WaitForPods(oc.AdminKubeClient().CoreV1().Pods(project_name), exutil.ParseLabelsOrDie("k8s-app=cluster-version-operator"), exutil.CheckPodIsReady, 1, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Get cvo pods: %v", cvo_pods_list)
		output, err := PodExec(oc, "/usr/bin/cluster-version-operator start --help", project_name, cvo_pods_list[0])
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		keywords = []string{"You must set both --serving-cert-file and --serving-key-file unless you set --listen empty"}
		for _, v := range keywords {
			o.Expect(output).Should(o.ContainSubstring(v))
		}

		g.By("Verify cvo metrics is only exported via https")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("servicemonitor", "cluster-version-operator", "-n", project_name, "-o=json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		var result map[string]interface{}
		json.Unmarshal([]byte(output), &result)
		endpoints := result["spec"].(map[string]interface{})["endpoints"]
		e2e.Logf("Get cvo's spec.endpoints: %v", endpoints)
		o.Expect(endpoints).Should(o.HaveLen(1))

		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("servicemonitor", "cluster-version-operator", "-n", project_name, "-o=jsonpath={.spec.endpoints[].scheme}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Get cvo's spec.endpoints scheme: %v", output)
		o.Expect(output).Should(o.Equal("https"))

		g.By("Get cvo endpoint URI")
		//output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("endpoints", "cluster-version-operator", "-n", project_name, "-o=jsonpath='{.subsets[0].addresses[0].ip}:{.subsets[0].ports[0].port}'").Output()
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("endpoints", "cluster-version-operator", "-n", project_name, "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		re := regexp.MustCompile(`cluster-version-operator\s+([^\s]*)`)
		matched_result := re.FindStringSubmatch(output)
		e2e.Logf("Regex mached result: %v", matched_result)
		o.Expect(matched_result).Should(o.HaveLen(2))
		endpoint_uri := matched_result[1]
		e2e.Logf("Get cvo endpoint URI: %v", endpoint_uri)
		o.Expect(endpoint_uri).ShouldNot(o.BeEmpty())

		g.By("Check metric server is providing service https, but not http")
		cmd := fmt.Sprintf("curl http://%s/metrics", endpoint_uri)
		output, err = PodExec(oc, cmd, project_name, cvo_pods_list[0])
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		keywords = []string{"Client sent an HTTP request to an HTTPS server"}
		for _, v := range keywords {
			o.Expect(output).Should(o.ContainSubstring(v))
		}

		g.By("Check metric server is providing service via https correctly.")
		cmd = fmt.Sprintf("curl -k -I https://%s/metrics", endpoint_uri)
		output, err = PodExec(oc, cmd, project_name, cvo_pods_list[0])
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		keywords = []string{"HTTP/1.1 200 OK"}
		for _, v := range keywords {
			o.Expect(output).Should(o.ContainSubstring(v))
		}
	})

	//author: yanyang@redhat.com
	g.It("Longduration-Author:yanyang-Medium-32138-cvo alert should not be fired when RetrievedUpdates failed due to nochannel [Serial][Slow]", func() {
		orgChannel, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].spec.channel}").Output()

		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", orgChannel).Execute()

		g.By("Enable alert by clearing channel")
		err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check RetrievedUpdates condition")
		reason, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].status.conditions[?(.type=='RetrievedUpdates')].reason}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(reason).To(o.Equal("NoChannel"))

		g.By("Alert CannotRetrieveUpdates does not appear within 60m")
		appeared, _, err := waitForAlert(oc, "CannotRetrieveUpdates", 600, 3600, "")
		o.Expect(appeared).NotTo(o.BeTrue())
		o.Expect(err.Error()).To(o.ContainSubstring("timed out waiting for the condition"))

		g.By("Alert CannotRetrieveUpdates does not appear after 60m")
		appeared, _, err = waitForAlert(oc, "CannotRetrieveUpdates", 300, 600, "")
		o.Expect(appeared).NotTo(o.BeTrue())
		o.Expect(err.Error()).To(o.ContainSubstring("timed out waiting for the condition"))
	})
})
