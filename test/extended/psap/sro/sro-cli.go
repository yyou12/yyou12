package sro

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	ci "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] PSAP should", func() {
	defer g.GinkgoRecover()

	var (
		oc                  = exutil.NewCLI("sro-cli-test", exutil.KubeConfigPath())
		machineNFDNamespace = "openshift-nfd"
		iaasPlatform        string
		isNFD               bool
	)

	g.BeforeEach(func() {
		// get IaaS platform
		iaasPlatform = ci.CheckPlatform(oc)

		// ensure NFD operator is installed
		isNFD = isNFDInstalled(oc, machineNFDNamespace)
	})

	// author: liqcui@redhat.com
	g.It("Author:liqcui-Medium-43325-Special Resource Operator - Deploy SRO without GPU from OCP CLI", func() {
		// test requires NFD to be installed
		if !isNFD {
			g.Skip("NFD is not installed - skipping test ...")
		}

		// currently test is only supported on AWS, GCP, and Azure
		if iaasPlatform != "aws" && iaasPlatform != "gcp" && iaasPlatform != "azure" {
			g.Skip("IAAS platform: " + iaasPlatform + " is not automated yet - skipping test ...")
		}
		sroDir := exutil.FixturePath("testdata", "psap", "sro")

		g.By("SRO - Get current cluster version")
		output, err := oc.AsAdmin().Run("get").Args("clusterversion").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("clucsterversion is %v", output)

		g.By("SRO - Create Namespace for SRO")
		nsTemplate := filepath.Join(sroDir, "sro-ns.yaml")
		ns := nsResource{
			name:     "openshift-special-resource-operator",
			template: nsTemplate,
		}
		defer ns.delete(oc)
		ns.createIfNotExist(oc)

		g.By("SRO - Create Operator Group for SRO")
		ogTemplate := filepath.Join(sroDir, "sro-og.yaml")
		og := ogResource{
			name:      "openshift-special-resource-operator",
			namespace: "openshift-special-resource-operator",
			template:  ogTemplate,
		}
		defer og.delete(oc)
		og.createIfNotExist(oc)

		g.By("SRO - Create Subscription for SRO")
		//Get default channnel version of packagemanifest
		pkgminfo := pkgmanifestinfo{
			pkgmanifestname: "openshift-special-resource-operator",
			namespace:       "openshift-special-resource-operator",
		}
		channelv, err := pkgminfo.getDefaultChannelVersion(oc)
		fmt.Printf("The default channel version of packagemanifest is %v\n", channelv)
		sroSource, err := pkgminfo.getPKGManifestSource(oc)
		fmt.Printf("The catalog source of packagemanifest is %v\n", sroSource)

		subTemplate := filepath.Join(sroDir, "sro-sub.yaml")
		sub := subResource{
			name:      "openshift-special-resource-operator",
			namespace: "openshift-special-resource-operator",
			channel:   channelv,
			template:  subTemplate,
			source:    sroSource,
		}
		defer sub.delete(oc)
		sub.createIfNotExist(oc)

		g.By("SRO - Verfiy the result for SRO test case")
		waitErr := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("deployment", "special-resource-controller-manager", "-n", sub.namespace, "-o=jsonpath={.status.conditions}").Output()
			if err != nil {
				e2e.Logf("the err:%v, and try next round", err)
				return false, nil
			}
			e2e.Logf("the deployment status is %v", output)
			if strings.Contains(output, "has successfully progressed") {
				return true, nil
			} else {
				return false, nil
			}
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("the pod of sub %v is not running", sub.name))

	})
})
