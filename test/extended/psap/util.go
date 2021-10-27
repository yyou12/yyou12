package psap

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	machineNFDNamespace    = "openshift-nfd"
	nfd_namespace_file     = exutil.FixturePath("testdata", "psap", "nfd-namespace.yaml")
	nfd_operatorgroup_file = exutil.FixturePath("testdata", "psap", "nfd-operatorgroup.yaml")
	nfd_sub_file           = exutil.FixturePath("testdata", "psap", "nfd-sub.yaml")
	nfd_instance_file      = exutil.FixturePath("testdata", "psap", "nfd-instance.yaml")
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
	err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespace", machineNFDNamespace).Execute()

	// if namespace exists, check if NFD is installed - exit if it is, continue with installation otherwise
	// if an error is thrown, namespace does not exist, create and continue with installation
	if err == nil {
		e2e.Logf("NFD namespace found - checking if NFD is installed ...")
		nfdInstalled := isPodInstalled(oc, machineNFDNamespace)
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
	exutil.ApplyNsResourceFromTemplate(oc, machineNFDNamespace, "--ignore-unknown-parameters=true", "-f", nfd_operatorgroup_file)

	// get default channel and create subscription from template
	channel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "nfd", "-n", "openshift-marketplace", "-o", "jsonpath={.status.defaultChannel}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Channel: %v", channel)
	exutil.ApplyNsResourceFromTemplate(oc, machineNFDNamespace, "--ignore-unknown-parameters=true", "-f", nfd_sub_file, "-p", "CHANNEL="+channel)

	// get cluster version and create NFD instance from template
	_, clusterBuild, err := exutil.GetClusterVersion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	splitValues := strings.Split(clusterBuild, ".")
	clusterVersion := splitValues[0] + "." + splitValues[1]
	e2e.Logf("Cluster Version: %v", clusterVersion)
	exutil.ApplyNsResourceFromTemplate(oc, machineNFDNamespace, "--ignore-unknown-parameters=true", "-f", nfd_instance_file, "-p", "IMAGE=quay.io/openshift/origin-node-feature-discovery:"+clusterVersion)

	// wait for NFD pods to come online to verify installation was successful
	err = wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
		podInstalled := isPodInstalled(oc, machineNFDNamespace)
		if !podInstalled {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("NFD installation failed - No pods found within namespace %s within timeout limit (3 minutes)", machineNFDNamespace))
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

// createFromCustomResource will create a new OCP resource in a given namespace from the given file
func createFromCustomResource(oc *exutil.CLI, namespace string, file string) error {
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", namespace, "-f", file).Execute()
}

// getTunedPod returns the name of the tuned pod on a given node in a given namespace
func getTunedPod(oc *exutil.CLI, namespace string, nodeName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace, "--field-selector=spec.nodeName="+nodeName, "-o", "name").Output()
}

// getTunedState returns a string representation of the spec.managementState of the default tuned in a given namespace
func getTunedState(oc *exutil.CLI, namespace string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", "default", "-n", namespace, "-o=jsonpath={.spec.managementState}").Output()
}

// patchTunedState will patch the state of the tuned operator to that specified if supported, will throw an error if patch fails or state unsupported
func patchTunedState(oc *exutil.CLI, namespace string, state string) error {
	state = strings.ToLower(state)
	if state == "unmanaged" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned/default", "-p", `{"spec":{"managementState":"Unmanaged"}}`, "--type", "merge", "-n", namespace).Execute()
	} else if state == "managed" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned/default", "-p", `{"spec":{"managementState":"Managed"}}`, "--type", "merge", "-n", namespace).Execute()
	} else {
		return fmt.Errorf("specified state %s is unsupported", state)
	}
}

// getTunedProfile returns a string representation of the status.tunedProfile of the given node in the given namespace
func getTunedProfile(oc *exutil.CLI, namespace string, tunedNodeName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("profile", tunedNodeName, "-n", namespace, "-o=jsonpath={.status.tunedProfile}").Output()
}

// checkIfTunedProfileApplied checks the logs for a given tuned pod in a given namespace to see if the expected profile was applied
func assertIfTunedProfileApplied(oc *exutil.CLI, namespace string, tunedPodName string, profile string) {
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		podLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", namespace, "--tail=9", tunedPodName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		isApplied := strings.Contains(podLogs, "tuned.daemon.daemon: static tuning from profile '"+profile+"' applied")
		if !isApplied {
			e2e.Logf("Profile '%s' has not yet been applied to %s - retrying...", profile, tunedPodName)
			return false, nil
		}
		e2e.Logf("Profile '%s' has been applied to %s - continuing...", profile, tunedPodName)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Profile was not applied to %s within timeout limit (30 seconds)", tunedPodName))
}

// getMaxUserWatchesValue parses out the line determining max_user_watches in inotify.conf
func getMaxUserWatchesValue(inotify string) string {
	re_line := regexp.MustCompile(`fs.inotify.max_user_watches = \d+`)
	re_value := regexp.MustCompile(`\d+`)
	max_user_watches := re_line.FindString(inotify)
	max_user_watches_value := re_value.FindString(max_user_watches)
	return max_user_watches_value
}

// getMaxUserInstancesValue parses out the line determining max_user_instances in inotify.conf
func getMaxUserInstancesValue(inotify string) string {
	re_line := regexp.MustCompile(`fs.inotify.max_user_instances = \d+`)
	re_value := regexp.MustCompile(`\d+`)
	max_user_instances := re_line.FindString(inotify)
	max_user_instances_value := re_value.FindString(max_user_instances)
	return max_user_instances_value
}

// getKernelPidMaxValue parses out the line determining pid_max in the kernel
func getKernelPidMaxValue(kernel string) string {
	re_line := regexp.MustCompile(`kernel.pid_max = \d+`)
	re_value := regexp.MustCompile(`\d+`)
	pid_max := re_line.FindString(kernel)
	pid_max_value := re_value.FindString(pid_max)
	return pid_max_value
}
