package mco

import (
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

// GetAllNodes returns a list of Node structs with the nodes found in this list
func (nl *nodeList) GetAllNodes() ([]node, error) {
	allNodeResources, err := nl.GetAllResources()
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

	return nl.GetAllNodes()
}

// GetAllWorkerNodes returns a list of worker Nodes
func (nl nodeList) GetAllWorkerNodes() ([]node, error) {
	nl.ByLabel("node-role.kubernetes.io/worker=")

	return nl.GetAllNodes()
}
