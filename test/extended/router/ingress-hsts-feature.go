package router

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	//e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("router-hsts", exutil.KubeConfigPath())

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43476-The PreloadPolicy option can be set to be enforced strictly to be present or absent in HSTS preload header checks [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		edgecert := filepath.Join(buildPruningBaseDir, "edge-route.pem")
		edgekey := filepath.Join(buildPruningBaseDir, "edge-route.key")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43476",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)
		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeEdgeRoute(oc, oc.Namespace(), "route-edge", "service-unsecure", edgecert, edgekey, routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload enforced to be absent")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\" , \"maxAge\":{}, \"preloadPolicy\" :\"RequireNoPreload\"}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"RequireNoPreload\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequireNoPreload"))

		g.By("Annotate the edge route with preload option to verify the effect")
		output1, err2 := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;preload", "--overwrite").Output()
		o.Expect(err2).To(o.HaveOccurred())
		o.Expect(output1).To(o.ContainSubstring("HSTS preload must not be specified"))

		g.By("Add the HSTS policy to global ingresses resource with preload enforced to be present")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"RequirePreload\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequirePreload"))

		g.By("verify the enforced policy by overwriting the route annotation to disable Preload headers")
		msg2, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS preload must be specified"))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43478-The PreloadPolicy option can be configured to be permissive with NoOpinion flag [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		edgecert := filepath.Join(buildPruningBaseDir, "edge-route.pem")
		edgekey := filepath.Join(buildPruningBaseDir, "edge-route.key")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43478",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)
		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routedomain := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeEdgeRoute(oc, oc.Namespace(), "route-edge", "service-unsecure", edgecert, edgekey, routedomain)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload option set to NoOpinion")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\" , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("NoOpinion"))

		g.By("Annotate the edge route with preload option to verify")
		_, err2 := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;preload", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())

		g.By("Annotate the edge route without preload option to verify")
		_, err2 = oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43474-The includeSubDomainsPolicy parameter can configure subdomain policy to inherit the HSTS policy of parent domain [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		edgecert := filepath.Join(buildPruningBaseDir, "edge-route.pem")
		edgekey := filepath.Join(buildPruningBaseDir, "edge-route.key")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43474",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)
		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeEdgeRoute(oc, oc.Namespace(), "route-edge", "service-unsecure", edgecert, edgekey, routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with IncludeSubdomain enforced to be absent")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireNoIncludeSubDomains\" , \"maxAge\":{}}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireNoIncludeSubDomains\", \"maxAge\":{}}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequireNoIncludeSubDomains"))

		g.By("Annotate the edge route with preload option to verify the effect")
		output1, err2 := oc.Run("annotate").Args("-n", oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;includeSubDomains", "--overwrite").Output()
		o.Expect(err2).To(o.HaveOccurred())
		o.Expect(output1).To(o.ContainSubstring("HSTS includeSubDomains must not be specified"))

		g.By("Add the HSTS policy to global ingresses resource with IncludeSubdomain enforced to be present")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequireIncludeSubDomains"))

		g.By("verify the enforced policy by overwriting the route annotation to disable Preload headers")
		msg2, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS includeSubDomains must be specified"))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43475-The includeSubDomainsPolicy option can be configured to be permissive with NoOpinion flag [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		edgecert := filepath.Join(buildPruningBaseDir, "edge-route.pem")
		edgekey := filepath.Join(buildPruningBaseDir, "edge-route.key")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43475",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)
		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeEdgeRoute(oc, oc.Namespace(), "route-edge", "service-unsecure", edgecert, edgekey, routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload option set to NoOpinion")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\" , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("NoOpinion"))

		g.By("Annotate the edge route with preload option to verify")
		_, err2 := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;preload", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())

		g.By("Annotate the edge route without preload option to verify")
		_, err2 = oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43479-The Maxage HSTS policy strictly adheres to validation of route based based on largestMaxAge and smallestMaxAge parameter [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		edgecert := filepath.Join(buildPruningBaseDir, "edge-route.pem")
		edgekey := filepath.Join(buildPruningBaseDir, "edge-route.key")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43479",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)
		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "pod failed to be ready state within allowed time!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeEdgeRoute(oc, oc.Namespace(), "route-edge", "service-unsecure", edgecert, edgekey, routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload option set to maxAge with lowest and highest timer option")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{\"largestMaxAge\": 40000, \"smallestMaxAge\": 100 }}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{\"largestMaxAge\": 40000, \"smallestMaxAge\": 100 }}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("largestMaxAge"))

		g.By("verify the enforced policy by overwriting the route annotation with largestMaxAge set higher than globally defined")
		msg2, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS max-age is greater than maximum age 40000s"))

		g.By("verify the enforced policy by overwriting the route annotation with largestMaxAge set lower than globally defined")
		msg2, err = oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS max-age is less than minimum age 100s"))

	})
})
