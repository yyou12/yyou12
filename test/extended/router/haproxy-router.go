package router

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("router-env", exutil.KubeConfigPath())

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-40677-Ingresscontroller with endpointPublishingStrategy of nodePort allows PROXY protocol for source forwarding", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np-PROXY.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp40677",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a NP ingresscontroller with PROXY protocol set")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		ingctrl.create(oc)
		defer ingctrl.delete(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the router env to verify the PROXY variable is applied")
		podname := getRouterPod(oc, "ocp40677")
		dssearch := readRouterPodEnv(oc, podname, "ROUTER_USE_PROXY_PROTOCOL")
		o.Expect(dssearch).To(o.ContainSubstring(`ROUTER_USE_PROXY_PROTOCOL=true`))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-OCP-40675-Ingresscontroller with endpointPublishingStrategy of hostNetwork allows PROXY protocol for source forwarding", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-hn-PROXY.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp40675",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a NP ingresscontroller with PROXY protocol set")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		ingctrl.create(oc)
		defer ingctrl.delete(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the router env to verify the PROXY variable is applied")
		routername := getRouterPod(oc, "ocp40675")
		dssearch := readRouterPodEnv(oc, routername, "ROUTER_USE_PROXY_PROTOCOL")
		o.Expect(dssearch).To(o.ContainSubstring(`ROUTER_USE_PROXY_PROTOCOL=true`))
	})
})
