package mco

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

type node struct {
	Resource
}

type nodeList struct {
	ResourceList
}

// NewNode construct a new node struct
func NewNode(oc *exutil.CLI, name string) *node {
	//NewResource(oc, "node", name)
	return &node{*NewResource(oc, "node", name)}
}

// NewNodeList construct a new node list struct to handle all existing nodes
func NewNodeList(oc *exutil.CLI) *nodeList {
	return &nodeList{*NewResourceList(oc, "node")}
}

// DebugNode creates a debugging session of the node with chroot
func (n *node) DebugNodeWithChroot(cmd ...string) (string, error) {
	return exutil.DebugNodeWithChroot(n.oc, n.name, cmd...)
}

// DebugNodeWithOptions launch debug container with options e.g. --image
func (n *node) DebugNodeWithOptions(options []string, cmd ...string) (string, error) {
	return exutil.DebugNodeWithOptions(n.oc, n.name, options, cmd...)
}

// DebugNode creates a debugging session of the node
func (n *node) DebugNode(cmd ...string) (string, error) {
	return exutil.DebugNode(n.oc, n.name, cmd...)
}

// AddCustomLabel add the given label to the node
func (n *node) AddCustomLabel(label string) (string, error) {
	return exutil.AddCustomLabelToNode(n.oc, n.name, label)

}

// DeleteCustomLabel removes the given label from the node
func (n *node) DeleteCustomLabel(label string) (string, error) {
	return exutil.DeleteCustomLabelFromNode(n.oc, n.name, label)

}

// GetMachineConfigDaemon returns the name of the ConfigDaemon pod for this node
func (n *node) GetMachineConfigDaemon() string {
	machineConfigDaemon, err := exutil.GetPodName(n.oc, "openshift-machine-config-operator", "k8s-app=machine-config-daemon", n.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	return machineConfigDaemon
}

// GetNodeHostname returns the cluster node hostname
func (n *node) GetNodeHostname() (string, error) {
	return exutil.GetNodeHostname(n.oc, n.name)
}

// ForceReapplyConfiguration create the file `/run/machine-config-daemon-force` in the node
//  in order to force MCO to reapply the current configuration
func (n *node) ForceReapplyConfiguration() error {
	_, err := n.DebugNodeWithChroot("touch", "/run/machine-config-daemon-force")

	return err
}

// GetUnitStatus executes `systemctl status` command on the node and returns the output
func (n *node) GetUnitStatus(unitName string) (string, error) {
	return n.DebugNodeWithChroot("systemctl", "status", unitName)
}

//GetAll returns a []node list with all existing nodes
func (nl *nodeList) GetAll() ([]node, error) {
	allNodeResources, err := nl.ResourceList.GetAll()
	if err != nil {
		return nil, err
	}
	allNodes := make([]node, 0, len(allNodeResources))

	for _, nodeRes := range allNodeResources {
		allNodes = append(allNodes, *NewNode(nl.oc, nodeRes.name))
	}

	return allNodes, nil
}

// GetAllMasterNodes returns a list of master Nodes
func (nl nodeList) GetAllMasterNodes() ([]node, error) {
	nl.ByLabel("node-role.kubernetes.io/master=")

	return nl.GetAll()
}

// GetAllWorkerNodes returns a list of worker Nodes
func (nl nodeList) GetAllWorkerNodes() ([]node, error) {
	nl.ByLabel("node-role.kubernetes.io/worker=")

	return nl.GetAll()
}

// GetAllMasterNodesOrFail returns a list of master Nodes
func (nl nodeList) GetAllMasterNodesOrFail() []node {
	masters, err := nl.GetAllMasterNodes()
	o.Expect(err).NotTo(o.HaveOccurred())
	return masters
}

// GetAllWorkerNodes returns a list of worker Nodes
func (nl nodeList) GetAllWorkerNodesOrFail() []node {
	workers, err := nl.GetAllWorkerNodes()
	o.Expect(err).NotTo(o.HaveOccurred())
	return workers
}

func (nl nodeList) GetAllRhelWokerNodesOrFail() []node {
	nl.ByLabel("node-role.kubernetes.io/worker=,node.openshift.io/os_id=rhel")

	workers, err := nl.GetAll()
	o.Expect(err).NotTo(o.HaveOccurred())
	return workers
}

func (nl nodeList) GetAllCoreOsWokerNodesOrFail() []node {
	nl.ByLabel("node-role.kubernetes.io/worker=,node.openshift.io/os_id=rhcos")

	workers, err := nl.GetAll()
	o.Expect(err).NotTo(o.HaveOccurred())
	return workers
}
