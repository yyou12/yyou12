package util

import (
	"strings"
)

// GetFirstWorkerNode returns a first worker node
func GetFirstWorkerNode(oc *CLI) (string, error) {
	return getClusterNodeBy(oc, "worker", "0")
}

// GetFirstMasterNode returns a first master node
func GetFirstMasterNode(oc *CLI) (string, error) {
	return getClusterNodeBy(oc, "master", "0")
}

// getClusterNodeBy returns a cluster node by role and indexing
func getClusterNodeBy(oc *CLI, role string, index string) (string, error) {
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items["+index+"].metadata.name}'").Output()
	return strings.Trim(stdout, "'"), err
}

// DebugNodeWithChroot creates a debugging session of the node with chroot
func DebugNodeWithChroot(oc *CLI, nodeName string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, true, cmd...)
}

// DebugNode creates a debugging session of the node
func DebugNode(oc *CLI, nodeName string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, false, cmd...)
}

func debugNode(oc *CLI, nodeName string, needChroot bool, cmd ...string) (string, error) {
	var cargs []string
	if needChroot {
		cargs = []string{"node/" + nodeName, "--", "chroot", "/host"}
	} else {
		cargs = []string{"node/" + nodeName, "--"}
	}
	cargs = append(cargs, cmd...)
	return oc.AsAdmin().Run("debug").Args(cargs...).Output()
}

// DeleteCustomLabelFromNode delete the custom label from the node
func DeleteCustomLabelFromNode(oc *CLI, node string, label string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("label").Args("node", node, "node-role.kubernetes.io/"+label+"-").Output()
}

// AddCustomLabelToNode add the custom label to the node
func AddCustomLabelToNode(oc *CLI, node string, label string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("label").Args("node", node, "node-role.kubernetes.io/"+label+"=").Output()
}
