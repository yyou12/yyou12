package router

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	//e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("router-ipfailover", exutil.KubeConfigPath())

	// author: hongli@redhat.com
	// might conflict with other ipfailover cases so set it as Serial
	g.It("Author:hongli-ConnectedOnly-Critical-41025-support to deploy ipfailover", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ipfailover.yaml")
		var (
			ipf = ipfailoverDescription{
				name:      "ipf-41025",
				namespace: "",
				image:     "",
				template:  customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		oc.SetupProject()
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, oc.Namespace())
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
