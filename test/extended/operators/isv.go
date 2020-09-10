package operators

import (
	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/isv] ISV_Operators", func() {

	var (
		oc             = exutil.NewCLI("isv", exutil.KubeConfigPath())
		currentPackage Packagemanifest
	)
	defer g.GinkgoRecover()

	for i := range CertifiedOperators {
		operator := CertifiedOperators[i]
		g.It(TestCaseName(operator, BasicPrefix), func() {
			g.By("by installing", func() {
				currentPackage = CreateSubscription(operator, oc, INSTALLPLAN_AUTOMATIC_MODE)
				CheckDeployment(currentPackage, oc)
			})
			g.By("by uninstalling", func() {
				RemoveOperatorDependencies(currentPackage, oc, true)
			})
		})
	}

})
