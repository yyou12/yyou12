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
		oc      = exutil.NewCLI("default-image-registry", exutil.KubeConfigPath())
		logInfo = "Only API objects will be removed.  No modifications to the image registry will be made"
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
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
			podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(podList.Items) == 0 {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
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
	})
})
