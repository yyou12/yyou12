package clusterinfrastructure

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"path/filepath"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc                        = exutil.NewCLI("cluster-autoscaler-operator", exutil.KubeConfigPath())
		autoscalerBaseDir         string
		clusterAutoscalerTemplate string
		clusterAutoscaler         clusterAutoscalerDescription
	)

	g.BeforeEach(func() {
		autoscalerBaseDir = exutil.FixturePath("testdata", "clusterinfrastructure", "autoscaler")
		clusterAutoscalerTemplate = filepath.Join(autoscalerBaseDir, "clusterautoscaler.yaml")
		clusterAutoscaler = clusterAutoscalerDescription{
			maxNode:   100,
			minCore:   0,
			maxCore:   320000,
			minMemory: 0,
			maxMemory: 6400000,
			template:  clusterAutoscalerTemplate,
		}
	})

	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-43174-ClusterAutoscaler CR could be deleted with foreground deletion", func() {
		g.By("Create clusterautoscaler")
		clusterAutoscaler.createClusterAutoscaler(oc)
		defer clusterAutoscaler.deleteClusterAutoscaler(oc)
		g.By("Delete clusterautoscaler with foreground deletion")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterautoscaler", "default", "--cascade=foreground").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterautoscaler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("default"))
	})
})
