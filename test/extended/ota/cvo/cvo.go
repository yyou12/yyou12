package cvo

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
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
	g.It("Longduration-NonPreRelease-Author:yanyang-Medium-32138-cvo alert should not be fired when RetrievedUpdates failed due to nochannel [Serial][Slow]", func() {
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

		defer restoreCVSpec(orgUpstream, orgChannel, oc)

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

	//author: yanyang@redhat.com
	g.It("Author:yanyang-High-42543-the removed resources are not created in a fresh installed cluster", func() {
		manifestDir := fmt.Sprintf("manifest-%d", time.Now().Unix())
		err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("release", "extract", "--to", manifestDir).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, _ := exec.Command("bash", "-c", fmt.Sprintf("grep -rl \"name: hello-openshift\" %s", manifestDir)).Output()
		o.Expect(string(out)).NotTo(o.BeEmpty())
		file := strings.TrimSpace(string(out))
		cmd := fmt.Sprintf("grep -A5 'name: hello-openshift' %s | grep 'release.openshift.io/delete: \"true\"'", file)
		result, _ := exec.Command("bash", "-c", cmd).Output()
		o.Expect(string(result)).NotTo(o.BeEmpty())

		g.By("Check imagestream hello-openshift not present in a fresh installed cluster")
		cmdOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("imagestream", "hello-openshift", "-n", "openshift").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Error from server (NotFound): imagestreams.image.openshift.io \"hello-openshift\" not found"))
	})

	//author: yanyang@redhat.com
	g.It("ConnectedOnly-Author:yanyang-Medium-43172-get the upstream and channel info by using oc adm upgrade [Serial]", func() {
		orgUpstream, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].spec.upstream}").Output()
		orgChannel, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].spec.channel}").Output()

		defer restoreCVSpec(orgUpstream, orgChannel, oc)

		g.By("Check when upstream is unset")
		if orgUpstream != "" {
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterversion/version", "--type=json", "-p", "[{\"op\":\"remove\", \"path\":\"/spec/upstream\"}]").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		cmdOut, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring("Upstream is unset, so the cluster will use an appropriate default."))
		o.Expect(cmdOut).To(o.ContainSubstring(fmt.Sprintf("Channel: %s", orgChannel)))

		desiredChannel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion/version", "-o=jsonpath={.status.desired.channels}").Output()

		o.Expect(err).NotTo(o.HaveOccurred())
		if desiredChannel == "" {
			o.Expect(cmdOut).NotTo(o.ContainSubstring("available channels:"))
		} else {
			msg := "available channels: "
			desiredChannel = desiredChannel[1 : len(desiredChannel)-1]
			splits := strings.Split(desiredChannel, ",")
			for _, split := range splits {
				split = strings.Trim(split, "\"")
				msg = msg + split + ", "
			}
			msg = msg[:len(msg)-2]

			o.Expect(cmdOut).To(o.ContainSubstring(msg))
		}

		g.By("Check when upstream is set")
		projectID := "openshift-qe"
		ctx := context.Background()
		client, err := storage.NewClient(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer client.Close()

		graphURL, bucket, object, err := buildGraph(client, oc, projectID)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer DeleteBucket(client, bucket)
		defer DeleteObject(client, bucket, object)

		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterversion/version", "--type=merge", "--patch", fmt.Sprintf("{\"spec\":{\"upstream\":\"%s\", \"channel\":\"channel-a\"}}", graphURL)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", "sleep 5").Output()
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).To(o.ContainSubstring(fmt.Sprintf("Upstream: %s", graphURL)))
		o.Expect(cmdOut).To(o.ContainSubstring("Channel: channel-a (available channels: channel-a, channel-b)"))

		g.By("Check when channel is unset")
		_, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "--allow-explicit-channel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmdOut).NotTo(o.ContainSubstring("Upstream:"))
		o.Expect(cmdOut).NotTo(o.ContainSubstring("Channel:"))
		o.Expect(cmdOut).To(o.ContainSubstring("Reason: NoChannel"))
		o.Expect(cmdOut).To(o.ContainSubstring("Message: The update channel has not been configured."))
	})

	//author: jiajliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:jiajliu-Medium-41728-cvo alert ClusterOperatorDegraded on degraded operators [Disruptive][Slow]", func() {

		testDataDir := exutil.FixturePath("testdata", "ota/cvo")
		badOauthFile := filepath.Join(testDataDir, "bad-oauth.yaml")

		g.By("Get goodOauthFile from the initial oauth yaml file to oauth-41728.yaml")
		goodOauthFile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("oauth", "cluster", "-o", "yaml").OutputToFile("oauth-41728.yaml")
		defer exec.Command("bash", "-c", fmt.Sprintf("rm -rf %s", goodOauthFile))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Prune goodOauthFile")
		oauthfile, err := exec.Command("bash", "-c", fmt.Sprintf("sed -i \"/resourceVersion/d\" %s && cat %s", goodOauthFile, goodOauthFile)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oauthfile).NotTo(o.ContainSubstring("resourceVersion"))

		g.By("Enable ClusterOperatorDegraded alert")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", badOauthFile).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", goodOauthFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check ClusterOperatorDegraded condition...")
		err = waitForCondition(60, 300, "True", "oc get co authentication -ojson|jq -r '.status.conditions[]|select(.type==\"Degraded\").status'")
		exutil.AssertWaitPollNoErr(err, "authentication operator is not degraded in 5m")

		g.By("Check ClusterOperatorDown alert is not firing and ClusterOperatorDegraded alert is fired correctly.")
		err = wait.Poll(5*time.Minute, 30*time.Minute, func() (bool, error) {
			alertDown := getAlert("ClusterOperatorDown")
			alertDegraded := getAlert("ClusterOperatorDegraded")
			o.Expect(alertDown).To(o.BeNil())
			if alertDegraded == nil || alertDegraded["state"] != "firing" {
				e2e.Logf("Waiting for alert ClusterOperatorDegraded to be triggered and fired...")
				return false, nil
			}
			o.Expect(alertDegraded["labels"].(map[string]interface{})["severity"].(string)).To(o.Equal("warning"))
			o.Expect(alertDegraded["annotations"].(map[string]interface{})["summary"].(string)).To(o.ContainSubstring("Cluster operator has been degraded for 30 minutes."))
			o.Expect(alertDegraded["annotations"].(map[string]interface{})["description"].(string)).To(o.ContainSubstring("The authentication operator is degraded"))
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "ClusterOperatorDegraded alert is not fired in 30m")

		g.By("Disable ClusterOperatorDegraded alert")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", goodOauthFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check alert is disabled")
		err = wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
			alertDegraded := getAlert("ClusterOperatorDegraded")
			if alertDegraded != nil {
				e2e.Logf("Waiting for alert being disabled...")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "alert is not disabled.")
	})

	//author: jiajliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:jiajliu-Medium-41778-ClusterOperatorDown and ClusterOperatorDegradedon alerts when unset conditions [Slow]", func() {

		testDataDir := exutil.FixturePath("testdata", "ota/cvo")
		badOauthFile := filepath.Join(testDataDir, "co-test.yaml")

		g.By("Enable alerts")
		err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", badOauthFile).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("co", "test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check operator's condition...")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "test", "-o=jsonpath={.status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.Equal(""))

		g.By("Waiting for alerts triggered...")
		err = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			alertDown := getAlert("ClusterOperatorDown")
			alertDegraded := getAlert("ClusterOperatorDegraded")
			if alertDown == nil || alertDegraded == nil {
				e2e.Logf("Waiting for alerts to be triggered...")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "No alert triggerred!")

		g.By("Check alert ClusterOperatorDown fired.")
		err = wait.Poll(5*time.Minute, 10*time.Minute, func() (bool, error) {
			alertDown := getAlert("ClusterOperatorDown")
			if alertDown["state"] != "firing" {
				e2e.Logf("Waiting for alert ClusterOperatorDown to be triggered and fired...")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "ClusterOperatorDown alert is not fired in 10m")

		g.By("Check alert ClusterOperatorDegraded fired.")
		err = wait.Poll(5*time.Minute, 20*time.Minute, func() (bool, error) {
			alertDegraded := getAlert("ClusterOperatorDegraded")
			if alertDegraded["state"] != "firing" {
				e2e.Logf("Waiting for alert ClusterOperatorDegraded to be triggered and fired...")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "ClusterOperatorDegraded alert is not fired in 30m")

		g.By("Disable alerts")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("co", "test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check alerts are disabled...")
		err = wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
			alertDown := getAlert("ClusterOperatorDown")
			alertDegraded := getAlert("ClusterOperatorDegraded")
			if alertDown != nil || alertDegraded != nil {
				e2e.Logf("Waiting for alerts being disabled...")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "alerts are not disabled.")
	})

	//author: jiajliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:jiajliu-Medium-41736-cvo alert ClusterOperatorDown on unavailable operators [Disruptive][Slow]", func() {

		masterNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-authentication-operator", "-o=jsonpath={.items[].spec.nodeName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Enable ClusterOperatorDown alert")
		err = oc.AsAdmin().Run("label").Args("node", masterNode, "kubernetes.io/os-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("label").Args("node", masterNode, "kubernetes.io/os=linux").Execute()

		g.By("Check ClusterOperatorDown condition...")
		err = waitForCondition(60, 300, "False", "oc get co authentication -ojson|jq -r '.status.conditions[]|select(.type==\"Available\").status'")
		exutil.AssertWaitPollNoErr(err, "authentication operator is not down in 5m")

		g.By("Check ClusterOperatorDown alert is fired correctly")
		err = wait.Poll(100*time.Second, 600*time.Second, func() (bool, error) {
			alertDown := getAlert("ClusterOperatorDown")
			if alertDown == nil || alertDown["state"] != "firing" {
				e2e.Logf("Waiting for alert ClusterOperatorDown to be triggered and fired...")
				return false, nil
			}
			o.Expect(alertDown["labels"].(map[string]interface{})["severity"].(string)).To(o.Equal("critical"))
			o.Expect(alertDown["annotations"].(map[string]interface{})["summary"].(string)).To(o.ContainSubstring("Cluster operator has not been available for 10 minutes."))
			o.Expect(alertDown["annotations"].(map[string]interface{})["description"].(string)).To(o.ContainSubstring("The authentication operator may be down or disabled"))
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "ClusterOperatorDown alert is not fired in 10m")

		g.By("Disable ClusterOperatorDown alert")
		err = oc.AsAdmin().Run("label").Args("node", masterNode, "kubernetes.io/os=linux").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check alert is disabled")
		err = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			alertDown := getAlert("ClusterOperatorDown")
			if alertDown != nil {
				e2e.Logf("Waiting for alert being disabled...")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "alert is not disabled.")
	})
})
