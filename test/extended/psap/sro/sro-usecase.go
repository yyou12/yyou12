package sro

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] PSAP SRO should", func() {
	defer g.GinkgoRecover()

	var (
		oc     = exutil.NewCLI("sro-cli-test", exutil.KubeConfigPath())
		isNFD  bool
		sroDir = exutil.FixturePath("testdata", "psap", "sro")
	)

	g.BeforeEach(func() {
		// ensure NFD operator is installed
		isNFD = checkIfNFDInstalled(oc)
		if !isNFD {
			g.Skip("NFD is not installed - skipping test ...")
		}

		//g.By("SRO - Get Current Clusterversion")
		//exutil.GetClusterVersion(oc)
		//Create Special Resource if Not Exist
		g.By("SRO - Create Namespace for SRO")
		nsTemplate := filepath.Join(sroDir, "sro-ns.yaml")
		ns := nsResource{
			name:     "openshift-special-resource-operator",
			template: nsTemplate,
		}
		ns.createIfNotExist(oc)

		g.By("SRO - Create Operator Group for SRO")
		ogTemplate := filepath.Join(sroDir, "sro-og.yaml")
		og := ogResource{
			name:      "openshift-special-resource-operator",
			namespace: "openshift-special-resource-operator",
			template:  ogTemplate,
		}
		og.createIfNotExist(oc)

		g.By("SRO - Create Subscription for SRO")
		//Get default channnel version of packagemanifest
		pkgminfo := pkgmanifestinfo{
			pkgmanifestname: "openshift-special-resource-operator",
			namespace:       "openshift-special-resource-operator",
		}
		channelv, _ := pkgminfo.getDefaultChannelVersion(oc)
		e2e.Logf("The default channel version of packagemanifest is %v\n", channelv)
		sroSource, _ := pkgminfo.getPKGManifestSource(oc)
		e2e.Logf("The catalog source of packagemanifest is %v\n", sroSource)

		subTemplate := filepath.Join(sroDir, "sro-sub.yaml")
		sub := subResource{
			name:      "openshift-special-resource-operator",
			namespace: "openshift-special-resource-operator",
			channel:   channelv,
			template:  subTemplate,
			source:    sroSource,
		}
		sub.createIfNotExist(oc)

		g.By("SRO - Verfiy the result for SRO test case")
		sroRes := oprResource{
			kind:      "deployment",
			name:      "special-resource-controller-manager",
			namespace: "openshift-special-resource-operator",
		}
		g.By("SRO - Check if SRO Operator is Ready")
		sroRes.waitOprResourceReady(oc)

	})
	// author: liqcui@redhat.com
	g.It("Longduration-Author:liqcui-Medium-43058-SRO Build and run the simple-kmod SpecialResource using the SRO image's local manifests [Slow]", func() {

		simpleKmodPodRes := oprResource{
			kind:      "pod",
			name:      "simple-kmod",
			namespace: "simple-kmod",
		}

		//Check if the simple kmod pods has already created in simple-kmod namespace
		hasSimpleKmod := simpleKmodPodRes.checkOperatorPOD(oc)
		//Cleanup cluster-wide SpecialResource simple-kmod
		simpleKmodSRORes := oprResource{
			kind:      "SpecialResource",
			name:      "simple-kmod",
			namespace: "",
		}
		defer simpleKmodSRORes.CleanupResource(oc)
		//If no simple-kmod pod, it will create a SpecialResource simple-kmod, the SpecialResource
		//will create ns and daemonset in namespace simple-kmod, and install simple-kmod kernel on
		//worker node
		if !hasSimpleKmod {
			sroSimpleKmodYaml := filepath.Join(sroDir, "sro-simple-kmod.yaml")
			g.By("Create Simple Kmod Application")
			//Create an empty opr resource, it's a cluster-wide resource, no namespace
			simpleKmodSRORes.applyResourceByYaml(oc, sroSimpleKmodYaml)
		}

		//Check if simple-kmod pod is ready
		g.By("SRO - Check the result for SRO test case 43058")
		simpleKmodDaemonset := oprResource{
			kind:      "daemonset",
			name:      "simple-kmod",
			namespace: "simple-kmod",
		}
		simpleKmodDaemonset.waitOprResourceReady(oc)

		//Check is the simple-kmod kernel installed on worker node
		assertSimpleKmodeOnNode(oc)
	})
})
