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

	//author: jechen@redhat.com
	g.It("Author:jechen-Medium-42878-Errorfile stanzas and dummy default html files have been added to the router", func() {
		g.By("Get pod (router) in openshift-ingress namespace")
		podname := getRouterPod(oc, "default")

		g.By("Check if there are default 404 and 503 error pages on the router")
		searchOutput := readRouterPodData(oc, podname, "ls -l", "error-page")
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-404.http`))
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-503.http`))

		g.By("Check if errorfile stanzas have been added into haproxy-config.template")
		searchOutput = readRouterPodData(oc, podname, "cat haproxy-config.template", "errorfile")
		o.Expect(searchOutput).To(o.ContainSubstring(`ROUTER_ERRORFILE_404`))
		o.Expect(searchOutput).To(o.ContainSubstring(`ROUTER_ERRORFILE_503`))
	})

	//author: jechen@redhat.com
	g.It("Author:jechen-High-43115-Configmap mounted on router volume after ingresscontroller has spec field HttpErrorCodePage populated with configmap name", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43115",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("1. create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		ingctrl.create(oc)
		defer ingctrl.delete(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		originalRouterpod := getRouterPod(oc, ingctrl.name)

		g.By("2.  Configure a customized error page configmap from files in openshift-config namespace")
		configmapName := "custom-43115-error-code-pages"
		cmFile1 := filepath.Join(buildPruningBaseDir, "error-page-503.http")
		cmFile2 := filepath.Join(buildPruningBaseDir, "error-page-404.http")
		_, error := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", configmapName, "--from-file="+cmFile1, "--from-file="+cmFile2, "-n", "openshift-config").Output()
		o.Expect(error).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("configmap", configmapName, "-n", "openshift-config").Output()

		g.By("3. Check if configmap is successfully configured in openshift-config namesapce")
		err = checkConfigMap(oc, "openshift-config", configmapName)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4. Patch the configmap created above to the custom ingresscontroller in openshift-ingress namespace")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"httpErrorCodePages\":{\"name\":\"custom-43115-error-code-pages\"}}}")

		g.By("5. Check if configmap is successfully patched into openshift-ingress namesapce, configmap with name ingctrl.name-errorpages should be created")
		expectedCmName := ingctrl.name + `-errorpages`
		err = checkConfigMap(oc, "openshift-ingress", expectedCmName)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("6. Obtain new router pod created, and check if error_code_pages directory is created on it")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+originalRouterpod)
		o.Expect(err).NotTo(o.HaveOccurred())
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("Check /var/lib/haproxy/conf directory to see if error_code_pages subdirectory is created on the router")
		searchOutput := readRouterPodData(oc, newrouterpod, "ls -al /var/lib/haproxy/conf", "error_code_pages")
		o.Expect(searchOutput).To(o.ContainSubstring(`error_code_pages`))

		g.By("7. Check if custom error code pages have been mounted")
		searchOutput = readRouterPodData(oc, newrouterpod, "ls -al /var/lib/haproxy/conf/error_code_pages", "error")
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-503.http -> ..data/error-page-503.http`))
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-404.http -> ..data/error-page-404.http`))

		searchOutput = readRouterPodData(oc, newrouterpod, "cat /var/lib/haproxy/conf/error_code_pages/error-page-503.http", "Unavailable")
		o.Expect(searchOutput).To(o.ContainSubstring(`HTTP/1.0 503 Service Unavailable`))
		o.Expect(searchOutput).To(o.ContainSubstring(`Custom:Application Unavailable`))

		searchOutput = readRouterPodData(oc, newrouterpod, "cat /var/lib/haproxy/conf/error_code_pages/error-page-404.http", "Not Found")
		o.Expect(searchOutput).To(o.ContainSubstring(`HTTP/1.0 404 Not Found`))
		o.Expect(searchOutput).To(o.ContainSubstring(`Custom:Not Found`))

	})

})
