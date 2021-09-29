package util

import (
	"strings"
)

// GetFirstWorkerNode returns a first worker node
func GetFirstWorkerNode(oc *CLI) (string, error) {
	workerNodes, err := getClusterNodesBy(oc, "worker")
	return workerNodes[0], err
}

// GetFirstMasterNode returns a first master node
func GetFirstMasterNode(oc *CLI) (string, error) {
	masterNodes, err := getClusterNodesBy(oc, "master")
	return masterNodes[0], err
}

// getClusterNodesBy returns the cluster nodes by role
func getClusterNodesBy(oc *CLI, role string) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// DebugNodeWithChroot creates a debugging session of the node with chroot
func DebugNodeWithChroot(oc *CLI, nodeName string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, []string{}, true, cmd...)
}

// DebugNodeWithOptions launch debug container with options e.g. --image
func DebugNodeWithOptions(oc *CLI, nodeName string, options []string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, options, false, cmd...)
}

// DebugNode creates a debugging session of the node
func DebugNode(oc *CLI, nodeName string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, []string{}, false, cmd...)
}

func debugNode(oc *CLI, nodeName string, cmdOptions []string, needChroot bool, cmd ...string) (string, error) {
	var cargs []string
	cargs = []string{"node/" + nodeName}
	if len(cmdOptions) > 0 {
		cargs = append(cargs, cmdOptions...)
	}
	if needChroot {
		cargs = append(cargs, "--", "chroot", "/host")
	} else {
		cargs = append(cargs, "--")
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

// GetFirstCoreOsWorkerNode returns the first CoreOS worker node
func GetFirstCoreOsWorkerNode(oc *CLI) (string, error) {
	return getFirstNodeByOsId(oc, "worker", "rhcos")
}

// GetFirstRhelWorkerNode returns the first rhel worker node
func GetFirstRhelWorkerNode(oc *CLI) (string, error) {
	return getFirstNodeByOsId(oc, "worker", "rhel")
}

// getFirstNodeByOsId returns the cluster node by role and os id
func getFirstNodeByOsId(oc *CLI, role string, osId string) (string, error) {
	nodes, err := getClusterNodesBy(oc, role)
	for _, node := range nodes {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node/"+node, "-o", "jsonpath=\"{.metadata.labels.node\\.openshift\\.io/os_id}\"").Output()
		if strings.Trim(stdout, "\"") == osId {
			return node, err
		}
	}
	return "", err
}
