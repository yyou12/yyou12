package router

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("router-tls", exutil.KubeConfigPath())

	// author: hongli@redhat.com
	g.It("Author:hongli-Critical-43300-enable client certificate with optional policy", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		cmFile := filepath.Join(buildPruningBaseDir, "ca-bundle.pem")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43300",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("create configmap client-ca-xxxxx in namespace openshift-config")
		createConfigMapFromFile(oc, "openshift-config", "client-ca-43300", cmFile)
		defer deleteConfigMap(oc, "openshift-config", "client-ca-43300")

		g.By("create custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		ingctrl.create(oc)
		defer ingctrl.delete(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("patch the ingresscontroller to enable client certificate with optional policy")
		routerpod := getRouterPod(oc, "ocp43300")
		patchResourceAsAdmin(oc, ingctrl.namespace, "ingresscontroller/ocp43300", "{\"spec\":{\"clientTLS\":{\"clientCA\":{\"name\":\"client-ca-43300\"},\"clientCertificatePolicy\":\"Optional\"}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check client certification config after custom router rolled out")
		newrouterpod := getRouterPod(oc, "ocp43300")
		env := readRouterPodEnv(oc, newrouterpod, "ROUTER_MUTUAL_TLS_AUTH")
		o.Expect(env).To(o.ContainSubstring(`ROUTER_MUTUAL_TLS_AUTH=optional`))
		o.Expect(env).To(o.ContainSubstring(`ROUTER_MUTUAL_TLS_AUTH_CA=/etc/pki/tls/client-ca/ca-bundle.pem`))
	})
})