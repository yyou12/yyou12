package storage

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// GetSAToken get a token assigned to prometheus-k8s from openshift-monitoring namespace
func getSAToken(oc *exutil.CLI) string {
	e2e.Logf("Getting a token assgined to prometheus-k8s from openshift-monitoring namespace...")
	token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(token).NotTo(o.BeEmpty())
	return token
}

// Check the alert raied (pengding or firing)
func checkAlertRaised(oc *exutil.CLI, alert_name string) {
	token := getSAToken(oc)
	url, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", "prometheus-k8s", "-n", "openshift-monitoring", "-o=jsonpath={.spec.host}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	alertCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\")'", token, url, alert_name)
	//alertAnnoCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\").annotations'", token, url, alert_name)
	//alertStateCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\").state'", token, url, alert_name)
	err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
		result, err := exec.Command("bash", "-c", alertCMD).Output()
		if err != nil {
			e2e.Logf("Error retrieving prometheus alert: %v, retry ...", err)
			return false, nil
		}
		if len(string(result)) == 0 {
			e2e.Logf("Prometheus alert is nil, retry ...")
			return false, nil
		}
		if !strings.Contains(string(result), "firing") && !strings.Contains(string(result), "pending") {
			e2e.Logf(string(result))
			return false, fmt.Errorf("alert state is not firing or pending")
		}
		e2e.Logf("Alert %s found with the status firing or pending", alert_name)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "alert state is not firing or pending")
}
