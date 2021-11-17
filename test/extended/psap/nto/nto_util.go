package nto

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
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

// getNTOPodName checks all pods in a given namespace and returns the first NTO pod name found
func getNTOPodName(oc *exutil.CLI, namespace string) (string, error) {
	podList, err := exutil.GetAllPods(oc, namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	podListSize := len(podList)
	for i := 0; i < podListSize; i++ {
		if strings.Contains(podList[i], "cluster-node-tuning-operator") {
			return podList[i], nil
		}
	}
	return "", fmt.Errorf("NTO pod was not found in namespace %s", namespace)
}

// getTunedState returns a string representation of the spec.managementState of the specified tuned in a given namespace
func getTunedState(oc *exutil.CLI, namespace string, tunedName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", tunedName, "-n", namespace, "-o=jsonpath={.spec.managementState}").Output()
}

// patchTunedState will patch the state of the specified tuned to that specified if supported, will throw an error if patch fails or state unsupported
func patchTunedState(oc *exutil.CLI, namespace string, tunedName string, state string) error {
	state = strings.ToLower(state)
	if state == "unmanaged" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "-p", `{"spec":{"managementState":"Unmanaged"}}`, "--type", "merge", "-n", namespace).Execute()
	} else if state == "managed" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "-p", `{"spec":{"managementState":"Managed"}}`, "--type", "merge", "-n", namespace).Execute()
	} else {
		return fmt.Errorf("specified state %s is unsupported", state)
	}
}

// getTunedPriority returns a string representation of the spec.recommend.priority of the specified tuned in a given namespace
func getTunedPriority(oc *exutil.CLI, namespace string, tunedName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", tunedName, "-n", namespace, "-o=jsonpath={.spec.recommend[*].priority}").Output()
}

// patchTunedPriority will patch the priority of the specified tuned to that specified in a given YAML or JSON file
// we cannot directly patch the value since it is nested within a list, thus the need for a patch file for this function
func patchTunedPriority(oc *exutil.CLI, namespace string, tunedName string, patchFile string) error {
	return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "--patch-file="+patchFile, "--type", "merge", "-n", namespace).Execute()
}

// getTunedRender returns a string representation of the rendered for tuned in the given namespace
func getTunedRender(oc *exutil.CLI, namespace string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", namespace, "tuned", "rendered", "-o", "yaml").Output()
}

// getTunedProfile returns a string representation of the status.tunedProfile of the given node in the given namespace
func getTunedProfile(oc *exutil.CLI, namespace string, tunedNodeName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("profile", tunedNodeName, "-n", namespace, "-o=jsonpath={.status.tunedProfile}").Output()
}

// assertIfTunedProfileApplied checks the logs for a given tuned pod in a given namespace to see if the expected profile was applied
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

// assertIfNodeSchedulingDisabled checks all nodes in a cluster to see if 'SchedulingDisabled' status is present on any node
func assertIfNodeSchedulingDisabled(oc *exutil.CLI) {
	err := wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
		nodeCheck, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		isNodeSchedulingDisabled := strings.Contains(nodeCheck, "SchedulingDisabled")
		if isNodeSchedulingDisabled {
			e2e.Logf("'SchedulingDisabled' status found!")
			return true, nil
		}
		e2e.Logf("'SchedulingDisabled' status not found - retrying...")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "No node was found with 'SchedulingDisabled' status within timeout limit (3 minutes)")
}

// assertIfMasterNodeChangesApplied checks all nodes in a cluster with the master role to see if 'default_hugepagesz=2M' is present on every node in /proc/cmdline
func assertIfMasterNodeChangesApplied(oc *exutil.CLI) {
	masterNodeList, err := exutil.GetClusterNodesBy(oc, "master")
	o.Expect(err).NotTo(o.HaveOccurred())
	masterNodeListSize := len(masterNodeList)
	for i := 0; i < masterNodeListSize; i++ {
		err := wait.Poll(1*time.Minute, 10*time.Minute, func() (bool, error) {
			output, err := exutil.DebugNode(oc, masterNodeList[i], "cat", "/proc/cmdline")
			o.Expect(err).NotTo(o.HaveOccurred())

			isMasterNodeChanged := strings.Contains(output, "default_hugepagesz=2M")
			if isMasterNodeChanged {
				e2e.Logf("Node %v has expected changes", masterNodeList[i])
				return true, nil
			}
			e2e.Logf("Node %v does not have expected changes - retrying...", masterNodeList[i])
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Node"+masterNodeList[i]+"did not have expected changes within timeout limit (10 minutes)")
	}
}

// assertIfMCPChangesApplied checks the MCP of a given oc client and determines if the machine counts are as expected
func assertIfMCPChangesApplied(oc *exutil.CLI) {
	err := wait.Poll(1*time.Minute, 15*time.Minute, func() (bool, error) {
		masterNodeList, err := exutil.GetClusterNodesBy(oc, "master")
		o.Expect(err).NotTo(o.HaveOccurred())
		masterNodeListSize := strconv.Itoa(len(masterNodeList))

		mcpCheckMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "-o=jsonpath={.items[0].status.machineCount}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mcpCheckReadyMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "-o=jsonpath={.items[0].status.readyMachineCount}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mcpCheckUpdatedMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "-o=jsonpath={.items[0].status.updatedMachineCount}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		isMachineCountApplied := strings.Contains(mcpCheckMachineCount, masterNodeListSize)
		isReadyMachineCountApplied := strings.Contains(mcpCheckReadyMachineCount, masterNodeListSize)
		isUpdatedMachineCountApplied := strings.Contains(mcpCheckUpdatedMachineCount, masterNodeListSize)

		if isMachineCountApplied && isReadyMachineCountApplied && isUpdatedMachineCountApplied {
			e2e.Logf("MachineConfigPool checks succeeded!")
			return true, nil
		}
		e2e.Logf("MachineConfigPool checks failed, the following values were found (all should be '%v'):\nmachineCount: %v\nreadyMachineCount: %v\nupdatedMachineCount: %v\nRetrying...", masterNodeListSize, mcpCheckMachineCount, mcpCheckReadyMachineCount, mcpCheckUpdatedMachineCount)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "MachineConfigPool checks were not successful within timeout limit (15 minutes)")
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
