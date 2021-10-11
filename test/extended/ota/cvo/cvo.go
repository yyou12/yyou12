package cvo

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"cloud.google.com/go/storage"
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
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("/usr/bin/cluster-version-operator start --help executs error on %v", cvo_pods_list[0]))
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
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cmd %s executs error on %v", cmd, cvo_pods_list[0]))
		e2e.Logf(output)
		keywords = []string{"Client sent an HTTP request to an HTTPS server"}
		for _, v := range keywords {
			o.Expect(output).Should(o.ContainSubstring(v))
		}

		g.By("Check metric server is providing service via https correctly.")
		cmd = fmt.Sprintf("curl -k -I https://%s/metrics", endpoint_uri)
		output, err = PodExec(oc, cmd, project_name, cvo_pods_list[0])
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cmd %s executs error on %v", cmd, cvo_pods_list[0]))
		e2e.Logf(output)
		keywords = []string{"HTTP/1.1 200 OK"}
		for _, v := range keywords {
			o.Expect(output).Should(o.ContainSubstring(v))
		}
	})

	//author: yanyang@redhat.com
	g.It("Longduration-CPaasrunOnly-Author:yanyang-Medium-32138-cvo alert should not be fired when RetrievedUpdates failed due to nochannel [Serial][Slow]", func() {
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

	//author: yanyang@redhat.com
	g.It("ConnectedOnly-Author:yanyang-Medium-43178-manage channel by using oc adm upgrade channel [Serial]", func() {
		projectID := "openshift-qe"
		ctx := context.Background()
		client, err := storage.NewClient(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer client.Close()

		graphURL, bucket, object, err := buildGraph(client, oc, projectID)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer DeleteBucket(client, bucket)
		defer DeleteObject(client, bucket, object)

		orgUpstream, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].spec.upstream}").Output()
		orgChannel, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].spec.channel}").Output()

		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "--allow-explicit-channel", orgChannel).Execute()
			exec.Command("bash", "-c", "sleep 5").Output()
			if orgUpstream == "" {
				oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterversion/version", "--type=json", "-p", "[{\"op\":\"remove\", \"path\":\"/spec/upstream\"}]").Execute()
			} else {
				oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterversion/version", "--type=merge", "--patch", fmt.Sprintf("{\"spec\":{\"upstream\":\"%s\"}}", orgUpstream)).Execute()
			}
		}()

		// Prerequisite: the available channels are not present
		g.By("The test requires the available channels are not present as a prerequisite")
		cmdOut, _ := oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(cmdOut).NotTo(o.ContainSubstring("available channels:"))

		version, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "version", "-o=jsonpath={.status.desired.version}").Output()

		g.By("Set to an unknown channel when available channels are not present")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "unknown-channel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring(fmt.Sprintf("warning: No channels known to be compatible with the current version \"%s\"; unable to validate \"unknown-channel\". Setting the update channel to \"unknown-channel\" anyway.", version)))
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Channel: unknown-channel"))

		g.By("Clear an unknown channel when available channels are not present")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("warning: Clearing channel \"unknown-channel\"; cluster will no longer request available update recommendations."))
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("NoChannel"))

		// Prerequisite: a dummy update server is ready and the available channels is present
		g.By("Change to a dummy update server")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterversion/version", "--type=merge", "--patch", fmt.Sprintf("{\"spec\":{\"upstream\":\"%s\", \"channel\":\"channel-a\"}}", graphURL)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", "sleep 5").Output()
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Channel: channel-a (available channels: channel-a, channel-b)"))

		g.By("Specify multiple channels")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "channel-a", "channel-b").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("error: multiple positional arguments given\nSee 'oc adm upgrade channel -h' for help and examples"))

		g.By("Set a channel which is same as the current channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "channel-a").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("info: Cluster is already in channel-a (no change)"))

		g.By("Clear a known channel which is in the available channels without --allow-explicit-channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("error: You are requesting to clear the update channel. The current channel \"channel-a\" is one of the available channels, you must pass --allow-explicit-channel to continue"))

		g.By("Clear a known channel which is in the available channels with --allow-explicit-channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "--allow-explicit-channel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("warning: Clearing channel \"channel-a\"; cluster will no longer request available update recommendations."))
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("NoChannel"))

		g.By("Re-clear the channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("info: Cluster channel is already clear (no change)"))

		g.By("Set to an unknown channel when the available channels are not present without --allow-explicit-channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "channel-d").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", "sleep 5").Output()
		o.Expect(cmdOut).To(o.ContainSubstring(fmt.Sprintf("warning: No channels known to be compatible with the current version \"%s\"; unable to validate \"channel-d\". Setting the update channel to \"channel-d\" anyway.", version)))
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Channel: channel-d (available channels: channel-a, channel-b)"))

		g.By("Set to an unknown channel which is not in the available channels without --allow-explicit-channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "channel-f").Output()
		o.Expect(err).To(o.HaveOccurred())
		exec.Command("bash", "-c", "sleep 5").Output()
		o.Expect(cmdOut).To(o.ContainSubstring("error: the requested channel \"channel-f\" is not one of the available channels (channel-a, channel-b), you must pass --allow-explicit-channel to continue"))

		g.By("Set to an unknown channel which is not in the available channels with --allow-explicit-channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "channel-f", "--allow-explicit-channel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", "sleep 5").Output()
		o.Expect(cmdOut).To(o.ContainSubstring("warning: The requested channel \"channel-f\" is not one of the available channels (channel-a, channel-b). You have used --allow-explicit-channel to proceed anyway. Setting the update channel to \"channel-f\"."))
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Channel: channel-f (available channels: channel-a, channel-b)"))

		g.By("Clear an unknown channel which is not in the available channels without --allow-explicit-channel")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("warning: Clearing channel \"channel-f\"; cluster will no longer request available update recommendations."))
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("NoChannel"))

		g.By("Set to a known channel when the available channels are not present")
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "channel-a").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", "sleep 5").Output()
		o.Expect(cmdOut).To(o.ContainSubstring(fmt.Sprintf("warning: No channels known to be compatible with the current version \"%s\"; unable to validate \"channel-a\". Setting the update channel to \"channel-a\" anyway.", version)))
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Channel: channel-a (available channels: channel-a, channel-b)"))

		g.By("Set to a known channel without --allow-explicit-channel")
		_, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "channel-b").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", "sleep 5").Output()
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Channel: channel-b (available channels: channel-a, channel-b)"))
	})
})
