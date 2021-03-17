package opm

import (
	"fmt"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] OLM opm should", func() {
	defer g.GinkgoRecover()

	var opmCLI = NewOpmCLI()

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Medium-27620-Validate operator bundle Image and Contents", func() {

		bundleImages := []struct {
			image  string
			expect string
		}{
			{"quay.io/olmqe/etcd-bundle:0.9.4", "All validation tests have been completed successfully"},
			{"quay.io/olmqe/etcd-bundle:wrong", "Bundle validation errors"},
		}
		opmCLI.showInfo = true
		for _, b := range bundleImages {
			g.By(fmt.Sprintf("Validating the %s", b.image))
			output, err := opmCLI.Run("alpha").Args("bundle", "validate", "-b", "none", "-t", b.image).Output()

			if strings.Contains(output, b.expect) {
				e2e.Logf(fmt.Sprintf("That's expected! %s", b.image))
			} else {
				e2e.Failf(fmt.Sprintf("Failed to validating the %s, error: %v", b.image, err))
			}

		}

	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Medium-34016-opm can prune operators from catalog", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		indexDB := filepath.Join(opmBaseDir, "index_34016.db")
		output, err := opmCLI.Run("registry").Args("prune", "-d", indexDB, "-p", "lib-bucket-provisioner").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "deleting packages") || !strings.Contains(output, "pkg=planetscale") {
			e2e.Failf(fmt.Sprintf("Failed to obtain the removed packages from prune : %s", output))
		}
	})
})

var _ = g.Describe("[sig-operators] OLM opm with docker", func() {
	defer g.GinkgoRecover()

	var dockerCLI = container.NewDockerCLI()
	var podmanCLI = container.NewPodmanCLI()
	var opmCLI = NewOpmCLI()

	// author: xzha@redhat.com
	g.It("Author:xzha-VMonly-Medium-25955-opm Ability to generate scaffolding for Operator Bundle using docker", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPathDocker := filepath.Join(opmBaseDir, "learn_operator")
		opmCLI.execCommandPath = TestDataPathDocker
		defer DeleteDir(TestDataPathDocker, "fixture-testdata")
		imageTag := "quay.io/olmqe/25955-operator-" + getRandomString() + ":v0.0.1"
		_, err := dockerCLI.RemoveImage(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("test using docker")
		g.By("step: opm alpha bundle generate")
		output, err := opmCLI.Run("alpha").Args("bundle", "generate", "-d", "package/0.0.1", "-p", "25955-operator", "-c", "alpha", "-e", "alpha").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		if !strings.Contains(output, "Writing annotations.yaml") || !strings.Contains(output, "Writing bundle.Dockerfile") {
			e2e.Failf("Failed to execute opm alpha bundle generate : %s", output)
		}
		e2e.Logf("clean test data")
		DeleteDir(TestDataPathDocker, "fixture-testdata")

		g.By("step: opm alpha bundle build")
		opmBaseDir = exutil.FixturePath("testdata", "opm")
		TestDataPathDocker = filepath.Join(opmBaseDir, "learn_operator")
		opmCLI.execCommandPath = TestDataPathDocker

		defer dockerCLI.RemoveImage(imageTag)
		output, _ = opmCLI.Run("alpha").Args("bundle", "build", "-d", "package/0.0.1", "--tag", imageTag, "-p", "25955-operator", "-c", "alpha", "-e", "alpha", "--overwrite").Output()
		e2e.Logf(output)
		if !strings.Contains(output, "Building bundle image") {
			e2e.Failf("Failed to execute opm alpha bundle build : %s", output)
		}
		e2e.Logf("step: check image %s exist", imageTag)
		existFlag, err := dockerCLI.CheckImageExist(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("check image exist is %v", existFlag)
		o.Expect(existFlag).To(o.BeTrue())
		e2e.Logf("clean test data")
		DeleteDir(TestDataPathDocker, "fixture-testdata")

		g.By("step: test using podman")
		opmBaseDir = exutil.FixturePath("testdata", "opm")
		TestDataPathPodman := filepath.Join(opmBaseDir, "learn_operator")
		opmCLI.execCommandPath = TestDataPathPodman
		defer DeleteDir(TestDataPathPodman, "fixture-testdata")

		_, err = podmanCLI.RemoveImage(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: opm alpha bundle build")
		defer podmanCLI.RemoveImage(imageTag)
		output, _ = opmCLI.Run("alpha").Args("bundle", "build", "-d", "package/0.0.1", "-b", "podman", "--tag", imageTag, "-p", "25955-operator", "-c", "alpha", "-e", "alpha", "--overwrite").Output()
		e2e.Logf(output)
		if !strings.Contains(output, "COMMIT "+imageTag) {
			e2e.Failf("Failed to execute opm alpha bundle build : %s", output)
		}

		e2e.Logf("step: check image %s exist", imageTag)
		existFlag, err = podmanCLI.CheckImageExist(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("check image exist is %v", existFlag)
		o.Expect(existFlag).To(o.BeTrue())
	})
})
