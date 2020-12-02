package operatorsdk

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] Operator_SDK should", func() {
	defer g.GinkgoRecover()

	var operatorsdkCLI = NewOperatorSDKCLI()
	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: jfan@redhat.com
	g.It("Medium-35458-SDK run bundle create registry image pod", func() {

		bundleImages := []struct {
			image  string
			indeximage string
			expect string
		}{
			{"quay.io/openshift-qe-optional-operators/ose-cluster-nfd-operator-bundle:latest", "quay.io/openshift-qe-optional-operators/ocp4-index:latest", "Successfully created registry pod"},
		}
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		for _, b := range bundleImages {
			g.By(fmt.Sprintf("create registry image pod %s", b.image))
			output, err := operatorsdkCLI.Run("run").Args("bundle", b.image, "--index-image", b.indeximage, "-n", oc.Namespace(), "--timeout=5m").Output()
			if strings.Contains(output, b.expect) {
				e2e.Logf(fmt.Sprintf("That's expected! %s", b.image))
			} else {
				e2e.Failf(fmt.Sprintf("Failed to validating the %s, error: %v", b.image, err))
			}
		}
	})
})
