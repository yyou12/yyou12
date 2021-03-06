//Kata operator tests
package kata

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
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

	g.It("Author:abhbaner-High-41566-High-41574-deploy & delete a pod with kata runtime", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)
		checkKataPodStatus(oc, podNs, newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)
		g.By("SUCCESS - Pod with kata runtime installed")
		g.By("TEARDOWN - deleting the kata pod")
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-High-43238-Operator prohibits creation of multiple kataconfigs", func() {
		var (
			kataConfigName2 = commonKataConfigName + "2"
			configFile      string
			msg             string
			err             error
			kcTemplate      = filepath.Join(testDataDir, "kataconfig.yaml")
		)
		g.By("Create 2nd kataconfig file")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", kcTemplate, "-p", "NAME="+kataConfigName2).OutputToFile(getRandomString() + "kataconfig-common.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the file of resource is %s", configFile)

		g.By("Apply 2nd kataconfig")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Output()
		o.Expect(msg).To(o.ContainSubstring("KataConfig instance already exists"))
		e2e.Logf("err %v, msg %v", err, msg)

		g.By("Success - cannot apply 2nd kataconfig")

	})

	g.It("Author:abhbaner-High-41263-Namespace check", func() {
		g.By("Checking if ns 'openshift-sandboxed-containers-operator' exists")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring(opNamespace))
		g.By("SUCCESS - Namespace check complete")

	})

	g.It("Author:abhbaner-High-43620-validate podmetrics for pod running kata", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)
		checkKataPodStatus(oc, podNs, newPodName)

		errCheck := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			podMetrics, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("podmetrics", newPodName, "-n", podNs).Output()
			if err != nil {
				e2e.Logf("error  %v, please try next round", err)
				return false, nil
			}
			e2e.Logf("Pod metrics output below  \n %s ", podMetrics)
			o.Expect(podMetrics).To(o.ContainSubstring("Cpu"))
			o.Expect(podMetrics).To(o.ContainSubstring("Memory"))
			o.Expect(podMetrics).To(o.ContainSubstring("Events"))
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("can not describe podmetrics %v in ns %v", newPodName, podNs))
		g.By("SUCCESS - Podmetrics for pod with kata runtime validated")
		g.By("TEARDOWN - deleting the kata pod")
	})

	g.It("Author:abhbaner-High-43617-High-43616-CLI checks pod logs & fetching pods in podNs", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)

		/* checkKataPodStatus prints the pods with the podNs and validates if
		its running or not thus verifying OCP-43616 */

		checkKataPodStatus(oc, podNs, newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)

		podlogs, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+newPodName, "-n", podNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podlogs).NotTo(o.BeEmpty())
		o.Expect(podlogs).To(o.ContainSubstring("httpd"))
		g.By("SUCCESS - Logs for pods with kata validated")
		g.By("TEARDOWN - deleting the kata pod")
	})

	g.It("Author:abhbaner-High-43514-kata pod displaying correct overhead", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)
		checkKataPodStatus(oc, podNs, newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)

		g.By("Checking Pod Overhead")
		podoverhead, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("runtimeclass", "kata").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podoverhead).NotTo(o.BeEmpty())
		o.Expect(podoverhead).To(o.ContainSubstring("Overhead"))
		o.Expect(podoverhead).To(o.ContainSubstring("Cpu"))
		o.Expect(podoverhead).To(o.ContainSubstring("Memory"))
		g.By("SUCCESS - kata pod overhead verified")
		g.By("TEARDOWN - deleting the kata pod")
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-High-43619-oc admin top pod works for pods that use kata runtime", func() {

		oc.SetupProject()
		var (
			commonPodTemplate = filepath.Join(testDataDir, "example.yaml")
			podNs             = oc.Namespace()
			podName           string
			err               error
			msg               string
			waitErr           error
			metricCount       = 0
		)

		g.By("Deploy a pod with kata runtime")
		podName = createKataPod(oc, podNs, commonPodTemplate, "admtop")
		defer deleteKataPod(oc, podNs, podName)
		checkKataPodStatus(oc, podNs, podName)

		g.By("Get oc top adm metrics for the pod")
		snooze = 360
		waitErr = wait.Poll(10*time.Second, snooze*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "pod", "-n", podNs, podName, "--no-headers").Output()
			if err == nil { // Will get error with msg: error: metrics not available yet
				metricCount = len(strings.Fields(msg))
			}
			if metricCount == 3 {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "metrics never appeared")
		if metricCount == 3 {
			e2e.Logf("metrics for pod %v", msg)
		}
		o.Expect(metricCount).To(o.Equal(3))

		g.By("Success")

	})
	
	g.It("Author:abhbaner-High-43516-operator is available in CatalogSource"    , func() {
        
        g.By("Checking catalog source for the operator")
        opMarketplace,err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests", "-n", "openshift-marketplace").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(opMarketplace).NotTo(o.BeEmpty())
        o.Expect(opMarketplace).To(o.ContainSubstring("sandboxed-containers-operator"))
        o.Expect(opMarketplace).To(o.ContainSubstring("Red Hat Operators"))
        g.By("SUCCESS -  'sandboxed-containers-operator' is present in packagemanifests")
        
    })

})
