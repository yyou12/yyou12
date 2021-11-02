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
	
	g.It("Longduration-Author:liqcui-Medium-43365-SRO Build and run SpecialResource ping-pong resource with SRO from CLI [Slow]", func() {

		g.By("Cleanup special resource ping-pong application default objects")
		//ping-pong example application contains ping-pong and cert-manager
		pingPongAppRes := oprResource{
			kind:      "SpecialResource",
			name:      "ping-pong",
			namespace: "",
		}
		certManagerAppRes := oprResource{
			kind:      "SpecialResource",
			name:      "cert-manager",
			namespace: "",
		}
		defer pingPongAppRes.CleanupResource(oc)
		defer certManagerAppRes.CleanupResource(oc)

		//create cluster-wide SpecialResource ping-pong and cert-manager via yaml file
		//No need to specify kind,name and namespace

		g.By("Create Ping-Pong and Cert Manager Application")
		pingPongYaml := filepath.Join(sroDir, "sro-ping-pong.yaml")
		pingPong := oprResource{
			kind:      "",
			name:      "",
			namespace: "",
		}
		pingPong.applyResourceByYaml(oc, pingPongYaml)

		//Check ping-pong server and client pods status
		g.By("SRO - Verfiy the result for SRO test case 43365")
		g.By("SRO - Check ping-pong application pod status")
		pingPongServerPod := oprResource{
			kind:      "deployment",
			name:      "ping-pong-server",
			namespace: "ping-pong",
		}
		pingPongServerPod.waitOprResourceReady(oc)

		pingPongClientPod := oprResource{
			kind:      "deployment",
			name:      "ping-pong-client",
			namespace: "ping-pong",
		}
		pingPongClientPod.waitOprResourceReady(oc)

		g.By("SRO - Check cert-manager application pod status")
		//Check cert-manager pods status
		certManagerPod := oprResource{
			kind:      "deployment",
			name:      "cert-manager",
			namespace: "cert-manager",
		}
		certManagerPod.waitOprResourceReady(oc)

		certManagerCainjectorPod := oprResource{
			kind:      "deployment",
			name:      "cert-manager-cainjector",
			namespace: "cert-manager",
		}
		certManagerCainjectorPod.waitOprResourceReady(oc)

		certManagerWebhookPOD := oprResource{
			kind:      "deployment",
			name:      "cert-manager-webhook",
			namespace: "cert-manager",
		}
		certManagerWebhookPOD.waitOprResourceReady(oc)

		g.By("SRO - Check ping-pong application logs")
		//Check if ping-pong application logs normally
		pingPongServerPod.assertOprPodLogs(oc, "Ping")
		pingPongServerPod.assertOprPodLogs(oc, "Pong")
		pingPongClientPod.assertOprPodLogs(oc, "Ping")
		pingPongClientPod.assertOprPodLogs(oc, "Pong")
	})
})
