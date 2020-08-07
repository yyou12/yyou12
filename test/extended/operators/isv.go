package operators

import (
	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/isv]", func() {

	var (
		oc             = exutil.NewCLI("isv", exutil.KubeConfigPath())
		currentPackage Packagemanifest
	)
	defer g.GinkgoRecover()

	for i := range CertifiedOperators {

		g.It(TestCaseName(CertifiedOperators[i], BasicPrefix), func() {
			g.By("by installing", func() {
				currentPackage = CreateSubscription(CertifiedOperators[i], oc)
				CheckDeployment(currentPackage, oc)
			})
			g.By("by uninstalling", func() {
				RemoveOperatorDependencies(currentPackage, oc, true)
			})
		})
	}

})
