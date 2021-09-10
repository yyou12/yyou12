package psap

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// isNFDInstalled will return false if NFD is not installed, and true otherwise
func isNFDInstalled(oc *exutil.CLI, machineNFDNamespace string) bool {
	e2e.Logf("Checking if NFD operator is installed ...")
	podList, err := oc.AdminKubeClient().CoreV1().Pods(machineNFDNamespace).List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(podList.Items) == 0 {
		e2e.Logf("NFD operator not found :(")
		return false
	}
	e2e.Logf("NFD operator found!")
	return true
}

// createYAMLFromMachineSet creates a YAML file with a given filename from a given machineset name in a given API namespace, throws an error if creation fails
func createYAMLFromMachineSet(oc *exutil.CLI, machineAPINamespace string, machineSetName string, filename string) (string, error) {
	machineset_yaml, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-n", machineAPINamespace, machineSetName, "-o", "yaml").OutputToFile(filename)
	return machineset_yaml, err
}

// createMachineSetFromYAML creates a new machineset from the YAML configuration in a given filename, throws an error if creation fails
func createMachineSetFromYAML(oc *exutil.CLI, filename string) error {
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", filename).Execute()
}
