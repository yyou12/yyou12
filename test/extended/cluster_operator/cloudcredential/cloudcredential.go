package cloudcredential

import (
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cco] CCO should", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLIWithoutNamespace("default")
		data         PrometheusQueryResult
		modeInMetric string
	)

	// author: lwan@redhat.com
	// It is destructive case, will remove root credentials, so adding [Disruptive].
	g.It("Author:lwan-Hign-31768-Report the mode of cloud-credential operation as a metric [Disruptive]", func() {
		g.By("Check if the current platform is a supported platform")
		rootSecretName, err := GetRootSecretName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if rootSecretName == "" {
			e2e.Logf("unsupported platform, there is no root credential in kube-system namespace,  will pass the test")
		} else {
			g.By("Get cco mode from cluster CR")
			modeInCR, err := GetCloudCredentialMode(oc)
			e2e.Logf("cco mode in cluster CR is %v", modeInCR)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Get cco mode from Metric")
			token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(token).NotTo(o.BeEmpty())
			pollErr := wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
				msg, _, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=cco_credentials_mode").Outputs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(msg).NotTo(o.BeEmpty())
				json.Unmarshal([]byte(msg), &data)
				modeInMetric = data.Data.Result[0].Metric.Mode
				e2e.Logf("cco mode in metric is %v", modeInMetric)
				g.By("Check if cco mode in metric is the same as mode in CR")
				if modeInCR != modeInMetric {
					e2e.Logf("cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
					return false, nil
				}
				return true, nil
			})
			if pollErr != nil {
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
				defer oc.AsAdmin().Run("patch").Args("cloudcredential/cluster", "--type=merge", "-p", patchYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from cluster CR")
				modeInCR, err := GetCloudCredentialMode(oc)
				e2e.Logf("cco mode in cluster CR is %v", modeInCR)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from Metric")
				pollErr := wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
					msg, _, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=cco_credentials_mode").Outputs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(msg).NotTo(o.BeEmpty())
					json.Unmarshal([]byte(msg), &data)
					modeInMetric = data.Data.Result[0].Metric.Mode
					e2e.Logf("cco mode in metric is %v", modeInMetric)
					g.By("Check if cco mode in metric is the same as mode in CR")
					if modeInCR != modeInMetric {
						e2e.Logf("cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
						return false, nil
					}
					return true, nil
				})
				if pollErr != nil {
					e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
				}
				g.By("Check cco mode when root credential is removed when cco is not in manual mode")
				e2e.Logf("remove root creds")
				rootSecretName, err := GetRootSecretName(oc)
				rootSecretYaml, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", rootSecretName, "-n=kube-system", "-o=yaml").OutputToFile("root-secret.yaml")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", rootSecretName, "-n=kube-system").Execute()
				defer oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", rootSecretYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from cluster CR")
				modeInCR, err = GetCloudCredentialMode(oc)
				e2e.Logf("cco mode in cluster CR is %v", modeInCR)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from Metric")
				pollErr = wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
					msg, _, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=cco_credentials_mode").Outputs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(msg).NotTo(o.BeEmpty())
					json.Unmarshal([]byte(msg), &data)
					modeInMetric = data.Data.Result[0].Metric.Mode
					e2e.Logf("cco mode in metric is %v", modeInMetric)
					g.By("Check if cco mode in metric is the same as mode in CR")
					if modeInCR != modeInMetric {
						e2e.Logf("cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
						return false, nil
					}
					return true, nil
				})
				if pollErr != nil {
					e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
				}

			}
		}
	})
})
