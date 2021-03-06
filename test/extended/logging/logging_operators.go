package logging

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-openshift-logging] Logging NonPreRelease cluster-logging-operator should", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLI("logging-clo", exutil.KubeConfigPath())
		cloNS             = "openshift-logging"
		eoNS              = "openshift-operators-redhat"
		subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
		SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
		AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
	)

	CLO := SubscriptionObjects{"cluster-logging-operator", cloNS, SingleNamespaceOG, subTemplate, "cluster-logging", CatalogSourceObjects{}}
	EO := SubscriptionObjects{"elasticsearch-operator", eoNS, AllNamespaceOG, subTemplate, "elasticsearch-operator", CatalogSourceObjects{}}

	g.BeforeEach(func() {
		g.By("deploy CLO and EO")
		CLO.SubscribeLoggingOperators(oc)
		EO.SubscribeLoggingOperators(oc)
	})

	// author qitang@redhat.com
	g.It("CPaasrunOnly-Author:qitang-Medium-42405-No configurations when forward to external ES with only username or password set in pipeline secret[Serial]", func() {
		g.By("create secret in openshift-logging namespace")
		s := resource{"secret", "pipelinesecret", cloNS}
		defer s.clear(oc)
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args(s.kind, "-n", s.namespace, "generic", s.name, "--from-literal=username=test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create CLF")
		clf := resource{"clusterlogforwarder", "instance", cloNS}
		defer clf.clear(oc)
		clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "42405.yaml")
		err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deploy EFK pods")
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
		cl := resource{"clusterlogging", "instance", cloNS}
		defer cl.deleteClusterLogging(oc)
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
		WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

		g.By("extract configmap/collector, and check if it is empty")
		baseDir := exutil.FixturePath("testdata", "logging")
		TestDataPath := filepath.Join(baseDir, "temp")
		defer exec.Command("rm", "-r", TestDataPath).Output()
		err = os.MkdirAll(TestDataPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("-n", cloNS, "cm/collector", "--confirm", "--to="+TestDataPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		file_stat, err := os.Stat(filepath.Join(TestDataPath, "fluent.conf"))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(file_stat.Size() == 0).To(o.BeTrue())
	})

	// author qitang@redhat.com
	g.It("CPaasrunOnly-Author:qitang-Medium-41069-gather cert generation status in openshift event[Serial]", func() {
		g.By("deploy EFK pods")
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
		cl := resource{"clusterlogging", "instance", cloNS}
		defer cl.deleteClusterLogging(oc)
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
		WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

		g.By("Make CLO regenrate certs")
		master_certs := resource{"secret", "master-certs", cloNS}
		defer oc.AsAdmin().WithoutNamespace().Run("scale").Args("deploy/cluster-logging-operator", "--replicas=1", "-n", cloNS).Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("deploy/cluster-logging-operator", "--replicas=0", "-n", cloNS).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret/master-certs", "-n", cloNS).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		master_certs.WaitUntilResourceIsGone(oc)
		err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("deploy/cluster-logging-operator", "--replicas=1", "-n", cloNS).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		master_certs.WaitForResourceToAppear(oc)

		g.By("check events")
		events, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", cloNS, "--field-selector", "involvedObject.kind=ClusterLogging").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(events).Should(o.ContainSubstring("reason FileMissing type Regenerate"))
		o.Expect(events).Should(o.ContainSubstring("reason ExpiredOrMissing type Regenerate"))
	})

})

