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
	} else if state == "removed" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "-p", `{"spec":{"managementState":"Removed"}}`, "--type", "merge", "-n", namespace).Execute()
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
func assertIfNodeSchedulingDisabled(oc *exutil.CLI) string {
	var nodeNames []string
	err := wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
		nodeCheck, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		isNodeSchedulingDisabled := strings.Contains(nodeCheck, "SchedulingDisabled")
		if isNodeSchedulingDisabled {
			e2e.Logf("'SchedulingDisabled' status found!")
			nodeNameReg := regexp.MustCompile(".*SchedulingDisabled.*")
			nodeNameList := nodeNameReg.FindAllString(nodeCheck, -1)
			nodeNamestr := nodeNameList[0]
			nodeNames = strings.Split(nodeNamestr, " ")
			e2e.Logf("Node Names is %v", nodeNames)
			return true, nil
		}
		e2e.Logf("'SchedulingDisabled' status not found - retrying...")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "No node was found with 'SchedulingDisabled' status within timeout limit (3 minutes)")
	e2e.Logf("Node Name is %v", nodeNames[0])
	return nodeNames[0]
}

// assertIfMasterNodeChangesApplied checks all nodes in a cluster with the master role to see if 'default_hugepagesz=2M' is present on every node in /proc/cmdline
func assertIfMasterNodeChangesApplied(oc *exutil.CLI, masterNodeName string) {

	err := wait.Poll(1*time.Minute, 5*time.Minute, func() (bool, error) {
		output, err := exutil.DebugNode(oc, masterNodeName, "cat", "/proc/cmdline")
		o.Expect(err).NotTo(o.HaveOccurred())

		isMasterNodeChanged := strings.Contains(output, "default_hugepagesz=2M")
		if isMasterNodeChanged {
			e2e.Logf("Node %v has expected changes:\n%v", masterNodeName, output)
			return true, nil
		}
		e2e.Logf("Node %v does not have expected changes - retrying...", masterNodeName)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "Node"+masterNodeName+"did not have expected changes within timeout limit")
}

// assertIfMCPChangesApplied checks the MCP of a given oc client and determines if the machine counts are as expected
func assertIfMCPChangesAppliedByName(oc *exutil.CLI, mcpName string, timeDurationMin int) {
	err := wait.Poll(1*time.Minute, time.Duration(timeDurationMin)*time.Minute, func() (bool, error) {
		var (
			mcpCheckMachineCount string
			err                  error
		)

		//For master node, only make sure one of master is ready.
		if strings.Contains(mcpName, "master") {
			mcpCheckMachineCount = "1"
		} else {
			mcpCheckMachineCount, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.machineCount}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		mcpCheckReadyMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.readyMachineCount}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mcpCheckUpdatedMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.updatedMachineCount}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mcpDegradedMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.degradedMachineCount}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if mcpCheckMachineCount == mcpCheckReadyMachineCount && mcpCheckMachineCount == mcpCheckUpdatedMachineCount && mcpDegradedMachineCount == "0" {
			e2e.Logf("MachineConfigPool checks succeeded!")
			return true, nil
		}
		e2e.Logf("MachineConfigPool %v checks failed, the following values were found (all should be '%v'):\nmachineCount: %v\nreadyMachineCount: %v\nupdatedMachineCount: %v\nmcpDegradedMachine:%v\nRetrying...", mcpName, mcpCheckMachineCount, mcpCheckMachineCount, mcpCheckReadyMachineCount, mcpCheckUpdatedMachineCount, mcpDegradedMachineCount)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "MachineConfigPool checks were not successful within timeout limit")
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

//Compare if the sysctl parameter is equal to specified value on all the node
func compareSpecifiedValueByName(oc *exutil.CLI, sysctlparm, specifiedvalue string) {
	nodeList, err := exutil.GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeListSize := len(nodeList)

	regexpstr, _ := regexp.Compile(sysctlparm + ".*")
	for i := 0; i < nodeListSize; i++ {
		output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", sysctlparm)
		conntrack_max := regexpstr.FindString(output)
		e2e.Logf("The value is %v on %v", conntrack_max, nodeList[i])
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))
	}

}

