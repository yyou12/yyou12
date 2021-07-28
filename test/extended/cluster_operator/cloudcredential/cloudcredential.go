package cloudcredential

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cco] Cluster_Operator CCO should", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("default-cco", exutil.KubeConfigPath())
		modeInMetric string
	)

	// author: lwan@redhat.com
	// It is destructive case, will remove root credentials, so adding [Disruptive]. The case duration is greater than 5 minutes
	// so adding [Slow]
	g.It("Author:lwan-Hign-31768-Report the mode of cloud-credential operation as a metric [Slow][Disruptive]", func() {
		g.By("Check if the current platform is a supported platform")
		rootSecretName, err := GetRootSecretName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if rootSecretName == "" {
			e2e.Logf("unsupported platform, there is no root credential in kube-system namespace,  will pass the test")
		} else {
			g.By("Check if cco mode in metric is the same as cco mode in cluster resources")
			g.By("Get cco mode from Cluster Resource")
			modeInCR, err := GetCloudCredentialMode(oc)
			e2e.Logf("cco mode in cluster CR is %v", modeInCR)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Check if cco mode in Metric is correct")
			err = CheckModeInMetric(oc, modeInCR)
			if err != nil {
				e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
			}
			if modeInCR == "mint" {
				g.By("if cco is in mint mode currently, then run the below test")
				g.By("Check cco mode when cco is in Passathrough mode")
				e2e.Logf("Force cco mode to Passthrough")
				originCCOMode, err := oc.AsAdmin().Run("get").Args("cloudcredential/cluster", "-o=jsonpath={.spec.credentialsMode}").Output()
				if originCCOMode == "" {
					originCCOMode = "\"\""
				}
				patchYaml := `
spec:
  credentialsMode: ` + originCCOMode
				err = oc.AsAdmin().Run("patch").Args("cloudcredential/cluster", "-p", `{"spec":{"credentialsMode":"Passthrough"}}`, "--type=merge").Execute()
				defer func() {
					err := oc.AsAdmin().Run("patch").Args("cloudcredential/cluster", "-p", patchYaml, "--type=merge").Execute()
					err = CheckModeInMetric(oc, modeInCR)
					if err != nil {
						e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
					}
				}()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from cluster CR")
				modeInCR, err := GetCloudCredentialMode(oc)
				e2e.Logf("cco mode in cluster CR is %v", modeInCR)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Check if cco mode in Metric is correct")
				err = CheckModeInMetric(oc, modeInCR)
				if err != nil {
					e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
				}
				g.By("Check cco mode when root credential is removed when cco is not in manual mode")
				e2e.Logf("remove root creds")
				rootSecretName, err := GetRootSecretName(oc)
				rootSecretYaml, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", rootSecretName, "-n=kube-system", "-o=yaml").OutputToFile("root-secret.yaml")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", rootSecretName, "-n=kube-system").Execute()
				defer func() {
					err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", rootSecretYaml).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from cluster CR")
				modeInCR, err = GetCloudCredentialMode(oc)
				e2e.Logf("cco mode in cluster CR is %v", modeInCR)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from Metric")
				err = CheckModeInMetric(oc, modeInCR)
				if err != nil {
					e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
				}
			}
		}
	})
})
