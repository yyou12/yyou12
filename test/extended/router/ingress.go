package router

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	// e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("router-ingressclass", exutil.KubeConfigPath())

	// author: hongli@redhat.com
	g.It("Author:hongli-Critical-41117-ingress operator manages the IngressClass for each ingresscontroller", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp41117",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("check the ingress class created by default ingresscontroller")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingressclass/openshift-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("openshift.io/ingress-to-route"))

		g.By("create another custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		ingctrl.create(oc)
		err = waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer ingctrl.delete(oc)

		g.By("check the ingressclass is created by custom ingresscontroller")
		ingressclassname := "openshift-" + ingctrl.name
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingressclass", ingressclassname).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("openshift.io/ingress-to-route"))

		g.By("delete the custom ingresscontroller and ensure the ingresscalsss is removed")
		ingctrl.delete(oc)
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingressclass", ingressclassname).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("NotFound"))
	})
})
