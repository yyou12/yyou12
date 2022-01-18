package image_registry

import (
	"os"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("default-registry-upgrade", exutil.KubeConfigPath())
	)
	// author: wewang@redhat.com
	g.It("NonPreRelease-PreChkUpgrade-Author:wewang-High-26401-Upgrade cluster with insecureRegistries and blockedRegistries defined prepare [Disruptive]", func() {
		g.By("Add insecureRegistries and blockedRegistries to image.config")
		output, err := oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec":{"registrySources":{"blockedRegistries": ["untrusted.com"],"insecureRegistries": ["insecure.com"]}}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("registries.conf gets updated")
		workNode, _ := exutil.GetFirstWorkerNode(oc)
		err = wait.Poll(30*time.Second, 6*time.Minute, func() (bool, error) {
			registriesstatus, _ := exutil.DebugNodeWithChroot(oc, workNode, "bash", "-c", "cat /etc/containers/registries.conf |grep -E '\"untrusted.com\"|\"insecure.com\"'")
			if strings.Contains(registriesstatus, "location = \"untrusted.com\"") && strings.Contains(registriesstatus, "location = \"insecure.com\"") {
				e2e.Logf("registries.conf updated")
				return true, nil
			} else {
				e2e.Logf("registries.conf not update")
				return false, nil
			}
		})
		exutil.AssertWaitPollNoErr(err, "registries.conf not contains registrysources")
	})

	// author: wewang@redhat.com
	g.It("NonPreRelease-PstChkUpgrade-Author:wewang-High-26401-Upgrade cluster with insecureRegistries and blockedRegistries defined after upgrade [Disruptive]", func() {
		g.By("registries.conf gets updated")
		defer oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec": {"registrySources": null}}`, "--type=merge").Execute()
		workNode, _ := exutil.GetFirstWorkerNode(oc)
		registriesstatus, _ := exutil.DebugNodeWithChroot(oc, workNode, "bash", "-c", "cat /etc/containers/registries.conf | grep -E '\"untrusted.com\"|\"insecure.com\"'")
		if strings.Contains(registriesstatus, "location = \"untrusted.com\"") && strings.Contains(registriesstatus, "location = \"insecure.com\"") {
			e2e.Logf("registries.conf updated")
		} else {
			e2e.Failf("registries.conf not update")
		}
	})

	// author: wewang@redhat.com
	g.It("NonPreRelease-PreChkUpgrade-Author:wewang-High-41400-Users providing custom AWS tags are set with bucket creation prepare", func() {
		g.By("Check platforms")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Check the cluster is with resourceTags")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.aws}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "resourceTags") {
			g.Skip("Skip for no resourceTags")
		}
		g.By("Get bucket name")
		bucket, _ := oc.AsAdmin().Run("get").Args("config.image", "-o=jsonpath={..spec.storage.s3.bucket}").Output()

		g.By("Set AWS credentials")
		accessKeyId, secureKey := getCreditFromCluster(oc)

		g.By("Check the tags")
		awscmd := "aws s3api get-bucket-tagging --bucket "
		cmd := exec.Command("bash", "-c", awscmd+bucket)
		cmd.Env = append(os.Environ(), accessKeyId, secureKey)
		tag, err := cmd.Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(tag)).To(o.ContainSubstring("customTag"))
		o.Expect(string(tag)).To(o.ContainSubstring("installer-qe"))
	})

	// author: wewang@redhat.com
	g.It("NonPreRelease-PstChkUpgrade-Author:wewang-High-41400- Users providing custom AWS tags are set with bucket creation after upgrade", func() {
		g.By("Check platforms")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Check the cluster is with resourceTags")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.aws}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "resourceTags") {
			g.Skip("Skip for no resourceTags")
		}
		g.By("Get bucket name")
		bucket, _ := oc.AsAdmin().Run("get").Args("config.image", "-o=jsonpath={..spec.storage.s3.bucket}").Output()

		g.By("Set AWS credentials")
		accessKeyId, secureKey := getCreditFromCluster(oc)

		g.By("Check the tags")
		awscmd := "aws s3api get-bucket-tagging --bucket "
		cmd := exec.Command("bash", "-c", awscmd+bucket)
		cmd.Env = append(os.Environ(), accessKeyId, secureKey)
		tag, err := cmd.Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(tag)).To(o.ContainSubstring("customTag"))
		o.Expect(string(tag)).To(o.ContainSubstring("installer-qe"))
	})
})