//Compare if the sysctl parameter is not equal to specified value on all the node
func compareSysctlDifferentFromSpecifiedValueByName(oc *exutil.CLI, sysctlparm, specifiedvalue string) {
	nodeList, err := exutil.GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeListSize := len(nodeList)

	regexpstr, _ := regexp.Compile(sysctlparm + ".*")
	for i := 0; i < nodeListSize; i++ {
		output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", sysctlparm)
		conntrack_max := regexpstr.FindString(output)
		e2e.Logf("The value is %v on %v", conntrack_max, nodeList[i])
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))
	}

}

//Compare the sysctl parameter's value on specified node, it should different than other node
func compareSysctlValueOnSepcifiedNodeByName(oc *exutil.CLI, tunedNodeName, sysctlparm, defaultvalue, specifiedvalue string) {
	nodeList, err := exutil.GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeListSize := len(nodeList)

	// tuned nodes should have value of 1048578, others should be 1048576
	regexpstr, _ := regexp.Compile(sysctlparm + ".*")
	for i := 0; i < nodeListSize; i++ {
		output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", sysctlparm)
		conntrack_max := regexpstr.FindString(output)
		e2e.Logf("The value is %v on %v", conntrack_max, nodeList[i])
		o.Expect(err).NotTo(o.HaveOccurred())
		if nodeList[i] == tunedNodeName {
			o.Expect(output).To(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))
		} else {
			if len(defaultvalue) == 0 {
				o.Expect(output).NotTo(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))
			} else {
				o.Expect(output).To(o.ContainSubstring(sysctlparm + " = " + defaultvalue))
			}
		}
	}
}

func getTunedPodNamebyNodeName(oc *exutil.CLI, tunedNodeName, namespace string) string {

	podNames, err := exutil.GetPodName(oc, namespace, "", tunedNodeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	//Get Pod name based on node name, and filter tuned pod name when mulitple pod return on the same node
	regexpstr, err := regexp.Compile(`tuned-.*`)
	o.Expect(err).NotTo(o.HaveOccurred())

	tunedPodName := regexpstr.FindString(podNames)
	e2e.Logf("The Tuned Pod Name is: %v", tunedPodName)
	return tunedPodName
}

type ntoResource struct {
	name        string
	namespace   string
	template    string
	sysctlparm  string
	sysctlvalue string
}

func (ntoRes *ntoResource) createTunedProfileIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", ntoRes.name, "-n", ntoRes.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No tuned in project: %s, create one: %s", ntoRes.namespace, ntoRes.name))
		exutil.CreateNsResourceFromTemplate(oc, ntoRes.namespace, "--ignore-unknown-parameters=true", "-f", ntoRes.template, "-p", "TUNED_NAME="+ntoRes.name, "SYSCTLPARM="+ntoRes.sysctlparm, "SYSCTLVALUE="+ntoRes.sysctlvalue)
	} else {
		e2e.Logf(fmt.Sprintf("Already exist %v in project: %s", ntoRes.name, ntoRes.namespace))
	}
}

func (ntoRes *ntoResource) createDebugTunedProfileIfNotExist(oc *exutil.CLI, isDebug bool) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", ntoRes.name, "-n", ntoRes.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No tuned in project: %s, create one: %s", ntoRes.namespace, ntoRes.name))
		exutil.CreateNsResourceFromTemplate(oc, ntoRes.namespace, "--ignore-unknown-parameters=true", "-f", ntoRes.template, "-p", "TUNED_NAME="+ntoRes.name, "SYSCTLPARM="+ntoRes.sysctlparm, "SYSCTLVALUE="+ntoRes.sysctlvalue, "ISDEBUG="+strconv.FormatBool(isDebug))
	} else {
		e2e.Logf(fmt.Sprintf("Already exist %v in project: %s", ntoRes.name, ntoRes.namespace))
	}
}

func (ntoRes *ntoResource) createIRQSMPAffinityProfileIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", ntoRes.name, "-n", ntoRes.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No tuned in project: %s, create one: %s", ntoRes.namespace, ntoRes.name))
		exutil.CreateNsResourceFromTemplate(oc, ntoRes.namespace, "--ignore-unknown-parameters=true", "-f", ntoRes.template, "-p", "TUNED_NAME="+ntoRes.name, "SYSCTLPARM="+ntoRes.sysctlparm, "SYSCTLVALUE="+ntoRes.sysctlvalue)
	} else {
		e2e.Logf(fmt.Sprintf("Already exist %v in project: %s", ntoRes.name, ntoRes.namespace))
	}
}

func (ntoRes *ntoResource) delete(oc *exutil.CLI) {
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ntoRes.namespace, "tuned", ntoRes.name, "--ignore-not-found").Execute()
}

