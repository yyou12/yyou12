package image_registry

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"time"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()

	var (
		oc                 = exutil.NewCLI("default-image-prune", exutil.KubeConfigPath())
		logInfo            = "Only API objects will be removed.  No modifications to the image registry will be made"
		warnInfo           = "batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob"
		monitoringns       = "openshift-monitoring"
		promPod            = "prometheus-k8s-0"
		queryImagePruner   = "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=image_registry_operator_image_pruner_install_status"
		queryImageRegistry = "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=image_registry_operator_storage_reconfigured_total"
		priorityClassName  = "system-cluster-critical"
		normalInfo         = "Creating image pruner with keepYoungerThan"
		debugInfo          = "Examining ImageStream"
		traceInfo          = "keeping because it is used by imagestreams"
		traceAllInfo       = "Content-Type: application/json"
		tolerationsInfo    = `[{"effect":"NoSchedule","key":"key","operator":"Equal","value":"value"}]`
	)
	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-35906-Only API objects will be removed in image pruner pod when image registry is set to Removed [Disruptive]", func() {
		g.By("Set image registry cluster Removed")
		err := oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Removed"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Set image registry cluster Managed")
			oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Execute()
			err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
				podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
				o.Expect(err).NotTo(o.HaveOccurred())
				if len(podList.Items) == 2 {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "Image registry pods list is not 2")
		}()
		err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
			podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(podList.Items) == 0 {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pods are not removed")
		g.By("Set imagepruner cronjob started every 2 minutes")
		err = oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"schedule":"*/2 * * * *"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"schedule":""}}`, "--type=merge").Execute()
		time.Sleep(2 * time.Minute)
		podsOfImagePrune := []corev1.Pod{}
		podsOfImagePrune = ListPodStartingWith("image-pruner", oc, "openshift-image-registry")
		if len(podsOfImagePrune) == 0 {
			e2e.Failf("Error retrieving logs")
		}
		g.By("Check the log of image pruner and expected info about:Only API objects will be removed")
		foundImagePruneLog := false
		foundImagePruneLog = DePodLogs(podsOfImagePrune, oc, logInfo)
		o.Expect(foundImagePruneLog).To(o.BeTrue())
		foundWarnPruneLog := true
		foundWarnPruneLog = DePodLogs(podsOfImagePrune, oc, warnInfo)
		o.Expect(!foundWarnPruneLog).To(o.BeTrue())
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-High-27613-registry operator can publish metrics reporting the status of image-pruner [Disruptive]", func() {
		g.By("granting the cluster-admin role to user")
		oc.SetupProject()
		_, err := oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-admin", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("adm").Args("policy", "remove-cluster-role-from-user", "cluster-admin", oc.Username()).Execute()
		_, err = oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-monitoring-view", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("adm").Args("policy", "remove-cluster-role-from-user", "cluster-monitoring-view", oc.Username()).Execute()
		g.By("Get prometheus token")
		token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("-n", "openshift-monitoring", "get-token", "prometheus-k8s").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Prometheus query results report image pruner installed")
		foundValue := metricReportStatus(queryImagePruner, monitoringns, promPod, token, 2)
		o.Expect(foundValue).To(o.BeTrue())
		g.By("Prometheus query results report image registry operator not reconfiged")
		foundValue = metricReportStatus(queryImageRegistry, monitoringns, promPod, token, 0)
		o.Expect(foundValue).To(o.BeTrue())

		g.By("Set imagepruner suspend")
		err = oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"suspend":true}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"suspend":false}}`, "--type=merge").Execute()
		g.By("Prometheus query results report image registry operator not reconfiged")
		foundValue = metricReportStatus(queryImageRegistry, monitoringns, promPod, token, 0)
		o.Expect(foundValue).To(o.BeTrue())
		g.By("Prometheus query results report image pruner not installed")
		err = wait.PollImmediate(30*time.Second, 1*time.Minute, func() (bool, error) {
			foundValue = metricReportStatus(queryImagePruner, monitoringns, promPod, token, 1)
			if foundValue != true {
				e2e.Logf("wait for next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Don't find the value")
		o.Expect(foundValue).To(o.BeTrue())
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Low-43717-Add necessary priority class to pruner", func() {
		g.By("Check priority class of pruner")
		out := getResource(oc, asAdmin, withoutNamespace, "cronjob.batch", "-n", "openshift-image-registry", "-o=jsonpath={.items[0].spec.jobTemplate.spec.template.spec.priorityClassName}")
		o.Expect(out).To(o.ContainSubstring(priorityClassName))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-35292-LogLevel setting for the pruner", func() {
		g.By("Set imagepruner cronjob started every 1 minutes")
		err := oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"schedule":"*/1 * * * *"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"schedule":""}}`, "--type=merge").Execute()

		g.By("Check log when imagerpruner loglevel is Normal")
		time.Sleep(1 * time.Minute)
		foundPruneLog := false
		foundPruneLog = imagePruneLog(oc, normalInfo)
		o.Expect(foundPruneLog).To(o.BeTrue())

		g.By("Check log when imagerpruner loglevel is Debug")
		err = oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"logLevel":"Debug"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"logLevel":"Normal"}}`, "--type=merge").Execute()
		time.Sleep(1 * time.Minute)
		foundPruneLog = false
		foundPruneLog = imagePruneLog(oc, debugInfo)
		o.Expect(foundPruneLog).To(o.BeTrue())

		g.By("Check log when imagerpruner loglevel is Trace")
		err = oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"logLevel":"Trace"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(1 * time.Minute)
		foundPruneLog = false
		foundPruneLog = imagePruneLog(oc, traceInfo)
		o.Expect(foundPruneLog).To(o.BeTrue())

		g.By("Check log when imagerpruner loglevel is TraceAll")
		err = oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"logLevel":"TraceAll"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(1 * time.Minute)
		foundPruneLog = false
		foundPruneLog = imagePruneLog(oc, traceAllInfo)
		o.Expect(foundPruneLog).To(o.BeTrue())
	})
	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-44113-Image pruner should use custom tolerations", func() {
		g.By("Set tolerations for imagepruner cluster")
		err := oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"tolerations":[{"effect":"NoSchedule","key":"key","operator":"Equal","value":"value"}]}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("patch").Args("imagepruner/cluster", "-p", `{"spec":{"tolerations":null}}`, "--type=merge").Execute()
		g.By("Check image pruner cron job uses these tolerations")
		out := getResource(oc, asAdmin, withoutNamespace, "cronjob/image-pruner", "-n", "openshift-image-registry", "-o=jsonpath={.spec.jobTemplate.spec.template.spec.tolerations}")
		o.Expect(out).Should(o.Equal(tolerationsInfo))
	})
})
