package clusterinfrastructure

import (
	o "github.com/onsi/gomega"
	"strconv"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type clusterAutoscalerDescription struct {
	maxNode   int
	minCore   int
	maxCore   int
	minMemory int
	maxMemory int
	template  string
}

type machineAutoscalerDescription struct {
	name        string
	namespace   string
	maxReplicas int
	minReplicas int
	template    string
}

func (clusterAutoscaler *clusterAutoscalerDescription) createClusterAutoscaler(oc *exutil.CLI) {
	e2e.Logf("Creating clusterautoscaler ...")
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", clusterAutoscaler.template, "-p", "MAXNODE="+strconv.Itoa(clusterAutoscaler.maxNode), "MINCORE="+strconv.Itoa(clusterAutoscaler.minCore), "MAXCORE="+strconv.Itoa(clusterAutoscaler.maxCore), "MINMEMORY="+strconv.Itoa(clusterAutoscaler.minMemory), "MAXMEMORY="+strconv.Itoa(clusterAutoscaler.maxMemory))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (clusterAutoscaler *clusterAutoscalerDescription) deleteClusterAutoscaler(oc *exutil.CLI) error {
	e2e.Logf("Deleting clusterautoscaler ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterautoscaler", "default").Execute()
}

func (machineAutoscaler *machineAutoscalerDescription) createMachineAutoscaler(oc *exutil.CLI) {
	e2e.Logf("Creating machineautoscaler ...")
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", machineAutoscaler.template, "-p", "NAME="+machineAutoscaler.name, "NAMESPACE="+machineAPINamespace, "REPLICAS="+strconv.Itoa(machineAutoscaler.maxReplicas), "CLUSTERID="+strconv.Itoa(machineAutoscaler.maxReplicas))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (machineAutoscaler *machineAutoscalerDescription) deleteMachineAutoscaler(oc *exutil.CLI) error {
	e2e.Logf("Deleting a machineautoscaler ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("machineautoscaler", machineAutoscaler.name, "-n", machineAPINamespace).Execute()
}