func (ntoRes ntoResource) assertTunedProfileApplied(oc *exutil.CLI) {

	err := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoRes.namespace, "profile").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, ntoRes.name) {
			//Check if the new profiles name applied on a node
			e2e.Logf("Current profile for each node: \n%v", output)
			return true, nil
		} else {
			e2e.Logf("The profile %v is not applied on node, try next around \n", ntoRes.name)
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, "New tuned profile isn't applied correctly, please check")
}

func assertNTOOperatorLogs(oc *exutil.CLI, namespace string, ntoOperatorPod string, profileName string) {
	ntoOperatorLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", namespace, ntoOperatorPod, "--tail=1").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(ntoOperatorLogs).To(o.ContainSubstring(profileName))
}

func isAllInOneCluster(oc *exutil.CLI) bool {
	masterNodes, _ := exutil.GetClusterNodesBy(oc, "master")
	workerNodes, _ := exutil.GetClusterNodesBy(oc, "worker")

	if len(masterNodes) == 3 && len(workerNodes) == 3 && masterNodes[0] == workerNodes[0] {
		return true
	} else {
		return false
	}
}

func isSNOCluster(oc *exutil.CLI) bool {

	//Only 1 master, 1 worker node and with the same hostname.
	masterNodes, _ := exutil.GetClusterNodesBy(oc, "master")
	workerNodes, _ := exutil.GetClusterNodesBy(oc, "worker")
	if len(masterNodes) == 1 && len(workerNodes) == 1 && masterNodes[0] == workerNodes[0] {
		return true
	} else {
		return false
	}
}

func assertAffineDefaultCPUSets(oc *exutil.CLI, tunedPodName, namespace string) bool {

	tunedCpusAllowedList, err := exutil.RemoteShPodWithBash(oc, namespace, tunedPodName, "grep ^Cpus_allowed_list /proc/`pgrep openshift-tuned`/status")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Tuned's Cpus_allowed_list is: \n%v", tunedCpusAllowedList)

	chronyCpusAllowedList, err := exutil.RemoteShPodWithBash(oc, namespace, tunedPodName, "grep Cpus_allowed_list /proc/`pidof chronyd`/status")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Chrony's Cpus_allowed_list is: \n%v", chronyCpusAllowedList)

	regTunedCpusAllowedList0, err := regexp.Compile(`.*0-.*`)
	o.Expect(err).NotTo(o.HaveOccurred())

	regChronyCpusAllowedList1, err := regexp.Compile(`.*0$`)
	o.Expect(err).NotTo(o.HaveOccurred())

	regChronyCpusAllowedList2, err := regexp.Compile(`.*0,2-.*`)
	o.Expect(err).NotTo(o.HaveOccurred())

	isMatch0 := regTunedCpusAllowedList0.MatchString(tunedCpusAllowedList)
	isMatch1 := regChronyCpusAllowedList1.MatchString(chronyCpusAllowedList)
	isMatch2 := regChronyCpusAllowedList2.MatchString(chronyCpusAllowedList)

	if isMatch0 && (isMatch1 || isMatch2) {
		e2e.Logf("assert affine default cpusets result: %v", true)
		return true
	} else {
		e2e.Logf("assert affine default cpusets result: %v", false)
		return false
	}
}

