package scheduler

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-scheduling] Workloads", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")
	var kz = "zone"
	var kn = "node"

	// author: yinzhou@redhat.com
	g.It("Critical-33836-Critical-33845-High-33767-Check Validate Pod with only one TopologySpreadConstraint topologyKey node", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "scheduler")
		podSelectorT := filepath.Join(buildPruningBaseDir, "pod_nodeselector.yaml")
		podSinglePtsT := filepath.Join(buildPruningBaseDir, "pod_single_pts.yaml")
		podSinglePtsNodeSelectorT := filepath.Join(buildPruningBaseDir, "pod_single_pts_nodeselector.yaml")

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		g.By("Apply dedicated Key for this test on the 3 nodes.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz, "zoneA")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn, "node1")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz, "zoneA")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn, "node2")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz, "zoneB")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn, "node3")
		g.By("Remove dedicated Key for this test on the 3 nodes.")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn)

		g.By("Test for case OCP-33836")
		g.By("create new namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-pts-33836").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-pts-33836").Execute()

		pod1 := podNodeSelector{
			name:       "mypod1-33836",
			namespace:  "test-pts-33836",
			nodeName:   "node1",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod2 := podNodeSelector{
			name:       "mypod2-33836",
			namespace:  "test-pts-33836",
			nodeName:   "node2",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod3 := podSinglePts{
			name:       "mypod3-33836",
			namespace:  "test-pts-33836",
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "node",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    1,
			template:   podSinglePtsT,
		}
		g.By("Trying to launch a pod with a label to node1")
		pod1.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod1nodename := pod1.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[0].Name, pod1nodename)

		g.By("Trying to launch a pod with a label to node2")
		pod2.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod2nodename := pod2.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[1].Name, pod2nodename)

		g.By("In this case, the new coming pod only scheduler to node3")
		pod3.createPodSinglePts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod3nodename := pod3.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[2].Name, pod3nodename)

		g.By("Test for case OCP-33845")
		g.By("create new namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-pts-33845").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-pts-33845").Execute()

		pod338451 := podNodeSelector{
			name:       "mypod1-33845",
			namespace:  "test-pts-33845",
			nodeName:   "node1",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod338452 := podNodeSelector{
			name:       "mypod2-33845",
			namespace:  "test-pts-33845",
			nodeName:   "node2",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod338453 := podNodeSelector{
			name:       "mypod3-33845",
			namespace:  "test-pts-33845",
			nodeName:   "node3",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod338454 := podSinglePts{
			name:       "mypod4-33845",
			namespace:  "test-pts-33845",
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "zone",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    2,
			template:   podSinglePtsT,
		}

		g.By("Trying to launch a pod with a label to node1")
		pod338451.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338451nodename := pod338451.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[0].Name, pod338451nodename)

		g.By("Trying to launch a pod with a label to node2")
		pod338452.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338452nodename := pod338452.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[1].Name, pod338452nodename)

		g.By("Trying to launch a pod with a label to node3")
		pod338453.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338453nodename := pod338453.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[2].Name, pod338453nodename)

		g.By("In this case, the new coming pod could scheduler to node1-node3")
		pod338454.createPodSinglePts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338454nodename := pod338454.getPodNodeName(oc)
		o.Expect(pod338454nodename).Should(o.BeElementOf([]string{nodeList.Items[0].Name, nodeList.Items[1].Name, nodeList.Items[2].Name}))

		g.By("Test for case OCP-33767")
		g.By("create new namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-pts-33767").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-pts-33767").Execute()

		pod337671 := podSinglePtsNodeSelector{
			name:       "mypod1-33767",
			namespace:  "test-pts-33767",
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "node",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    1,
			nodeKey:    "zone",
			nodeValue:  "zoneA",
			template:   podSinglePtsNodeSelectorT,
		}

		pod337672 := podSinglePtsNodeSelector{
			name:       "mypod2-33767",
			namespace:  "test-pts-33767",
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "node",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    1,
			nodeKey:    "zone",
			nodeValue:  "zoneA",
			template:   podSinglePtsNodeSelectorT,
		}

		g.By("Trying to launch a pod with a label to zoneA node1 or node2")
		pod337671.createPodSinglePtsNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod337671nodename := pod337671.getPodNodeName(oc)
		o.Expect(pod337671nodename).Should(o.BeElementOf([]string{nodeList.Items[0].Name, nodeList.Items[1].Name}))

		g.By("In this case, the new coming pod could scheduler to zoneA,but not same node with pod337671")
		pod337672.createPodSinglePtsNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod337672nodename := pod337672.getPodNodeName(oc)
		o.Expect(pod337672nodename).Should(o.BeElementOf([]string{nodeList.Items[0].Name, nodeList.Items[1].Name}))
		o.Expect(pod337672nodename).NotTo(o.Equal(pod337671nodename))
	})
})
