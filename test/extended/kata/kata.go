//Kata operator tests
package kata

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-kata] Kata", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLI("kata", exutil.KubeConfigPath())
		opNamespace          = "openshift-sandboxed-containers-operator"
		commonKataConfigName = "example-kataconfig" 
		// Team - for specific kataconfig and pod, please define and create them in g.It.
		testDataDir  = exutil.FixturePath("testdata", "kata")
		iaasPlatform string
		
	)

	g.BeforeEach(func() {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		iaasPlatform = strings.ToLower(output)
		e2e.Logf("the current platform is %v", iaasPlatform)
		ns := filepath.Join(testDataDir, "namespace.yaml")
		og := filepath.Join(testDataDir, "operatorgroup.yaml")
		sub := filepath.Join(testDataDir, "subscription.yaml")
		commonKc := filepath.Join(testDataDir, "kataconfig.yaml")
		

		createIfNoOperator(oc, opNamespace, ns, og, sub)
		createIfNoKataConfig(oc, opNamespace, commonKc, commonKataConfigName)

	})
	// author: abhbaner@redhat.com
	g.It("Author:abhbaner-High-39499-Operator installation", func() {
		g.By("Checking sandboxed-operator operator installation")
		e2e.Logf("Operator install check successfull as part of setup !!!!!")
		g.By("SUCCESSS - sandboxed-operator operator installed")

	})

	g.It("Author:abhbaner-High-43522-Common Kataconfig installation", func() {
		g.By("Install Common kataconfig and verify it")
		e2e.Logf("common kataconfig %v is installed", commonKataConfigName)
		g.By("SUCCESSS - kataconfig installed")

	})

	g.It("Author:abhbaner-High-41566-deploy a pod with kata runtime", func() {
		commonPodName :="example" 
		commonPod := filepath.Join(testDataDir, "example.yaml")
		
		oc.SetupProject()
		podNs := oc.Namespace()	

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc,podNs,commonPod,commonPodName)
		defer deleteKataPod(oc,podNs,newPodName)
		checkKataPodStatus(oc,podNs,newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)
		g.By("SUCCESSS - Pod with kata runtime installed")
  	        g.By("TEARDOWN - deleting the kata pod")
	})  

    
})




