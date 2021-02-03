package opm

import (
	"fmt"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] OLM opm should", func() {
	defer g.GinkgoRecover()

	var opmCLI = NewOpmCLI()

	// author: jiazha@redhat.com
	g.It("Medium-27620-Validate operator bundle Image and Contents", func() {

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
	g.It("Medium-34016-opm can prune operators from catalog", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		indexDB := filepath.Join(opmBaseDir, "index_34016.db")
		output, err := opmCLI.Run("registry").Args("prune", "-d", indexDB, "-p", "lib-bucket-provisioner").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "deleting packages") || !strings.Contains(output, "pkg=planetscale") {
			e2e.Failf(fmt.Sprintf("Failed to obtain the removed packages from prune : %s", output))
		}
	})
})
