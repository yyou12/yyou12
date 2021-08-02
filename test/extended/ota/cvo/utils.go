package cvo

import (
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/apimachinery/pkg/util/wait"
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
