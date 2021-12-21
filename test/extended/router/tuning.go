package router

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("router-env-nbthread", exutil.KubeConfigPath())
	// author: shudili@redhat.com
	g.It("Author:shudili-Critical-41110-The threadCount ingresscontroller parameter controls the nbthread option for the haproxy router", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp41110",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
			threadcount = "6"
		)

		g.By("Create a ingresscontroller with threadCount set")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Patch the new ingresscontroller with tuningOptions/threadCount "+threadcount)
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		podname := getRouterPod(oc, ingctrl.name)
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\": {\"tuningOptions\": {\"threadCount\": "+threadcount+"}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+podname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+podname))

		g.By("Check the router env to verify the PROXY variable ROUTER_THREADS with "+threadcount+" is applied")
		newpodname := getRouterPod(oc, ingctrl.name)
		dssearch := readRouterPodEnv(oc, newpodname, "ROUTER_THREADS")
		o.Expect(dssearch).To(o.ContainSubstring("ROUTER_THREADS="+threadcount))
		g.By("check the haproxy config on the router pod to ensure the nbthread is updated")

		nbthread := readRouterPodData(oc, newpodname, "cat haproxy.config", "nbthread")
		o.Expect(nbthread).To(o.ContainSubstring("nbthread "+threadcount))

	})
	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-Low-41128-Ingresscontroller should not accept invalid nbthread setting", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp41128",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
			threadcount_default = "4"
			threadcount1        = "-1"
			threadcount2        = "512"
			threadcount3        = `"abc"`
		)

		g.By("create a custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Patch the new ingresscontroller with negative(" + threadcount1 + ") value as threadCount")
		output1, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"ingresscontroller/"+ingctrl.name, "-p", "{\"spec\": {\"tuningOptions\": {\"threadCount\": "+threadcount1+"}}}",
			"--type=merge", "-n", ingctrl.namespace).Output()
		o.Expect(output1).To(o.ContainSubstring("Invalid value: -1: spec.tuningOptions.threadCount in body should be greater than or equal to 1"))

		g.By("Patch the new ingresscontroller with high(" + threadcount2 + ") value for threadCount")
		output2, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"ingresscontroller/"+ingctrl.name, "-p", "{\"spec\": {\"tuningOptions\": {\"threadCount\": "+threadcount2+"}}}",
			"--type=merge", "-n", ingctrl.namespace).Output()
		o.Expect(output2).To(o.ContainSubstring("Invalid value: 512: spec.tuningOptions.threadCount in body should be less than or equal to 64"))

		g.By("Patch the new ingresscontroller with string(" + threadcount3 + ") value for threadCount")
		output3, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"ingresscontroller/"+ingctrl.name, "-p", "{\"spec\": {\"tuningOptions\": {\"threadCount\": "+threadcount3+"}}}",
			"--type=merge", "-n", ingctrl.namespace).Output()
		o.Expect(output3).To(o.ContainSubstring(`Invalid value: "string": spec.tuningOptions.threadCount in body must be of type integer: "string"`))

		g.By("Check the router env to verify the default value of ROUTER_THREADS is applied")
		podname := getRouterPod(oc, ingctrl.name)
		thread_value := readRouterPodEnv(oc, podname, "ROUTER_THREADS")
		o.Expect(thread_value).To(o.ContainSubstring("ROUTER_THREADS=" + threadcount_default))
	})
})
