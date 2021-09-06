package opm

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	db "github.com/openshift/openshift-tests-private/test/extended/util/db"
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

	// author: kuiwang@redhat.com
	g.It("Author:kuiwang-Medium-43185-DC based opm subcommands out of alpha", func() {
		g.By("check init, serve, render and validate under opm")
		output, err := opmCLI.Run("").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		o.Expect(output).To(o.ContainSubstring("init "))
		o.Expect(output).To(o.ContainSubstring("serve "))
		o.Expect(output).To(o.ContainSubstring("render "))
		o.Expect(output).To(o.ContainSubstring("validate "))

		g.By("check init, serve, render and validate not under opm alpha")
		output, err = opmCLI.Run("alpha").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		o.Expect(output).NotTo(o.ContainSubstring("init "))
		o.Expect(output).NotTo(o.ContainSubstring("serve "))
		o.Expect(output).NotTo(o.ContainSubstring("render "))
		o.Expect(output).NotTo(o.ContainSubstring("validate "))
	})

	// author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-43171-opm render blob from bundle, db based index, dc based index, db file and directory", func() {
		g.By("render db-based index image")
		output, err := opmCLI.Run("render").Args("quay.io/olmqe/olm-index:OLM-2199").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("\"image\": \"quay.io/olmqe/cockroachdb-operator:5.0.3-2199\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.3"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.4\""))
		o.Expect(output).To(o.ContainSubstring("\"replaces\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.4\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.5\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.5"))
		o.Expect(output).To(o.ContainSubstring("G9yX2NwdV9saW1pdCI6eyJkZWZhdWx0IjoiNCIsInR5cGUiO"))
		o.Expect(output).To(o.ContainSubstring("nRJkhFiQuzDX/kIs7oymi/znDqF/u01OSDLakLMhPHjGPLsG"))
		o.Expect(output).To(o.ContainSubstring("2WHFDbGZFbVkvSkFyQVdDRW5sanh1aTFvZUtzV083WnhteFF"))

		g.By("render dc-based index image with one file")
		output, err = opmCLI.Run("render").Args("quay.io/olmqe/olm-index:OLM-2199-DC-example").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("\"image\": \"quay.io/olmqe/cockroachdb-operator:5.0.3-2199\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.3"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.4\""))
		o.Expect(output).To(o.ContainSubstring("\"replaces\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.4\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.5\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.5"))
		o.Expect(output).To(o.ContainSubstring("G9yX2NwdV9saW1pdCI6eyJkZWZhdWx0IjoiNCIsInR5cGUiO"))
		o.Expect(output).To(o.ContainSubstring("nRJkhFiQuzDX/kIs7oymi/znDqF/u01OSDLakLMhPHjGPLsG"))
		o.Expect(output).To(o.ContainSubstring("2WHFDbGZFbVkvSkFyQVdDRW5sanh1aTFvZUtzV083WnhteFF"))

		g.By("render dc-based index image with different files")
		output, err = opmCLI.Run("render").Args("quay.io/olmqe/olm-index:OLM-2199-DC-example-Df").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("\"image\": \"quay.io/olmqe/cockroachdb-operator:5.0.3-2199\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.3"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.4\""))
		o.Expect(output).To(o.ContainSubstring("\"replaces\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.4\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.5\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.5"))
		o.Expect(output).To(o.ContainSubstring("G9yX2NwdV9saW1pdCI6eyJkZWZhdWx0IjoiNCIsInR5cGUiO"))
		o.Expect(output).To(o.ContainSubstring("nRJkhFiQuzDX/kIs7oymi/znDqF/u01OSDLakLMhPHjGPLsG"))
		o.Expect(output).To(o.ContainSubstring("2WHFDbGZFbVkvSkFyQVdDRW5sanh1aTFvZUtzV083WnhteFF"))

		g.By("render dc-based index image with different directory")
		output, err = opmCLI.Run("render").Args("quay.io/olmqe/olm-index:OLM-2199-DC-example-Dd").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("\"image\": \"quay.io/olmqe/cockroachdb-operator:5.0.3-2199\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.3"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.4\""))
		o.Expect(output).To(o.ContainSubstring("\"replaces\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.4\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.5\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.5"))
		o.Expect(output).To(o.ContainSubstring("G9yX2NwdV9saW1pdCI6eyJkZWZhdWx0IjoiNCIsInR5cGUiO"))
		o.Expect(output).To(o.ContainSubstring("nRJkhFiQuzDX/kIs7oymi/znDqF/u01OSDLakLMhPHjGPLsG"))
		o.Expect(output).To(o.ContainSubstring("2WHFDbGZFbVkvSkFyQVdDRW5sanh1aTFvZUtzV083WnhteFF"))

		g.By("render bundle image")
		output, err = opmCLI.Run("render").Args("quay.io/olmqe/cockroachdb-operator:5.0.4-2199", "quay.io/olmqe/cockroachdb-operator:5.0.3-2199").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("\"name\": \"cockroachdb\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.4\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("\"package\": \"cockroachdb\""))
		o.Expect(output).NotTo(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).NotTo(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.3"))
		o.Expect(output).To(o.ContainSubstring("\"group\": \"charts.operatorhub.io\""))
		o.Expect(output).To(o.ContainSubstring("\"version\": \"5.0.4\""))
		o.Expect(output).To(o.ContainSubstring("\"version\": \"5.0.3\""))

		g.By("render directory")
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		configDir := filepath.Join(opmBaseDir, "render", "configs")
		output, err = opmCLI.Run("render").Args(configDir).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("\"image\": \"quay.io/olmqe/cockroachdb-operator:5.0.3-2199\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.3"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"cockroachdb.v5.0.4\""))
		o.Expect(output).To(o.ContainSubstring("\"replaces\": \"cockroachdb.v5.0.3\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.4\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.4"))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"windup-operator.0.0.5\""))
		o.Expect(output).To(o.ContainSubstring("quay.io/windupeng/windup-operator-native:0.0.5"))

	})

	// author: kuiwang@redhat.com
	g.It("Author:kuiwang-Medium-43180-opm init dc configuration package", func() {
		g.By("init package")
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		readme := filepath.Join(opmBaseDir, "render", "init", "readme.md")
		testpng := filepath.Join(opmBaseDir, "render", "init", "test.png")

		output, err := opmCLI.Run("init").Args("--default-channel=alpha", "-d", readme, "-i", testpng, "mta-operator").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		o.Expect(output).To(o.ContainSubstring("\"schema\": \"olm.package\""))
		o.Expect(output).To(o.ContainSubstring("\"name\": \"mta-operator\""))
		o.Expect(output).To(o.ContainSubstring("\"defaultChannel\": \"alpha\""))
		o.Expect(output).To(o.ContainSubstring("zcfHkVw9GfpbJmeev9F08WW8uDkaslwX6avlWGU6N"))
		o.Expect(output).To(o.ContainSubstring("\"description\": \"it is testing\""))

	})

	// author: kuiwang@redhat.com
	g.It("Author:kuiwang-Medium-43248-Support ignoring files when loading declarative configs", func() {

		opmBaseDir := exutil.FixturePath("testdata", "opm")
		correctIndex := path.Join(opmBaseDir, "render", "validate", "configs")
		wrongIndex := path.Join(opmBaseDir, "render", "validate", "configs-wrong")
		wrongIgnoreIndex := path.Join(opmBaseDir, "render", "validate", "configs-wrong-ignore")

		g.By("validate correct index")
		output, err := opmCLI.Run("validate").Args(correctIndex).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)

		g.By("validate wrong index")
		output, err = opmCLI.Run("validate").Args(wrongIndex).Output()
		o.Expect(err).To(o.HaveOccurred())
		e2e.Logf(output)

		g.By("validate index with ignore wrong json")
		output, err = opmCLI.Run("validate").Args(wrongIgnoreIndex).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)

	})

	// author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-43096-opm alpha diff support heads only", func() {

		g.By("check the bundle image is not supportted")
		_, err := opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/mta-operator:v0.0.4-1869").Output()
		o.Expect(err).To(o.HaveOccurred())

		g.By("opm diff index image with heads-only mode")
		output, err := opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-head").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("schema: olm.package"))
		o.Expect(output).To(o.ContainSubstring("name: cockroachdb"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869"))
		o.Expect(output).To(o.ContainSubstring("name: cockroachdb.v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("zdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZW"))
		o.Expect(output).To(o.ContainSubstring("image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("defaultChannel: beta"))
		o.Expect(output).To(o.ContainSubstring("name: mta-operator"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.2"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/windupeng/windup-operator-native:0.0.2"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.3"))
		o.Expect(output).To(o.ContainSubstring("RHaSIsInR5cGUiOiJzdHJpbmcifSwiZXhlY3V0b3JfbWVtX3J"))
		o.Expect(output).To(o.ContainSubstring("replaces: windup-operator.0.0.2"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.4"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/mta-operator:v0.0.4-1869"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/mta-operator:v0.0.5-1869"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/windupeng/windup-operator-native:0.0.5"))
		o.Expect(output).To(o.ContainSubstring("versionRange: 0.0.5"))

		g.By("opm diff index db with heads-only mode")
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		indexDb := filepath.Join(opmBaseDir, "render", "diff", "index.db")
		output, err = opmCLI.Run("alpha").Args("diff", indexDb).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("schema: olm.package"))
		o.Expect(output).To(o.ContainSubstring("name: cockroachdb"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869"))
		o.Expect(output).To(o.ContainSubstring("name: cockroachdb.v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("zdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZW"))
		o.Expect(output).To(o.ContainSubstring("image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("defaultChannel: beta"))
		o.Expect(output).To(o.ContainSubstring("name: mta-operator"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.2"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/windupeng/windup-operator-native:0.0.2"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.3"))
		o.Expect(output).To(o.ContainSubstring("RHaSIsInR5cGUiOiJzdHJpbmcifSwiZXhlY3V0b3JfbWVtX3J"))
		o.Expect(output).To(o.ContainSubstring("replaces: windup-operator.0.0.2"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.4"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/mta-operator:v0.0.4-1869"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/mta-operator:v0.0.5-1869"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/windupeng/windup-operator-native:0.0.5"))
		o.Expect(output).To(o.ContainSubstring("versionRange: 0.0.5"))

		g.By("opm diff index image which package has no dependecy for heads-only mode")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-head-nodep").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("schema: olm.package"))
		o.Expect(output).To(o.ContainSubstring("name: cockroachdb"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869"))
		o.Expect(output).To(o.ContainSubstring("name: cockroachdb.v5.0.4"))
		o.Expect(output).To(o.ContainSubstring("zdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZW"))
		o.Expect(output).To(o.ContainSubstring("image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/helmoperators/cockroachdb:v5.0.4"))
		o.Expect(output).NotTo(o.ContainSubstring("versionRange: 0.0.5"))
		o.Expect(output).To(o.ContainSubstring("defaultChannel: beta"))
		o.Expect(output).To(o.ContainSubstring("name: mta-operator"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.2"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.3"))
		o.Expect(output).To(o.ContainSubstring("RHaSIsInR5cGUiOiJzdHJpbmcifSwiZXhlY3V0b3JfbWVtX3J"))
		o.Expect(output).To(o.ContainSubstring("replaces: windup-operator.0.0.2"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))
		o.Expect(output).NotTo(o.ContainSubstring("image: quay.io/olmqe/mta-operator:v0.0.4-1869"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/mta-operator:v0.0.5-1869"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/windupeng/windup-operator-native:0.0.5"))

	})

	// author: kuiwang@redhat.com
	// it conflicts with case 43096, so set it as Serial and keep 43096 executed in parallel.
	g.It("ConnectedOnly-Author:kuiwang-Medium-43097-opm alpha diff support latest [Serial]", func() {

		g.By("opm alpha diff same index image")
		output, err := opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-1", "quay.io/olmqe/olm-index:OLM-1869-latest-1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		diffResults := strings.Split(output, "\n")
		for _, result := range diffResults {
			o.Expect(result).To(o.ContainSubstring("level=warning"))
		}

		g.By("opm alpha diff index with bundle change: new, dependency change, upgrade graph change")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-1", "quay.io/olmqe/olm-index:OLM-1869-latest-2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/buildv2-operator:0.3.0-1869-nodep"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.3-1869-deppack"))
		o.Expect(output).To(o.ContainSubstring("versionRange: 0.0.5"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869-nodep"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))

		g.By("opm alpha diff index with bundle change: change channel, upgrade graph change")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-1", "quay.io/olmqe/olm-index:OLM-1869-latest-2-4").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.3-1869-nodep-beta"))
		o.Expect(output).NotTo(o.ContainSubstring("versionRange: 0.0.5"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))

		g.By("opm alpha diff index with bundle change: same bundle, upgrade graph change")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-1", "quay.io/olmqe/olm-index:OLM-1869-latest-2-5").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))

		g.By("opm alpha diff index with new bundle which dependency is in new index")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-4", "quay.io/olmqe/olm-index:OLM-1869-latest-5").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869-depgvk"))
		o.Expect(output).To(o.ContainSubstring("kind: Windup"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.5"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))

		g.By("opm alpha diff index with new bundle which dependency is in old index")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-6", "quay.io/olmqe/olm-index:OLM-1869-latest-7").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/buildv2-operator:0.3.0-1869-deppack"))
		o.Expect(output).To(o.ContainSubstring("versionRange: 0.0.4"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869-depgvk"))
		o.Expect(output).To(o.ContainSubstring("kind: Windup"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.5"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))

		g.By("opm alpha diff index with new bundle which dependency is only in new index")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-8", "quay.io/olmqe/olm-index:OLM-1869-latest-9").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.3-1869-deppack"))
		o.Expect(output).To(o.ContainSubstring("versionRange: 0.0.5"))
		o.Expect(output).NotTo(o.ContainSubstring("image: quay.io/olmqe/buildv2"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.5"))
		o.Expect(output).To(o.ContainSubstring("name: windup-operator.0.0.4"))

		g.By("opm alpha diff index with new bundle which dependency is not in both old and new index")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-10", "quay.io/olmqe/olm-index:OLM-1869-latest-11").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869-deppack-bv"))
		o.Expect(output).To(o.ContainSubstring("versionRange: 0.3.0"))
		o.Expect(output).NotTo(o.ContainSubstring("image: quay.io/olmqe/buildv2"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.5"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))

		g.By("opm alpha diff index with index db")
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		indexDb := filepath.Join(opmBaseDir, "render", "diff", "index-latest-2.db")
		output, err = opmCLI.Run("alpha").Args("diff", "quay.io/olmqe/olm-index:OLM-1869-latest-1", indexDb).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/buildv2-operator:0.3.0-1869-nodep"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.3-1869-deppack"))
		o.Expect(output).To(o.ContainSubstring("versionRange: 0.0.5"))
		o.Expect(output).To(o.ContainSubstring("image: quay.io/olmqe/cockroachdb-operator:5.0.4-1869-nodep"))
		o.Expect(output).NotTo(o.ContainSubstring("name: windup-operator.0.0.4"))

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

	// author: bandrade@redhat.com
	g.It("Author:bandrade-VMonly-Low-30318-Bundle build understands packages", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		testDataPath := filepath.Join(opmBaseDir, "aqua")
		opmCLI.ExecCommandPath = testDataPath
		defer DeleteDir(testDataPath, "fixture-testdata")

		g.By("step: opm alpha bundle generate")
		output, err := opmCLI.Run("alpha").Args("bundle", "generate", "-d", "1.0.1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		if !strings.Contains(output, "Writing annotations.yaml") || !strings.Contains(output, "Writing bundle.Dockerfile") {
			e2e.Failf("Failed to execute opm alpha bundle generate : %s", output)
		}
	})
})

var _ = g.Describe("[sig-operators] OLM opm with podman", func() {
	defer g.GinkgoRecover()

	var podmanCLI = container.NewPodmanCLI()
	var opmCLI = NewOpmCLI()
	var sqlit = db.NewSqlit()
	var quayCLI = container.NewQuayCLI()
	var oc = exutil.NewCLI("vmonly-"+getRandomString(), exutil.KubeConfigPath())

	// author: kuiwang@redhat.com
	g.It("ConnectedOnly-VMonly-Author:kuiwang-Medium-44201-Add a warning to validation for v1beta1 deprecations in an operator bundle", func() {

		containerCLI := podmanCLI

		bundleImage := "quay.io/olmqe/teiid-operator:v0.4.0-2222"
		defer containerCLI.RemoveImage(bundleImage)

		g.By("opm validate it with warning")
		output, err := opmCLI.Run("alpha").Args("bundle", "validate", "-t", bundleImage, "-b", "podman").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("v1beta1 CustomResourceDefinitions will be removed in the future"))
	})

	// author: xzha@redhat.com
	g.It("Author:xzha-ConnectedOnly-VMonly-Medium-25955-opm Ability to generate scaffolding for Operator Bundle", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "learn_operator")
		opmCLI.ExecCommandPath = TestDataPath
		defer DeleteDir(TestDataPath, "fixture-testdata")
		imageTag := "quay.io/olmqe/25955-operator-" + getRandomString() + ":v0.0.1"

		g.By("step: opm alpha bundle generate")
		output, err := opmCLI.Run("alpha").Args("bundle", "generate", "-d", "package/0.0.1", "-p", "25955-operator", "-c", "alpha", "-e", "alpha").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		if !strings.Contains(output, "Writing annotations.yaml") || !strings.Contains(output, "Writing bundle.Dockerfile") {
			e2e.Failf("Failed to execute opm alpha bundle generate : %s", output)
		}

		g.By("step: opm alpha bundle build")
		e2e.Logf("clean test data")
		DeleteDir(TestDataPath, "fixture-testdata")
		opmBaseDir = exutil.FixturePath("testdata", "opm")
		TestDataPath = filepath.Join(opmBaseDir, "learn_operator")
		opmCLI.ExecCommandPath = TestDataPath
		_, err = podmanCLI.RemoveImage(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("run opm alpha bundle build")
		defer podmanCLI.RemoveImage(imageTag)
		output, _ = opmCLI.Run("alpha").Args("bundle", "build", "-d", "package/0.0.1", "-b", "podman", "--tag", imageTag, "-p", "25955-operator", "-c", "alpha", "-e", "alpha", "--overwrite").Output()
		e2e.Logf(output)
		if !strings.Contains(output, "COMMIT "+imageTag) {
			e2e.Failf("Failed to execute opm alpha bundle build : %s", output)
		}

		e2e.Logf("step: check image %s exist", imageTag)
		existFlag, err := podmanCLI.CheckImageExist(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("check image exist is %v", existFlag)
		o.Expect(existFlag).To(o.BeTrue())
	})

	// author: xzha@redhat.com
	g.It("Author:xzha-ConnectedOnly-VMonly-Medium-37294-OPM can strand packages with prune stranded", func() {
		containerTool := "podman"
		containerCLI := podmanCLI
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "temp")
		opmCLI.ExecCommandPath = TestDataPath
		defer DeleteDir(TestDataPath, "fixture-testdata")
		indexImage := "quay.io/olmqe/etcd-index:test-37294"
		indexImageSemver := "quay.io/olmqe/etcd-index:test-37294-semver"

		g.By("step: check etcd-index:test-37294, operatorbundle has two records, channel_entry has one record")
		indexdbpath1 := filepath.Join(TestDataPath, getRandomString())
		err := os.Mkdir(TestDataPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.Mkdir(indexdbpath1, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImage, "--path", "/database/index.db:"+indexdbpath1).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath1, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err := sqlit.DBMatch(path.Join(indexdbpath1, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath1, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("step: prune-stranded this index image")
		indexImageTmp1 := indexImage + getRandomString()
		defer containerCLI.RemoveImage(indexImageTmp1)
		output, err := opmCLI.Run("index").Args("prune-stranded", "-f", indexImage, "--tag", indexImageTmp1, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexImageTmp1).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(indexImageTmp1, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: check index image operatorbundle has one record")
		indexdbpath2 := filepath.Join(TestDataPath, getRandomString())
		err = os.Mkdir(indexdbpath2, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImageTmp1, "--path", "/database/index.db:"+indexdbpath2).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath2, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.DBMatch(path.Join(indexdbpath2, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath2, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("test 2")
		g.By("step: step: check etcd-index:test-37294-semver, operatorbundle has two records, channel_entry has two records")
		indexdbpath3 := filepath.Join(TestDataPath, getRandomString())
		err = os.Mkdir(indexdbpath3, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImageSemver, "--path", "/database/index.db:"+indexdbpath3).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath3, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.DBMatch(path.Join(indexdbpath3, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath3, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("step: prune-stranded this index image")
		indexImageTmp2 := indexImage + getRandomString()
		defer containerCLI.RemoveImage(indexImageTmp2)
		output, err = opmCLI.Run("index").Args("prune-stranded", "-f", indexImageSemver, "--tag", indexImageTmp2, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexImageTmp2).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(indexImageTmp2, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: check index image has both v0.9.2 and v0.9.2")
		indexdbpath4 := filepath.Join(TestDataPath, getRandomString())
		err = os.Mkdir(indexdbpath4, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImageTmp2, "--path", "/database/index.db:"+indexdbpath4).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath4, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.DBMatch(path.Join(indexdbpath4, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath4, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		e2e.Logf("step: check index image has both v0.9.2 and v0.9.2 SUCCESS")
	})

	// author: kuiwang@redhat.com
	g.It("Author:kuiwang-ConnectedOnly-VMonly-Medium-40167-bundle image is missed in index db of index image", func() {
		var (
			opmBaseDir   = exutil.FixturePath("testdata", "opm")
			TestDataPath = filepath.Join(opmBaseDir, "temp")
			opmCLI       = NewOpmCLI()

			containerTool = "podman"
			containerCLI  = podmanCLI

			// it is shared image. could not need to remove it.
			indexImage = "quay.io/olmqe/cockroachdb-index:2.1.11-40167"
			// it is generated by case. need to remove it after case exist normally or abnormally
			customIndexImage = "quay.io/olmqe/cockroachdb-index:2.1.11-40167-custome-" + getRandomString()
		)
		defer DeleteDir(TestDataPath, "fixture-testdata")
		defer containerCLI.RemoveImage(customIndexImage)
		defer quayCLI.DeleteTag(strings.Replace(customIndexImage, "quay.io/", "", 1))
		err := os.Mkdir(TestDataPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		opmCLI.ExecCommandPath = TestDataPath

		g.By("prune redhat index image to get custom index image")
		if output, err := opmCLI.Run("index").Args("prune", "-f", indexImage, "-p", "cockroachdb", "-t", customIndexImage, "-c", containerTool).Output(); err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		if output, err := containerCLI.Run("push").Args(customIndexImage).Output(); err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("extract db file")
		indexdbpath1 := filepath.Join(TestDataPath, getRandomString())
		err = os.Mkdir(indexdbpath1, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", customIndexImage, "--path", "/database/index.db:"+indexdbpath1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath1, "index.db"))

		g.By("check if the bunld image is in db index")
		rows, err := sqlit.QueryDB(path.Join(indexdbpath1, "index.db"), "select image from related_image where operatorbundle_name like 'cockroachdb%';")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer rows.Close()
		var imageList string
		var image string
		for rows.Next() {
			rows.Scan(&image)
			imageList = imageList + image
		}
		e2e.Logf("imageList is %v", imageList)
		o.Expect(imageList).To(o.ContainSubstring("cockroachdb-operator"))

	})

	// author: xzha@redhat.com
	g.It("Author:xzha-ConnectedOnly-VMonly-Medium-40530-The index image generated by opm index prune should not leave unrelated images", func() {
		containerCLI := podmanCLI
		containerTool := "podman"
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "temp")
		opmCLI.ExecCommandPath = TestDataPath
		defer DeleteDir(TestDataPath, "fixture-testdata")
		indexImage := "quay.io/olmqe/redhat-operator-index:40530"
		defer containerCLI.RemoveImage(indexImage)

		g.By("step: check the index image has other bundles except cluster-logging")
		indexTmpPath1 := filepath.Join(TestDataPath, getRandomString())
		err := os.MkdirAll(indexTmpPath1, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImage, "--path", "/database/index.db:"+indexTmpPath1).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexTmpPath1, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())

		rows, err := sqlit.QueryDB(path.Join(indexTmpPath1, "index.db"), "select distinct(operatorbundle_name) from related_image where operatorbundle_name not in (select operatorbundle_name from channel_entry)")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer rows.Close()
		var OperatorBundles []string
		var name string
		for rows.Next() {
			rows.Scan(&name)
			OperatorBundles = append(OperatorBundles, name)
		}
		o.Expect(OperatorBundles).NotTo(o.BeEmpty())

		g.By("step: Prune the index image to keep cluster-logging only")
		indexImage1 := indexImage + getRandomString()
		defer containerCLI.RemoveImage(indexImage1)
		output, err := opmCLI.Run("index").Args("prune", "-f", indexImage, "-p", "cluster-logging", "-t", indexImage1, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexImage1).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(indexImage1, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: check database, there is no related images")
		indexTmpPath2 := filepath.Join(TestDataPath, getRandomString())
		err = os.MkdirAll(indexTmpPath2, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImage1, "--path", "/database/index.db:"+indexTmpPath2).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexTmpPath2, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())

		rows2, err := sqlit.QueryDB(path.Join(indexTmpPath2, "index.db"), "select distinct(operatorbundle_name) from related_image where operatorbundle_name not in (select operatorbundle_name from channel_entry)")
		o.Expect(err).NotTo(o.HaveOccurred())
		OperatorBundles = nil
		defer rows2.Close()
		for rows2.Next() {
			rows2.Scan(&name)
			OperatorBundles = append(OperatorBundles, name)
		}
		o.Expect(OperatorBundles).To(o.BeEmpty())

		g.By("step: check the image mirroring mapping")
		manifestsPath := filepath.Join(TestDataPath, getRandomString())
		output, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("catalog", "mirror", indexImage1, "localhost:5000", "--manifests-only", "--to-manifests="+manifestsPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("/database/index.db"))

		result, err := exec.Command("bash", "-c", "cat "+manifestsPath+"/mapping.txt").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).NotTo(o.BeEmpty())

		result, _ = exec.Command("bash", "-c", "cat "+manifestsPath+"/mapping.txt|grep -v ose-cluster-logging|grep -v ose-logging|grep -v redhat-operator-index:40530").Output()
		o.Expect(result).To(o.BeEmpty())
		g.By("step: 40530 SUCCESS")

	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-ConnectedOnly-VMonly-Medium-34049-opm can prune operators from index", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "temp")
		indexTmpPath := filepath.Join(TestDataPath, getRandomString())
		defer DeleteDir(TestDataPath, indexTmpPath)
		err := os.MkdirAll(indexTmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		containerCLI := podmanCLI
		containerTool := "podman"
		sourceImageTag := "quay.io/olmqe/multi-index:2.0"
		imageTag := "quay.io/olmqe/multi-index:3.0." + getRandomString()
		defer podmanCLI.RemoveImage(imageTag)
		defer podmanCLI.RemoveImage(sourceImageTag)
		output, err := opmCLI.Run("index").Args("prune", "-f", sourceImageTag, "-p", "planetscale", "-t", imageTag, "-c", containerTool).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "deleting packages") || !strings.Contains(output, "pkg=lib-bucket-provisioner") {
			e2e.Failf(fmt.Sprintf("Failed to obtain the removed packages from prune : %s", output))
		}

		output, err = containerCLI.Run("push").Args(imageTag).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(imageTag, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", imageTag, "--path", "/database/index.db:"+indexTmpPath).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexTmpPath, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())

		result, err := sqlit.DBMatch(path.Join(indexTmpPath, "index.db"), "channel_entry", "operatorbundle_name", []string{"lib-bucket-provisioner.v1.0.0"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeFalse())

	})

	g.It("Author:xzha-ConnectedOnly-VMonly-Medium-26594-Related Images", func() {
		containerCLI := podmanCLI
		containerTool := "podman"
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "eclipse-che")
		TmpDataPath := filepath.Join(opmBaseDir, "tmp")
		err := os.MkdirAll(TmpDataPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		bundleImageTag := "quay.io/olmqe/eclipse-che:7.32.2-" + getRandomString()

		defer exec.Command("kill", "-9", "$(lsof -t -i:26594)").Output()
		defer DeleteDir(TestDataPath, "fixture-testdata")
		defer containerCLI.RemoveImage(bundleImageTag)
		defer quayCLI.DeleteTag(strings.Replace(bundleImageTag, "quay.io/", "", 1))

		g.By("step: build bundle image ")
		opmCLI.ExecCommandPath = TestDataPath
		output, err := opmCLI.Run("alpha").Args("bundle", "build", "-d", "7.32.2", "-b", containerTool, "-t", bundleImageTag, "-p", "eclipse-che", "-c", "alpha", "-e", "alpha", "--overwrite").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("Writing annotations.yaml"))
		o.Expect(string(output)).To(o.ContainSubstring("Writing bundle.Dockerfile"))

		if output, err = containerCLI.Run("push").Args(bundleImageTag).Output(); err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("step: build bundle.db")
		dbFilePath := TmpDataPath + "bundles.db"
		if output, err := opmCLI.Run("registry").Args("add", "-b", bundleImageTag, "-d", dbFilePath, "-c", containerTool, "--mode", "semver").Output(); err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("step: Check if the related images stores in this database")
		image := "quay.io/che-incubator/configbump@sha256:175ff2ba1bd74429de192c0a9facf39da5699c6da9f151bd461b3dc8624dd532"

		result, err := sqlit.DBMatch(dbFilePath, "package", "name", []string{"eclipse-che"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBHas(dbFilePath, "related_image", "image", []string{image})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("step: Run the opm registry server binary to load manifest and serves a grpc API to query it.")
		e2e.Logf("step: Run the registry-server ")
		cmd := exec.Command("opm", "registry", "serve", "-d", dbFilePath, "-t", filepath.Join(TmpDataPath, "26594.log"), "-p", "26594")
		cmd.Dir = TmpDataPath
		err = cmd.Start()
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(time.Second * 1)
		e2e.Logf("step: check api.Registry/ListPackages")
		outputCurl, err := exec.Command("grpcurl", "-plaintext", "localhost:26594", "api.Registry/ListPackages").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(outputCurl)).To(o.ContainSubstring("eclipse-che"))
		e2e.Logf("step: check api.Registry/GetBundleForChannel")
		outputCurl, err = exec.Command("grpcurl", "-plaintext", "-d", "{\"pkgName\":\"eclipse-che\",\"channelName\":\"alpha\"}", "localhost:26594", "api.Registry/GetBundleForChannel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(outputCurl)).To(o.ContainSubstring(image))
		cmd.Process.Kill()
		g.By("step: SUCCESS")

	})

	g.It("Author:xzha-ConnectedOnly-Medium-43409-opm can list catalog contents", func() {
		dbimagetag := "quay.io/olmqe/community-operator-index:v4.8"
		dcimagetag := "quay.io/olmqe/community-operator-index:v4.8-dc"

		g.By("1, testing with index.db image ")
		g.By("1.1 list packages")
		output, err := opmCLI.Run("alpha").Args("list", "packages", dbimagetag).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale API Management"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))

		g.By("1.2 list channels")
		output, err = opmCLI.Run("alpha").Args("list", "channels", dbimagetag).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.7.0"))

		g.By("1.3 list channels in a package")
		output, err = opmCLI.Run("alpha").Args("list", "channels", dbimagetag, "3scale-community-operator").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.9"))

		g.By("1.4 list bundles")
		output, err = opmCLI.Run("alpha").Args("list", "bundles", dbimagetag).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.6.0"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.7.0"))

		g.By("1.5 list bundles in a package")
		output, err = opmCLI.Run("alpha").Args("list", "bundles", dbimagetag, "wso2am-operator").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.0.0"))
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.0.1"))
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.1.0"))

		g.By("2, testing with dc format index image")
		g.By("2.1 list packages")
		output, err = opmCLI.Run("alpha").Args("list", "packages", dcimagetag).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale API Management"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))

		g.By("2.2 list channels")
		output, err = opmCLI.Run("alpha").Args("list", "channels", dcimagetag).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.7.0"))

		g.By("2.3 list channels in a package")
		output, err = opmCLI.Run("alpha").Args("list", "channels", dcimagetag, "3scale-community-operator").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.9"))

		g.By("2.4 list bundles")
		output, err = opmCLI.Run("alpha").Args("list", "bundles", dcimagetag).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.6.0"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.7.0"))

		g.By("2.5 list bundles in a package")
		output, err = opmCLI.Run("alpha").Args("list", "bundles", dcimagetag, "wso2am-operator").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.0.0"))
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.0.1"))
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.1.0"))

		g.By("3, testing with index.db file")
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TmpDataPath := filepath.Join(opmBaseDir, "tmp")
		indexdbFilePath := filepath.Join(TmpDataPath, "index.db")
		err = os.MkdirAll(TmpDataPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("get index.db")
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", dbimagetag, "--path", "/database/index.db:"+TmpDataPath).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", indexdbFilePath)
		if _, err := os.Stat(indexdbFilePath); os.IsNotExist(err) {
			e2e.Logf("get index.db Failed")
		}

		g.By("3.1 list packages")
		output, err = opmCLI.Run("alpha").Args("list", "packages", indexdbFilePath).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale API Management"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))

		g.By("3.2 list channels")
		output, err = opmCLI.Run("alpha").Args("list", "channels", indexdbFilePath).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.7.0"))

		g.By("3.3 list channels in a package")
		output, err = opmCLI.Run("alpha").Args("list", "channels", indexdbFilePath, "3scale-community-operator").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.10"))
		o.Expect(string(output)).To(o.ContainSubstring("threescale-2.9"))

		g.By("3.4 list bundles")
		output, err = opmCLI.Run("alpha").Args("list", "bundles", indexdbFilePath).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.6.0"))
		o.Expect(string(output)).To(o.ContainSubstring("3scale-community-operator.v0.7.0"))

		g.By("3.5 list bundles in a package")
		output, err = opmCLI.Run("alpha").Args("list", "bundles", indexdbFilePath, "wso2am-operator").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.0.0"))
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.0.1"))
		o.Expect(string(output)).To(o.ContainSubstring("wso2am-operator.v1.1.0"))

		g.By("step: SUCCESS")
	})

	g.It("Author:xzha-ConnectedOnly-Medium-43756-resolve and mirror dependencies automatically", func() {
		imagetag1 := "quay.io/olmqe/community-operator-index:43756-1"
		imagetag2 := "quay.io/olmqe/community-operator-index:43756-2"
		imagetag3 := "quay.io/olmqe/community-operator-index:43756-3-dc"

		g.By("1, opm alpha diff image1")
		output, err := opmCLI.Run("alpha").Args("diff", imagetag1, "-oyaml").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).To(o.ContainSubstring("name: ditto-operator.v0.1.0"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.6"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.7"))

		g.By("opm alpha diff image2")
		output, err = opmCLI.Run("alpha").Args("diff", imagetag2, "-oyaml").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: ditto-operator.v0.1.0"))
		o.Expect(string(output)).To(o.ContainSubstring("name: ditto-operator.v0.1.1"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: planetscale-operator.v0.1.6"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.7"))

		g.By("opm alpha diff images1 and image2")
		output, err = opmCLI.Run("alpha").Args("diff", imagetag1, imagetag2, "-o", "yaml").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: ditto-operator.v0.1.0"))
		o.Expect(string(output)).To(o.ContainSubstring("name: ditto-operator.v0.1.1"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: planetscale-operator.v0.1.6"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: planetscale-operator.v0.1.7"))

		g.By("opm alpha diff image3")
		output, err = opmCLI.Run("alpha").Args("diff", imagetag3, "-o", "yaml").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: ditto-operator.v0.1.0"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: ditto-operator.v0.1.1"))
		o.Expect(string(output)).To(o.ContainSubstring("name: ditto-operator.v0.2.0"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.6"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.7"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.8"))

		g.By("opm alpha diff images1 image3")
		output, err = opmCLI.Run("alpha").Args("diff", imagetag1, imagetag3, "-o", "yaml").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: ditto-operator.v0.1.0"))
		o.Expect(string(output)).To(o.ContainSubstring("name: ditto-operator.v0.1.1"))
		o.Expect(string(output)).To(o.ContainSubstring("name: ditto-operator.v0.2.0"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: planetscale-operator.v0.1.6"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: planetscale-operator.v0.1.7"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.8"))

		g.By("opm alpha diff images2 image3")
		output, err = opmCLI.Run("alpha").Args("diff", imagetag2, imagetag3, "-o", "yaml").Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: ditto-operator.v0.1.0"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: ditto-operator.v0.1.1"))
		o.Expect(string(output)).To(o.ContainSubstring("name: ditto-operator.v0.2.0"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: planetscale-operator.v0.1.6"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("name: planetscale-operator.v0.1.7"))
		o.Expect(string(output)).To(o.ContainSubstring("name: planetscale-operator.v0.1.8"))

		g.By("step: SUCCESS")
	})

	// author: xzha@redhat.com
	g.It("Author:xzha-ConnectedOnly-VMonly-Medium-43147-opm support rebuild index if any bundles have been truncated", func() {
		containerCLI := podmanCLI
		containerTool := "podman"
		indexImage := "quay.io/olmqe/ditto-index:43147"
		indexImageDep := "quay.io/olmqe/ditto-index:43147-dep" + getRandomString()
		indexImageOW := "quay.io/olmqe/ditto-index:43147-ow" + getRandomString()

		defer containerCLI.RemoveImage(indexImage)
		defer containerCLI.RemoveImage(indexImageDep)
		defer containerCLI.RemoveImage(indexImageOW)
		defer quayCLI.DeleteTag(strings.Replace(indexImageDep, "quay.io/", "", 1))

		g.By("step: run deprecatetruncate")
		output, err := opmCLI.Run("index").Args("deprecatetruncate", "-b", "quay.io/olmqe/ditto-operator:0.1.1", "-f", indexImage, "-t", indexImageDep, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexImageDep).Output()
		if err != nil {
			e2e.Logf(output)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("check there is no channel alpha")
		output, err = opmCLI.Run("alpha").Args("list", "channels", indexImageDep).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).NotTo(o.ContainSubstring("alpha"))
		o.Expect(string(output)).To(o.ContainSubstring("beta"))
		o.Expect(string(output)).NotTo(o.ContainSubstring("ditto-operator.v0.1.0"))

		g.By("re-adding the bundle")
		output, err = opmCLI.Run("index").Args("add", "-b", "quay.io/olmqe/ditto-operator:0.1.1,quay.io/olmqe/ditto-operator:0.2.0", "-f", indexImageDep, "-t", indexImageOW, "--overwrite-latest", "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).NotTo(o.ContainSubstring("ERRO"))

		g.By("step: 43147 SUCCESS")
	})

	// author: xzha@redhat.com
	g.It("Author:xzha-ConnectedOnly-VMonly-Medium-43562-opm should raise error when adding an bundle whose version is higher than the bundle being added", func() {
		containerCLI := podmanCLI
		containerTool := "podman"
		indexImage := "quay.io/olmqe/ditto-index:43562"
		indexImage1 := "quay.io/olmqe/ditto-index:43562-1" + getRandomString()
		indexImage2 := "quay.io/olmqe/ditto-index:43562-2" + getRandomString()

		defer containerCLI.RemoveImage(indexImage)
		defer containerCLI.RemoveImage(indexImage1)
		defer containerCLI.RemoveImage(indexImage2)

		g.By("step: run add ditto-operator.v0.1.0 replace ditto-operator.v0.1.1")
		output1, err := opmCLI.Run("index").Args("add", "-b", "quay.io/olmqe/ditto-operator:43562-0.1.0", "-f", indexImage, "-t", indexImage1, "-c", containerTool).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(string(output1)).To(o.ContainSubstring("error"))
		o.Expect(string(output1)).To(o.ContainSubstring("permissive mode disabled"))
		o.Expect(string(output1)).To(o.ContainSubstring("this may be due to incorrect channel head"))

		output2, err := opmCLI.Run("index").Args("add", "-b", "quay.io/olmqe/ditto-operator:43562-0.1.2", "-f", indexImage, "-t", indexImage1, "-c", containerTool).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(string(output2)).To(o.ContainSubstring("error"))
		o.Expect(string(output2)).To(o.ContainSubstring("permissive mode disabled"))
		o.Expect(string(output2)).To(o.ContainSubstring("this may be due to incorrect channel head"))

		g.By("test case 43562 SUCCESS")
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-VMonly-High-30786-Bundle addition commutativity", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		defer DeleteDir(opmBaseDir, "fixture-testdata")
		TestDataPath := filepath.Join(opmBaseDir, "temp")
		opmCLI.ExecCommandPath = TestDataPath

		var (
			bundles    [3]string
			bundleName [3]string
			indexName  = "index30786"
			matched    bool
			sqlResults []db.Channel
		)

		g.By("Setup environment")
		// see OCP-30786 for creation of these images
		bundles[0] = "quay.io/olmqe/etcd-bundle:0.9.0-39795"
		bundles[1] = "quay.io/olmqe/etcd-bundle:0.9.2-39795"
		bundles[2] = "quay.io/olmqe/etcd-bundle:0.9.4-39795"
		bundleName[0] = "etcdoperator.v0.9.0"
		bundleName[1] = "etcdoperator.v0.9.2"
		bundleName[2] = "etcdoperator.v0.9.4"
		containerCLI := podmanCLI

		indexTmpPath1 := filepath.Join(TestDataPath, "database")
		err := os.MkdirAll(indexTmpPath1, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create index image with a,b")
		index := 1
		a := 0
		b := 1
		order := "a,b"
		s := fmt.Sprintf("%v,%v", bundles[a], bundles[b])
		t1 := fmt.Sprintf("%v:%v", indexName, index)
		defer podmanCLI.RemoveImage(t1)
		msg, err := opmCLI.Run("index").Args("add", "-b", s, "-t", t1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v", bundles[a], bundles[b]), msg)
		o.Expect(matched).To(o.BeTrue())

		msg, err = containerCLI.Run("images").Args("-n", t1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("IMAGES in %v: %v", order, msg)
		o.Expect(msg).NotTo(o.BeEmpty())
		podmanCLI.RemoveImage(t1)

		g.By("Generate db with a,b & check with sqlite")
		msg, err = opmCLI.Run("index").Args("add", "-b", s, "--generate").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v", bundles[a], bundles[b]), msg)
		o.Expect(matched).To(o.BeTrue())

		sqlResults, err = sqlit.QueryOperatorChannel(path.Join(indexTmpPath1, "index.db"))
		// force string compare
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("sqlite contents %v: %v", order, sqlResults)
		o.Expect(fmt.Sprintf("%v", sqlResults[0])).To(o.ContainSubstring(bundleName[1]))
		o.Expect(fmt.Sprintf("%v", sqlResults[1])).To(o.ContainSubstring(bundleName[0]))
		os.Remove(path.Join(indexTmpPath1, "index.db"))

		g.By("Create index image with b,a")
		index++
		a = 1
		b = 0
		order = "b,a"
		s = fmt.Sprintf("%v,%v", bundles[a], bundles[b])
		t2 := fmt.Sprintf("%v:%v", indexName, index)
		defer podmanCLI.RemoveImage(t2)
		msg, err = opmCLI.Run("index").Args("add", "-b", s, "-t", t2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v", bundles[a], bundles[b]), msg)
		o.Expect(matched).To(o.BeTrue())

		msg, err = containerCLI.Run("images").Args("-n", t2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("IMAGES in %v: %v", order, msg)
		o.Expect(msg).NotTo(o.BeEmpty())
		podmanCLI.RemoveImage(t2)

		g.By("Generate db with b,a & check with sqlite")
		msg, err = opmCLI.Run("index").Args("add", "-b", s, "--generate").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v", bundles[a], bundles[b]), msg)
		o.Expect(matched).To(o.BeTrue())

		sqlResults, err = sqlit.QueryOperatorChannel(path.Join(indexTmpPath1, "index.db"))
		// force string compare
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("sqlite contents %v: %v", order, sqlResults)
		o.Expect(fmt.Sprintf("%v", sqlResults[0])).To(o.ContainSubstring(bundleName[1]))
		o.Expect(fmt.Sprintf("%v", sqlResults[1])).To(o.ContainSubstring(bundleName[0]))
		os.Remove(path.Join(indexTmpPath1, "index.db"))

		g.By("Create index image with a,b,c")
		index++
		a = 0
		b = 1
		c := 2
		order = "a,b,c"
		s = fmt.Sprintf("%v,%v,%v", bundles[a], bundles[b], bundles[c])
		t3 := fmt.Sprintf("%v:%v", indexName, index)
		defer podmanCLI.RemoveImage(t3)
		msg, err = opmCLI.Run("index").Args("add", "-b", s, "-t", t3).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v %v", bundles[a], bundles[b], bundles[c]), msg)
		o.Expect(matched).To(o.BeTrue())

		msg, err = containerCLI.Run("images").Args("-n", t3).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("IMAGES in %v: %v", order, msg)
		o.Expect(msg).NotTo(o.BeEmpty())
		podmanCLI.RemoveImage(t3)

		g.By("Generate db with a,b,c & check with sqlite")
		msg, err = opmCLI.Run("index").Args("add", "-b", s, "--generate").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v %v", bundles[a], bundles[b], bundles[c]), msg)
		o.Expect(matched).To(o.BeTrue())

		sqlResults, err = sqlit.QueryOperatorChannel(path.Join(indexTmpPath1, "index.db"))
		// force string compare
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("sqlite contents %v: %v", order, sqlResults)
		o.Expect(fmt.Sprintf("%v", sqlResults[0])).To(o.ContainSubstring(bundleName[2]))
		o.Expect(fmt.Sprintf("%v", sqlResults[1])).To(o.ContainSubstring(bundleName[1]))
		o.Expect(fmt.Sprintf("%v", sqlResults[2])).To(o.ContainSubstring(bundleName[0]))
		os.Remove(path.Join(indexTmpPath1, "index.db"))

		g.By("Create index image with b,c,a")
		index++
		a = 1
		b = 2
		c = 0
		order = "b,c,a"
		s = fmt.Sprintf("%v,%v,%v", bundles[a], bundles[b], bundles[c])
		t4 := fmt.Sprintf("%v:%v", indexName, index)
		defer podmanCLI.RemoveImage(t4)
		msg, err = opmCLI.Run("index").Args("add", "-b", s, "-t", t4).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v %v", bundles[a], bundles[b], bundles[c]), msg)
		o.Expect(matched).To(o.BeTrue())

		msg, err = containerCLI.Run("images").Args("-n", t4).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("IMAGES in %v: %v", order, msg)
		o.Expect(msg).NotTo(o.BeEmpty())
		podmanCLI.RemoveImage(t4)
		// no db check

		g.By("Create index image with c,a,b")
		index++
		a = 2
		b = 0
		c = 1
		order = "c,a,b"
		s = fmt.Sprintf("%v,%v,%v", bundles[a], bundles[b], bundles[c])
		t5 := fmt.Sprintf("%v:%v", indexName, index)
		defer podmanCLI.RemoveImage(t5)
		msg, err = opmCLI.Run("index").Args("add", "-b", s, "-t", t5).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v %v", bundles[a], bundles[b], bundles[c]), msg)
		o.Expect(matched).To(o.BeTrue())

		msg, err = containerCLI.Run("images").Args("-n", t5).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("IMAGES in %v: %v", order, msg)
		o.Expect(msg).NotTo(o.BeEmpty())
		podmanCLI.RemoveImage(t5)
		// no db check

		g.By("Generate db with b,a,c & check with sqlite")
		a = 1
		b = 0
		c = 2
		order = "b,a,c"
		s = fmt.Sprintf("%v,%v,%v", bundles[a], bundles[b], bundles[c])
		// no image check, just db

		msg, err = opmCLI.Run("index").Args("add", "-b", s, "--generate").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
		matched, err = regexp.MatchString(fmt.Sprintf("bundles=.*%v %v %v", bundles[a], bundles[b], bundles[c]), msg)
		o.Expect(matched).To(o.BeTrue())

		sqlResults, err = sqlit.QueryOperatorChannel(path.Join(indexTmpPath1, "index.db"))
		// force string compare
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("sqlite contents %v: %v", order, sqlResults)
		o.Expect(fmt.Sprintf("%v", sqlResults[0])).To(o.ContainSubstring(bundleName[2]))
		o.Expect(fmt.Sprintf("%v", sqlResults[1])).To(o.ContainSubstring(bundleName[1]))
		o.Expect(fmt.Sprintf("%v", sqlResults[2])).To(o.ContainSubstring(bundleName[0]))
		os.Remove(path.Join(indexTmpPath1, "index.db"))

		g.By("Finished")
	})

})
