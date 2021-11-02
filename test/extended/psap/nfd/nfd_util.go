package nfd

import (
	"fmt"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	nfdNamespace           = "openshift-nfd"
	nfd_namespace_file     = exutil.FixturePath("testdata", "psap", "nfd", "nfd-namespace.yaml")
	nfd_operatorgroup_file = exutil.FixturePath("testdata", "psap", "nfd", "nfd-operatorgroup.yaml")
	nfd_sub_file           = exutil.FixturePath("testdata", "psap", "nfd", "nfd-sub.yaml")
	nfd_instance_file      = exutil.FixturePath("testdata", "psap", "nfd", "nfd-instance.yaml")
)

// isPodInstalled will return true if any pod is found in the given namespace, and false otherwise
func isPodInstalled(oc *exutil.CLI, namespace string) bool {
	e2e.Logf("Checking if pod is found in namespace %s...", namespace)
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(podList.Items) == 0 {
		e2e.Logf("No pod found in namespace %s :(", namespace)
		return false
	}
	e2e.Logf("Pod found in namespace %s!", namespace)
	return true
}

// installNFD attempts to install the Node Feature Discovery operator and verify that it is running
func installNFD(oc *exutil.CLI) {

	// check if NFD namespace already exists
	err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespace", nfdNamespace).Execute()

	// if namespace exists, check if NFD is installed - exit if it is, continue with installation otherwise
	// if an error is thrown, namespace does not exist, create and continue with installation
	if err == nil {
		e2e.Logf("NFD namespace found - checking if NFD is installed ...")
		nfdInstalled := isPodInstalled(oc, nfdNamespace)
		if nfdInstalled {
			e2e.Logf("NFD installation found! Continuing with test ...")
			return
		}
		e2e.Logf("NFD namespace found but no pods running - attempting installation ...")
	} else {
		e2e.Logf("NFD namespace not found - creating namespace and installing NFD ...")
		exutil.CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", nfd_namespace_file)
	}

	// create NFD operator group from template
	exutil.ApplyNsResourceFromTemplate(oc, nfdNamespace, "--ignore-unknown-parameters=true", "-f", nfd_operatorgroup_file)

	// get default channel and create subscription from template
	channel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "nfd", "-n", "openshift-marketplace", "-o", "jsonpath={.status.defaultChannel}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Channel: %v", channel)
	exutil.ApplyNsResourceFromTemplate(oc, nfdNamespace, "--ignore-unknown-parameters=true", "-f", nfd_sub_file, "-p", "CHANNEL="+channel)

	// get cluster version and create NFD instance from template
	clusterVersion, _, err := exutil.GetClusterVersion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Cluster Version: %v", clusterVersion)
	exutil.ApplyNsResourceFromTemplate(oc, nfdNamespace, "--ignore-unknown-parameters=true", "-f", nfd_instance_file, "-p", "IMAGE=quay.io/openshift/origin-node-feature-discovery:"+clusterVersion)

	// wait for NFD pods to come online to verify installation was successful
	err = wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
		podInstalled := isPodInstalled(oc, nfdNamespace)
		if !podInstalled {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("NFD installation failed - No pods found within namespace %s within timeout limit (3 minutes)", nfdNamespace))
}

// createYAMLFromMachineSet creates a YAML file with a given filename from a given machineset name in a given namespace, throws an error if creation fails
func createYAMLFromMachineSet(oc *exutil.CLI, namespace string, machineSetName string, filename string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-n", namespace, machineSetName, "-o", "yaml").OutputToFile(filename)
}

// createMachineSetFromYAML creates a new machineset from the YAML configuration in a given filename, throws an error if creation fails
func createMachineSetFromYAML(oc *exutil.CLI, filename string) error {
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", filename).Execute()
}

// deleteMachineSet will delete a given machineset name from a given namespace
func deleteMachineSet(oc *exutil.CLI, namespace string, machineSetName string) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("machineset", machineSetName, "-n", namespace).Execute()
}
