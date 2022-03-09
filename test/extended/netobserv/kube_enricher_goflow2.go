package netobserv

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-netobserv] Network_Observability", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("netobserv", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		networkType := exutil.CheckNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Network type is not ovn, skip for non-supported network type!!!")
		}
	})

	// author: jechen@redhat.com
	g.It("Author:jechen-High-45304-Kube-enricher uses goflow2 as collector for network flow [Serial]", func() {
		goflowDeploymentTemplate := filepath.Join(exutil.FixturePath("testdata", "netobserv"), "templatized_flows_v1alpha1_flowcollector.yaml")

		var (
			goflowkube = goflowKubeDescription{
				serviceNs: "network-observability",
				name:      "goflow-kube",
				cmname:    "goflow-kube-config",
				template:  goflowDeploymentTemplate,
			}
		)
		g.By("1. create new namespace")
		oc.SetupProject()

		g.By("2. Create goflow-kube deployment")
		goflowkube.create(oc, oc.Namespace(), goflowkube.template)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("flowCollector", "cluster").Execute()

		g.By("3. Enable Network Observability plugin")
		change := "[{\"op\":\"add\", \"path\":\"/spec/plugins\", \"value\":[\"network-observability-plugin\"]}]"
		patchResourceAsAdmin(oc, oc.Namespace(), "console.operator.openshift.io", "cluster", change)
		recovery := "[{\"op\":\"remove\", \"path\":\"/spec/plugins\"}]"
		defer patchResourceAsAdmin(oc, oc.Namespace(), "console.operator.openshift.io", "cluster", recovery)

		g.By("4. Verify goflow collector is added")
		output := getGoflowCollector(oc, "flowCollector")
		o.Expect(output).To(o.ContainSubstring("cluster"))

		g.By("5. Wait for goflow-kube pod be in running state")
		waitPodReady(oc, oc.Namespace(), goflowkube.name)

		g.By("5. Get goflow pod, check the goflow pod logs and verify that flows are recorded")
		podname := getGoflowPod(oc, oc.Namespace(), goflowkube.name)
		podLogs, err := exutil.WaitAndGetSpecificPodLogs(oc, oc.Namespace(), "", podname, "BiFlowDirection")
		exutil.AssertWaitPollNoErr(err, "Did not get log for the pod with app=goflow-kube label")
		verifyFlowRecord(podLogs)
	})
})
