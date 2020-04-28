package operators

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// XXX: bandrade@redhat.com -  Added this only to test private repository, must be removed
	g.It("OLM-High-OCP-XXXXX- test configmap deletion", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		configmap := filepath.Join(buildPruningBaseDir, "configmap-test.yaml")
		err := oc.SetNamespace("default").AsAdmin().Run("create").Args("-f", configmap).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		msg, err := oc.SetNamespace("default").AsAdmin().Run("get").Args("cm", "test").Output()
		if !strings.Contains(msg, "test") {
			e2e.Failf("it should have confimap test, but output was: %s", msg)
		}
		oc.SetNamespace("default").AsAdmin().Run("delete").Args("-f", configmap).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

	})

})