var _ = g.Describe("[sig-openshift-logging] Logging NonPreRelease elasticsearch-operator should", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLI("logging-eo", exutil.KubeConfigPath())
		cloNS             = "openshift-logging"
		eoNS              = "openshift-operators-redhat"
		subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
		SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
		AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
	)

	CLO := SubscriptionObjects{"cluster-logging-operator", cloNS, SingleNamespaceOG, subTemplate, "cluster-logging", CatalogSourceObjects{}}
	EO := SubscriptionObjects{"elasticsearch-operator", eoNS, AllNamespaceOG, subTemplate, "elasticsearch-operator", CatalogSourceObjects{}}

	g.BeforeEach(func() {
		g.By("deploy CLO and EO")
		CLO.SubscribeLoggingOperators(oc)
		EO.SubscribeLoggingOperators(oc)
	})

	// author qitang@redhat.com
	g.It("CPaasrunOnly-Author:qitang-High-41659-release locks on indices when disk utilization falls below flood watermark threshold[Serial][Slow]", func() {
		g.By("deploy EFK pods")
		sc, err := getStorageClassName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-storage-template.yaml")
		cl := resource{"clusterlogging", "instance", cloNS}
		defer cl.deleteClusterLogging(oc)
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "PVC_SIZE=5Gi")
		WaitForEFKPodsToBeReady(oc, cloNS)

		g.By("make ES disk usage > 95%")
		podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
		o.Expect(err).NotTo(o.HaveOccurred())
		create_file := "dd if=/dev/urandom of=/elasticsearch/persistent/file.txt bs=1048576 count=4700"
		_, _ = e2e.RunHostCmd(cloNS, podList.Items[0].Name, create_file)
		check_disk_usage := "es_util --query=_cat/nodes?h=h,disk.used_percent"
		stdout, err := e2e.RunHostCmdWithRetries(cloNS, podList.Items[0].Name, check_disk_usage, 3*time.Second, 30*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		disk_usage_1, _ := strconv.ParseFloat(strings.TrimSuffix(stdout, "\n"), 32)
		fmt.Printf("\n\ndisk usage is: %f\n\n", disk_usage_1)
		o.Expect(big.NewFloat(disk_usage_1).Cmp(big.NewFloat(95)) > 0).Should(o.BeTrue())

		g.By("check indices settings, should have \"index.blocks.read_only_allow_delete\": \"true\"")
		indices_settings := "es_util --query=app*/_settings/index.blocks.read_only_allow_delete?pretty"
		err = wait.Poll(5*time.Second, 120*time.Second, func() (done bool, err error) {
			output, err := e2e.RunHostCmdWithRetries(cloNS, podList.Items[0].Name, indices_settings, 3*time.Second, 30*time.Second)
			if err != nil {
				return false, err
			} else {
				if strings.Contains(output, "read_only_allow_delete") {
					return true, nil
				} else {
					return false, nil
				}
			}
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The EO doesn't add %s to index setting", "index.blocks.read_only_allow_delete"))

		g.By("release ES node disk")
		remove_file := "rm -rf /elasticsearch/persistent/file.txt"
		_, err = e2e.RunHostCmdWithRetries(cloNS, podList.Items[0].Name, remove_file, 3*time.Second, 30*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		stdout2, err := e2e.RunHostCmdWithRetries(cloNS, podList.Items[0].Name, check_disk_usage, 3*time.Second, 30*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		disk_usage_2, _ := strconv.ParseFloat(strings.TrimSuffix(stdout2, "\n"), 32)
		fmt.Printf("\n\ndisk usage is: %f\n\n", disk_usage_2)
		o.Expect(big.NewFloat(disk_usage_2).Cmp(big.NewFloat(95)) <= 0).Should(o.BeTrue())

		g.By("check indices settings again, should not have \"index.blocks.read_only_allow_delete\": \"true\"")
		err = wait.Poll(5*time.Second, 120*time.Second, func() (done bool, err error) {
			output, err := e2e.RunHostCmdWithRetries(cloNS, podList.Items[0].Name, indices_settings, 3*time.Second, 30*time.Second)
			if err != nil {
				return false, err
			} else {
				if strings.Contains(output, "read_only_allow_delete") {
					return false, nil
				} else {
					return true, nil
				}
			}
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The EO doesn't remove %s from index setting", "index.blocks.read_only_allow_delete"))
	})

	// author qitang@redhat.com
	g.It("CPaasrunOnly-Author:qitang-Medium-48657-Take redundancyPolicy into consideration when scale down ES nodes[Serial][Slow]", func() {
		g.By("deploy EFK pods")
		sc, err := getStorageClassName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-storage-template.yaml")
		cl := resource{"clusterlogging", "instance", cloNS}
		defer cl.deleteClusterLogging(oc)
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "PVC_SIZE=5Gi", "-p", "ES_NODE_COUNT=5", "-p", "REDUNDANCY_POLICY=ZeroRedundancy")
		WaitForEFKPodsToBeReady(oc, cloNS)

		g.By("scale down ES nodes when redundancy podlicy is ZeroRedundancy")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", cloNS, "cl/instance", "-p", "{\"spec\": {\"logStore\": {\"elasticsearch\": {\"nodeCount\": 4}}}}", "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		message := "Data node scale down rate is too high based on minimum number of replicas for all indices"

		g.By("check ES status")
		checkResource(oc, true, false, message, []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", cloNS, "-ojsonpath={.status.conditions}"})
		checkResource(oc, true, true, "green", []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", cloNS, "-ojsonpath={.status.cluster.status}"})
		esPods_1, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-data=true"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(esPods_1.Items) == 5).To(o.BeTrue())

		g.By("update redundancy policy to SingleRedundancy")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", cloNS, "cl/instance", "-p", "{\"spec\": {\"logStore\": {\"elasticsearch\": {\"redundancyPolicy\": \"SingleRedundancy\"}}}}", "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check ES status, no pod removed")
		checkResource(oc, true, false, message, []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", cloNS, "-ojsonpath={.status.conditions}"})
		esPods_2, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-data=true"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(esPods_2.Items) == 5).To(o.BeTrue())

		g.By("update index settings, change number_of_replicas to 1")
		masterPods, _ := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
		for _, index := range []string{"app", "infra", "audit"} {
			cmd := "es_util --query=" + index + "*/_settings?pretty -XPUT -d'{\"index\": {\"number_of_replicas\": 1}}'"
			_, err = e2e.RunHostCmd(cloNS, masterPods.Items[0].Name, cmd)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("check ES status, should have one pod removed")
		err = wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
			esPods, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-data=true"})
			if err != nil {
				return false, err
			} else {
				if len(esPods.Items) == 4 {
					return true, nil
				}
				return false, nil
			}
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ES pod count is not %d", 4))
		checkResource(oc, true, true, "green", []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", cloNS, "-ojsonpath={.status.cluster.status}"})

		g.By("reduce ES nodeCount to 2")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", cloNS, "cl/instance", "-p", "{\"spec\": {\"logStore\": {\"elasticsearch\": {\"nodeCount\": 2}}}}", "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check ES status, no pod removed")
		checkResource(oc, true, false, message, []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", cloNS, "-ojsonpath={.status.conditions}"})
		esPods_3, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-data=true"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(esPods_3.Items) == 4).To(o.BeTrue())
		checkResource(oc, true, true, "green", []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", cloNS, "-ojsonpath={.status.cluster.status}"})
	})

	// author qitang@redhat.com
	g.It("CPaasrunOnly-Author:qitang-Medium-46775-Index management jobs delete logs by namespace name and namespace prefix[Serial][Slow]", func() {
		logFile := exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
		//create some projects with different prefix, then create pod to generate logs
		g.By("create pod to generate logs")
		oc.SetupProject()
		proj_1 := oc.Namespace()
		err := oc.WithoutNamespace().Run("new-app").Args("-n", proj_1, "-f", logFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		oc.SetupProject()
		proj_2 := oc.Namespace()
		err = oc.WithoutNamespace().Run("new-app").Args("-n", proj_2, "-f", logFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		proj_3 := "logging-46775-1-" + getRandomString()
		defer oc.WithoutNamespace().Run("delete").Args("project", proj_3).Execute()
		err = oc.WithoutNamespace().Run("new-project").Args(proj_3).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("new-app").Args("-n", proj_3, "-f", logFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		proj_4 := "logging-46775-2-" + getRandomString()
		defer oc.WithoutNamespace().Run("delete").Args("project", proj_4).Execute()
		err = oc.WithoutNamespace().Run("new-project").Args(proj_4).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("new-app").Args("-n", proj_4, "-f", logFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deploy logging pods, enable delete by query")
		cl := resource{"clusterlogging", "instance", cloNS}
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-delete-by-query.yaml")
		sc, err := getStorageClassName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cl.deleteClusterLogging(oc)
		appNamespaceSpec := []PruneNamespace{{Namespace: proj_1, MinAge: "3m"}, {Namespace: "logging-46775-", MinAge: "3m"}}
		out, err := json.Marshal(appNamespaceSpec)
		o.Expect(err).NotTo(o.HaveOccurred())
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "APP_NAMESPACE_SPEC="+string(out))
		WaitForEFKPodsToBeReady(oc, cloNS)
		WaitForIMCronJobToAppear(oc, cloNS, "elasticsearch-im-prune-app")
		masterPods, _ := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
		projects := []string{proj_1, proj_2, proj_3, proj_4}
		for _, proj := range projects {
			waitForProjectLogsAppear(oc, cloNS, masterPods.Items[0].Name, proj, "app-00")
		}

		// make sure there have enough data in ES
		// if there doesn't have any data collected 3 minutes ago, then the following checking steps don't make sense
		// to wait for 3 minutes as the minAge is 3 minutes
		time.Sleep(180 * time.Second)
		g.By("wait for cronjob elasticsearch-im-prune-app and elasticsearch-im-prune-infra to complete")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("job", "-n", cloNS, "--all").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForIMJobsToComplete(oc, cloNS, 360*time.Second)

		//TODO: using xxxx-xx-xxTxx:xx:xx as the paremeter in time range is better than now-4m/m, the key point is how to get the job's schedule time
		//Ref: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html
		g.By("check if the logs are removed correctlly")
		// for proj_1, logs collected 3 minutes ago should be removed, here check doc count collected 4 minutes ago as it takes some time for the jobs to complete
		query_1 := "{\"query\": {\"bool\": {\"must\": [{\"match_phrase\": {\"kubernetes.namespace_name\": \"" + proj_1 + "\"}},{\"range\": {\"@timestamp\": {\"lte\": \"now-4m/m\"}}}]}}}"
		count_1, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", query_1)
		o.Expect(count_1 == 0).Should(o.BeTrue())
		o.Expect(err).NotTo(o.HaveOccurred())

		// for proj_2, logs collected 3 minutes ago should not be removed
		query_2 := "{\"query\": {\"bool\": {\"must\": [{\"match_phrase\": {\"kubernetes.namespace_name\": \"" + proj_2 + "\"}},{\"range\": {\"@timestamp\": {\"lte\": \"now-4m/m\"}}}]}}}"
		count_2, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", query_2)
		o.Expect(count_2 > 0).Should(o.BeTrue())
		o.Expect(err).NotTo(o.HaveOccurred())

		// for proj_3 and proj_4, logs collected 3 minutes ago should be removed, this is to test the namespace prefix
		query_3 := "{\"query\": {\"bool\": {\"must\": [{\"regexp\": {\"kubernetes.namespace_name\": \"logging-46775@\"}},{\"range\": {\"@timestamp\": {\"lte\": \"now-4m/m\"}}}]}}}"
		count_3, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", query_3)
		o.Expect(count_3 == 0).Should(o.BeTrue())
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author qitang@redhat.com
	g.It("CPaasrunOnly-Author:qitang-Medium-46881-Index management jobs delete logs by namespaces per different minAge[Serial][Slow]", func() {
		logFile := exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
		g.By("create several projects to generate logs")
		oc.SetupProject()
		proj_1 := oc.Namespace()
		err := oc.WithoutNamespace().Run("new-app").Args("-n", proj_1, "-f", logFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		oc.SetupProject()
		proj_2 := oc.Namespace()
		err = oc.WithoutNamespace().Run("new-app").Args("-f", logFile, "-n", proj_2).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deploy logging pods, enable delete by query")
		cl := resource{"clusterlogging", "instance", cloNS}
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-delete-by-query.yaml")
		sc, err := getStorageClassName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cl.deleteClusterLogging(oc)
		appNamespaceSpec := []PruneNamespace{{Namespace: proj_1, MinAge: "3m"}, {Namespace: proj_2, MinAge: "6m"}}
		out, err := json.Marshal(appNamespaceSpec)
		o.Expect(err).NotTo(o.HaveOccurred())
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "APP_NAMESPACE_SPEC="+string(out), "-p", "PRUNE_INTERVAL=3m")
		WaitForEFKPodsToBeReady(oc, cloNS)
		WaitForIMCronJobToAppear(oc, cloNS, "elasticsearch-im-prune-app")
		masterPods, _ := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
		waitForProjectLogsAppear(oc, cloNS, masterPods.Items[0].Name, proj_1, "app-00")
		waitForProjectLogsAppear(oc, cloNS, masterPods.Items[0].Name, proj_2, "app-00")

		// make sure there have enough data in ES
		// if there doesn't have any data collected 3 minutes ago, then the following checking steps don't make sense
		// to wait for 3 minutes as the minAge is 3 minutes
		time.Sleep(180 * time.Second)
		g.By(fmt.Sprintf("remove pod in %s to stop generating logs", proj_1))
		rc_1 := resource{"ReplicationController", "logging-centos-logtest", proj_1}
		rc_1.clear(oc)

		g.By("wait for cronjob elasticsearch-im-prune-app to complete")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("job", "-n", cloNS, "--all").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForIMJobsToComplete(oc, cloNS, 240*time.Second)

		//TODO: using xxxx-xx-xxTxx:xx:xx as the paremeter in time range is better than now-4m/m, the key point is how to get the job's schedule time
		//Ref: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html
		g.By("check current log count of each project")
		// for proj_1, logs collected 3 minutes ago should be removed, here check doc count collected 4 minutes ago as it takes some time for the jobs to complete
		// sometimes the count isn't 0 because the job is completed, but the data haven't been removed, so here need to wait for several seconds
		query_1 := "{\"query\": {\"bool\": {\"must\": [{\"match_phrase\": {\"kubernetes.namespace_name\": \"" + proj_1 + "\"}},{\"range\": {\"@timestamp\": {\"lte\": \"now-4m/m\"}}}]}}}"
		err = wait.Poll(3*time.Second, 45*time.Second, func() (done bool, err error) {
			count_1, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", query_1)
			if err != nil {
				return false, err
			} else {
				if count_1 == 0 {
					return true, nil
				} else {
					return false, nil
				}
			}
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("There still have some logs from %s", proj_1))
		// for proj_2, logs collected 3 minutes ago should not be removed
		count_2, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", "{\"query\": {\"bool\": {\"must\": [{\"match_phrase\": {\"kubernetes.namespace_name\": \""+proj_2+"\"}},{\"range\": {\"@timestamp\": {\"lte\": \"now-4m/m\"}}}]}}}")
		o.Expect(count_2 > 0).Should(o.BeTrue())
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for a new job to complete
		g.By("wait for cronjob elasticsearch-im-prune-app to complete")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("job", "-n", cloNS, "--all").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForIMJobsToComplete(oc, cloNS, 240*time.Second)

		g.By("check logs in ES again, for proj_1, no logs exist, for proj_2, logs collected 6 minutes ago should be removed")
		count_3, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", "{\"query\": {\"match_phrase\": {\"kubernetes.namespace_name\": \""+proj_1+"\"}}}")
		o.Expect(count_3 == 0).Should(o.BeTrue())
		o.Expect(err).NotTo(o.HaveOccurred())
		// for proj_2, logs collected 6 minutes ago should be removed, here check doc count collected 7 minutes ago as it takes some time for the jobs to complete
		count_4, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", "{\"query\": {\"bool\": {\"must\": [{\"match_phrase\": {\"kubernetes.namespace_name\": \""+proj_2+"\"}},{\"range\": {\"@timestamp\": {\"lte\": \"now-7m/m\"}}}]}}}")
		o.Expect(count_4 == 0).Should(o.BeTrue())
		o.Expect(err).NotTo(o.HaveOccurred())

		count_5, err := getDocCountByQuery(oc, cloNS, masterPods.Items[0].Name, "app", "{\"query\": {\"bool\": {\"must\": [{\"match_phrase\": {\"kubernetes.namespace_name\": \""+proj_2+"\"}},{\"range\": {\"@timestamp\": {\"gte\": \"now-7m/m\", \"lte\": \"now-4m/m\"}}}]}}}")
		o.Expect(count_5 > 0).Should(o.BeTrue())
		o.Expect(err).NotTo(o.HaveOccurred())
	})

})

var _ = g.Describe("[sig-openshift-logging] Logging NonPreRelease operators upgrade testing", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLI("logging-upgrade", exutil.KubeConfigPath())
		cloNS             = "openshift-logging"
		eoNS              = "openshift-operators-redhat"
		eo                = "elasticsearch-operator"
		clo               = "cluster-logging-operator"
		cloPackageName    = "cluster-logging"
		eoPackageName     = "elasticsearch-operator"
		subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
		SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
		AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
	)

	g.BeforeEach(func() {
		CLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, CatalogSourceObjects{}}
		EO := SubscriptionObjects{eo, eoNS, AllNamespaceOG, subTemplate, eoPackageName, CatalogSourceObjects{}}
		g.By("uninstall CLO and EO")
		CLO.uninstallLoggingOperator(oc)
		EO.uninstallLoggingOperator(oc)
	})

	// author: qitang@redhat.com
	g.It("Longduration-CPaasrunOnly-Author:qitang-High-44983-Logging auto upgrade in minor version[Disruptive][Slow]", func() {
		var targetchannel = "stable"
		var oh OperatorHub
		g.By("check source/redhat-operators status in operatorhub")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("operatorhub/cluster", "-ojson").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		json.Unmarshal([]byte(output), &oh)
		var disabled bool
		for _, source := range oh.Status.Sources {
			if source.Name == "redhat-operators" {
				disabled = source.Disabled
				break
			}
		}
		o.Expect(disabled).ShouldNot(o.BeTrue())
		g.By(fmt.Sprintf("Subscribe operators to %s channel", targetchannel))
		source := CatalogSourceObjects{targetchannel, "redhat-operators", "openshift-marketplace"}
		preCLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, source}
		preEO := SubscriptionObjects{eo, eoNS, AllNamespaceOG, subTemplate, eoPackageName, source}
		defer preCLO.uninstallLoggingOperator(oc)
		preCLO.SubscribeLoggingOperators(oc)
		defer preEO.uninstallLoggingOperator(oc)
		preEO.SubscribeLoggingOperators(oc)

		g.By("Deploy clusterlogging")
		sc, err := getStorageClassName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-storage-template.yaml")
		cl := resource{"clusterlogging", "instance", preCLO.Namespace}
		defer cl.deleteClusterLogging(oc)
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "ES_NODE_COUNT=3", "-p", "REDUNDANCY_POLICY=SingleRedundancy")
		WaitForEFKPodsToBeReady(oc, preCLO.Namespace)

		//get current csv version
		preCloCSV := preCLO.getInstalledCSV(oc)
		preEoCSV := preEO.getInstalledCSV(oc)

		//disable source/redhat-operators if it's not disabled
		if !disabled {
			defer oc.AsAdmin().WithoutNamespace().Run("patch").Args("operatorhub/cluster", "-p", "{\"spec\": {\"sources\": [{\"name\": \"redhat-operators\", \"disabled\": false}]}}", "--type=merge").Execute()
			err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("operatorhub/cluster", "-p", "{\"spec\": {\"sources\": [{\"name\": \"redhat-operators\", \"disabled\": true}]}}", "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		// get currentCSV in packagemanifests
		currentCloCSV := getCurrentCSVFromPackage(oc, targetchannel, cloPackageName)
		currentEoCSV := getCurrentCSVFromPackage(oc, targetchannel, eoPackageName)
		var upgraded = false
		//change source to qe-app-registry if needed, and wait for the new operators to be ready
		if preCloCSV != currentCloCSV {
			g.By(fmt.Sprintf("upgrade CLO to %s", currentCloCSV))
			err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", cloNS, "sub/"+preCLO.PackageName, "-p", "{\"spec\": {\"source\": \"qe-app-registry\"}}", "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			//add workaround for bz 2002276
			_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", "-n", "openshift-marketplace", "-l", "olm.catalogSource=qe-app-registry").Execute()
			checkResource(oc, true, true, currentCloCSV, []string{"sub", preCLO.PackageName, "-n", preCLO.Namespace, "-ojsonpath={.status.currentCSV}"})
			WaitForDeploymentPodsToBeReady(oc, preCLO.Namespace, preCLO.OperatorName)
			upgraded = true
		}
		if preEoCSV != currentEoCSV {
			g.By(fmt.Sprintf("upgrade EO to %s", currentEoCSV))
			err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", eoNS, "sub/"+preEO.PackageName, "-p", "{\"spec\": {\"source\": \"qe-app-registry\"}}", "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			//add workaround for bz 2002276
			_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", "-n", "openshift-marketplace", "-l", "olm.catalogSource=qe-app-registry").Execute()
			checkResource(oc, true, true, currentEoCSV, []string{"sub", preEO.PackageName, "-n", preEO.Namespace, "-ojsonpath={.status.currentCSV}"})
			WaitForDeploymentPodsToBeReady(oc, preEO.Namespace, preEO.OperatorName)
			upgraded = true
		}

		if upgraded {
			g.By("waiting for the EFK pods to be ready after upgrade")
			WaitForEFKPodsToBeReady(oc, cloNS)
			checkResource(oc, true, true, "green", []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", preCLO.Namespace, "-ojsonpath={.status.cluster.status}"})
			//check PVC count, it should be equal to ES node count
			pvc, _ := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(cloNS).List(metav1.ListOptions{LabelSelector: "logging-cluster=elasticsearch"})
			o.Expect(len(pvc.Items) == 3).To(o.BeTrue())

			g.By("checking if the collector can collect logs after upgrading")
			oc.SetupProject()
			app_proj := oc.Namespace()
			jsonLogFile := exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
			err = oc.WithoutNamespace().Run("new-app").Args("-n", app_proj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			prePodList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForProjectLogsAppear(oc, cloNS, prePodList.Items[0].Name, app_proj, "app-00")
		}
	})

	// author: qitang@redhat.com
	g.It("Longduration-CPaasrunOnly-Author:qitang-Medium-40508-upgrade from prior version to current version[Serial][Slow]", func() {
		// to add logging 5.3, create a new catalog source with image: quay.io/openshift-qe-optional-operators/ocp4-index:latest
		catsrcTemplate := exutil.FixturePath("testdata", "logging", "subscription", "catsrc.yaml")
		catsrc := resource{"catsrc", "logging-upgrade-" + getRandomString(), "openshift-marketplace"}
		defer catsrc.clear(oc)
		catsrc.applyFromTemplate(oc, "-f", catsrcTemplate, "-n", catsrc.namespace, "-p", "NAME="+catsrc.name, "-p", "IMAGE=quay.io/openshift-qe-optional-operators/ocp4-index:latest")
		waitForPodReadyWithLabel(oc, catsrc.namespace, "olm.catalogSource="+catsrc.name)

		// for 5.4, test upgrade from 5.3 to 5.4
		preSource := CatalogSourceObjects{"stable-5.3", catsrc.name, catsrc.namespace}
		g.By(fmt.Sprintf("Subscribe operators to %s channel", preSource.Channel))
		preCLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, preSource}
		preEO := SubscriptionObjects{eo, eoNS, AllNamespaceOG, subTemplate, eoPackageName, preSource}
		defer preCLO.uninstallLoggingOperator(oc)
		preCLO.SubscribeLoggingOperators(oc)
		defer preEO.uninstallLoggingOperator(oc)
		preEO.SubscribeLoggingOperators(oc)

		g.By("Deploy clusterlogging")
		sc, err := getStorageClassName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-storage-template.yaml")
		cl := resource{"clusterlogging", "instance", preCLO.Namespace}
		defer cl.deleteClusterLogging(oc)
		cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "-p", "STORAGE_CLASS="+sc, "-p", "ES_NODE_COUNT=3", "-p", "REDUNDANCY_POLICY=SingleRedundancy")
		WaitForEFKPodsToBeReady(oc, preCLO.Namespace)

		//change channel, and wait for the new operators to be ready
		var source = CatalogSourceObjects{"stable-5.4", "qe-app-registry", "openshift-marketplace"}
		//change channel, and wait for the new operators to be ready
		version := strings.Split(source.Channel, "-")[1]
		g.By(fmt.Sprintf("upgrade CLO&EO to %s", source.Channel))
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", cloNS, "sub/"+preCLO.PackageName, "-p", "{\"spec\": {\"channel\": \""+source.Channel+"\", \"source\": \""+source.SourceName+"\", \"sourceNamespace\": \""+source.SourceNamespace+"\"}}", "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", eoNS, "sub/"+preEO.PackageName, "-p", "{\"spec\": {\"channel\": \""+source.Channel+"\", \"source\": \""+source.SourceName+"\", \"sourceNamespace\": \""+source.SourceNamespace+"\"}}", "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		//add workaround for bz 2002276
		_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", "-n", "openshift-marketplace", "-l", "olm.catalogSource="+source.SourceName).Execute()

		checkResource(oc, true, false, version, []string{"sub", preCLO.PackageName, "-n", preCLO.Namespace, "-ojsonpath={.status.currentCSV}"})
		cloCurrentCSV, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "-n", preCLO.Namespace, preCLO.PackageName, "-ojsonpath={.status.currentCSV}").Output()
		resource{"csv", cloCurrentCSV, preCLO.Namespace}.WaitForResourceToAppear(oc)
		checkResource(oc, true, true, "Succeeded", []string{"csv", cloCurrentCSV, "-n", preCLO.Namespace, "-ojsonpath={.status.phase}"})
		WaitForDeploymentPodsToBeReady(oc, preCLO.Namespace, preCLO.OperatorName)

		checkResource(oc, true, false, version, []string{"sub", preEO.PackageName, "-n", preEO.Namespace, "-ojsonpath={.status.currentCSV}"})
		eoCurrentCSV, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "-n", preEO.Namespace, preEO.PackageName, "-ojsonpath={.status.currentCSV}").Output()
		resource{"csv", eoCurrentCSV, preEO.Namespace}.WaitForResourceToAppear(oc)
		checkResource(oc, true, true, "Succeeded", []string{"csv", eoCurrentCSV, "-n", preEO.Namespace, "-ojsonpath={.status.phase}"})
		WaitForDeploymentPodsToBeReady(oc, preEO.Namespace, preEO.OperatorName)

		g.By("waiting for the EFK pods to be ready after upgrade")
		WaitForEFKPodsToBeReady(oc, cloNS)
		checkResource(oc, true, true, "green", []string{"elasticsearches.logging.openshift.io", "elasticsearch", "-n", preCLO.Namespace, "-ojsonpath={.status.cluster.status}"})

		//check PVC count, it should be equal to ES node count
		pvc, _ := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(cloNS).List(metav1.ListOptions{LabelSelector: "logging-cluster=elasticsearch"})
		o.Expect(len(pvc.Items) == 3).To(o.BeTrue())

		g.By("checking if the collector can collect logs after upgrading")
		oc.SetupProject()
		app_proj := oc.Namespace()
		jsonLogFile := exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
		err = oc.WithoutNamespace().Run("new-app").Args("-n", app_proj, "-f", jsonLogFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		prePodList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForProjectLogsAppear(oc, cloNS, prePodList.Items[0].Name, app_proj, "app-00")
	})
})
