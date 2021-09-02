package cvo

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// GetDeploymentsYaml dumps out deployment in yaml format in specific namespace
func GetDeploymentsYaml(oc *exutil.CLI, deployment_name string, namespace string) (string, error) {
	e2e.Logf("Dumping deployments %s from namespace %s", deployment_name, namespace)
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", deployment_name, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error dumping deployments: %v", err)
		return "", err
	}
	e2e.Logf(out)
	return out, err
}

// PodExec executes a single command or a bash script in the running pod. It returns the
// command output and error if the command finished with non-zero status code or the
// command took longer than 3 minutes to run.
func PodExec(oc *exutil.CLI, script string, namespace string, podName string) (string, error) {
	var out string
	waitErr := wait.PollImmediate(1*time.Second, 3*time.Minute, func() (bool, error) {
		var err error
		out, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", namespace, podName, "--", "/bin/bash", "-c", script).Output()
		return true, err
	})
	return out, waitErr
}

// GetSAToken get a token assigned to prometheus-k8s from openshift-monitoring namespace
func getSAToken(oc *exutil.CLI) (string, error) {
	e2e.Logf("Getting a token assgined to prometheus-k8s from openshift-monitoring namespace...")
	token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	return token, err
}

// WaitForAlert check if an alert appears
// Return value: bool: indicate if the alert is found
// Return value: map: annotation map which contains reason and message information
// Retrun value: error: any error
func waitForAlert(oc *exutil.CLI, alertString string, interval time.Duration, timeout time.Duration, state string) (bool, map[string]string, error) {
	e2e.Logf("Waiting for alert %s pending or firing...", alertString)
	url, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", "prometheus-k8s", "-n", "openshift-monitoring", "-o=jsonpath={.spec.host}").Output()
	if err != nil || len(url) == 0 {
		return false, nil, fmt.Errorf("error getting the hostname of route prometheus-k8s %v", err)
	}
	token, err := getSAToken(oc)
	if err != nil || len(token) == 0 {
		return false, nil, fmt.Errorf("error getting SA token %v", err)
	}

	alertCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\")'", token, url, alertString)
	alertAnnoCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\").annotations'", token, url, alertString)
	if len(state) > 0 {
		alertCMD = fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\" and .state == \"%s\")'", token, url, alertString, state)
		alertAnnoCMD = fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\" and .state == \"%s\").annotations'", token, url, alertString, state)
	}

	// Poll returns timed out waiting for the condition when timeout is reached
	count := 0
	if pollErr := wait.Poll(interval*time.Second, timeout*time.Second, func() (bool, error) {
		count += 1
		metrics, err := exec.Command("bash", "-c", alertCMD).Output()
		if err != nil {
			e2e.Logf("Error retrieving prometheus alert metrics: %v, retry %d...", err, count)
			return false, nil
		}
		if metrics == nil {
			e2e.Logf("Prometheus alert metrics nil, retry %d...", count)
			return false, nil
		}
		if alertString == "firing" && int(interval)*count < int(timeout) {
			return true, fmt.Errorf("error alert firing but timeout is not reached")
		}
		return true, nil
	}); pollErr != nil {
		return false, nil, pollErr
	}
	e2e.Logf("Alert %s found", alertString)
	annotation, err := exec.Command("bash", "-c", alertAnnoCMD).Output()
	if err != nil || annotation == nil {
		return true, nil, fmt.Errorf("error getting annotation for alert %s", alertString)
	}
	var annoMap map[string]string
	if err := json.Unmarshal(annotation, &annoMap); err != nil {
		return true, nil, fmt.Errorf("error converting annotation to map for alert %s", alertString)
	}

	return true, annoMap, nil
}