func assertDebugSettings(oc *exutil.CLI, tunedNodeName string, ntoNamespace string, isDebug string) bool {
	nodeProfile, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("profile", tunedNodeName, "-n", ntoNamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	regDebugCheck, err := regexp.Compile(".*Debug:.*" + isDebug)
	o.Expect(err).NotTo(o.HaveOccurred())

	isMatch := regDebugCheck.MatchString(nodeProfile)
	loglines := regDebugCheck.FindAllString(nodeProfile, -1)
	e2e.Logf("The result is: %v", loglines[0])
	if isMatch {
		return true
	} else {
		return false
	}
}

func getDefaultSMPAffinityBitMaskbyCPUCores(oc *exutil.CLI, workerNodeName string) string {
	//Currently support 32core cpu worker nodes
	smpbitMask := 0xffffffff
	smpbitMaskIntStr := fmt.Sprintf("%d", smpbitMask)

	//Get CPU number in specified worker nodes
	cpuNum, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", workerNodeName, "-ojsonpath={.status.capacity.cpu}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	cpuNumStr := string(cpuNum)
	cpuNumInt, err := strconv.Atoi(cpuNumStr)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the total cpu numbers in worker nodes %v is : %v", workerNodeName, cpuNumStr)

	//Get corresponding smpMask
	rightMoveBit := 32 - cpuNumInt
	smpMaskInt, err := strconv.Atoi(smpbitMaskIntStr)
	o.Expect(err).NotTo(o.HaveOccurred())
	smpMaskStr := fmt.Sprintf("%x", smpMaskInt>>rightMoveBit)
	e2e.Logf("the bit mask for cpu numbers in worker nodes %v is : %v", workerNodeName, smpMaskStr)
	return smpMaskStr
}

//Convert hex into int string
func hexToInt(x string) string {
	base, err := strconv.ParseInt(x, 16, 10)
	o.Expect(err).NotTo(o.HaveOccurred())
	return strconv.FormatInt(base, 10)
}

func assertIsolateCPUCoresAffectedBitMask(defaultSMPBitMask string, isolatedCPU string) string {

	defaultSMPBitMaskStr := hexToInt(defaultSMPBitMask)
	isolatedCPUStr := hexToInt(isolatedCPU)

	defaultSMPBitMaskInt, err := strconv.Atoi(defaultSMPBitMaskStr)
	o.Expect(err).NotTo(o.HaveOccurred())
	isolatedCPUInt, err := strconv.Atoi(isolatedCPUStr)
	o.Expect(err).NotTo(o.HaveOccurred())

	SMPBitMask := fmt.Sprintf("%x", defaultSMPBitMaskInt^isolatedCPUInt)
	return SMPBitMask
}

func assertDefaultIRQSMPAffinityAffectedBitMask(defaultSMPBitMask string, isolatedCPU string) bool {

	var isMatch bool
	defaultSMPBitMaskStr := hexToInt(defaultSMPBitMask)
	isolatedCPUStr := hexToInt(isolatedCPU)

	defaultSMPBitMaskInt, _ := strconv.Atoi(defaultSMPBitMaskStr)
	isolatedCPUInt, err := strconv.Atoi(isolatedCPUStr)
	o.Expect(err).NotTo(o.HaveOccurred())

	if defaultSMPBitMaskInt == isolatedCPUInt {
		isMatch = true
		return isMatch
	} else {
		isMatch = false
		return isMatch
	}
}

func AssertTunedAppliedMC(oc *exutil.CLI, mcpName string, filter string) {
	mcNameList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", "--no-headers", "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	mcNameReg, _ := regexp.Compile(".*" + mcpName + ".*")
	mcName := mcNameReg.FindAllString(mcNameList, -1)

	mcOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mcName[0], "-oyaml").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(mcOutput).To(o.ContainSubstring(filter))

	//Print machineconfig content by filter
	mccontent, _ := regexp.Compile(".*" + filter + ".*")
	contentLines := mccontent.FindAllString(mcOutput, -1)
	e2e.Logf("The result is: %v", contentLines[0])
	o.Expect(mcOutput).To(o.ContainSubstring(filter))
}

func AssertTunedAppliedToNode(oc *exutil.CLI, tunedNodeName string, filter string) bool {
	cmdLineOutput, err := exutil.DebugNode(oc, tunedNodeName, "cat", "/proc/cmdline")
	o.Expect(err).NotTo(o.HaveOccurred())
	var isMatch bool
	if strings.Contains(cmdLineOutput, filter) {
		//Print machineconfig content by filter
		cmdLineReg, _ := regexp.Compile(".*" + filter + ".*")
		contentLines := cmdLineReg.FindAllString(cmdLineOutput, -1)
		e2e.Logf("The result is: %v", contentLines[0])
		isMatch = true
	} else {
		isMatch = false
	}
	return isMatch
}

func assertNTOTunedLogsLastLines(oc *exutil.CLI, namespace string, ntoTunedPod string, lineN string, filter string) {
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		ntoTunedLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", namespace, ntoTunedPod, "--tail="+lineN).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		regTunedPodLogs, err := regexp.Compile(".*" + filter + ".*")
		o.Expect(err).NotTo(o.HaveOccurred())
		isMatch := regTunedPodLogs.MatchString(ntoTunedLogs)
		if isMatch {
			loglines := regTunedPodLogs.FindAllString(ntoTunedLogs, -1)
			e2e.Logf("The logs of tuned pod %v is: %v", ntoTunedPod, loglines[0])
			return true, nil
		} else {
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, "The tuned pod's log doesn't contain the keywords, please check")
}
