package clusterinfrastructure

import (
	"math/rand"
	"strconv"
	"time"

	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	machineAPINamespace = "openshift-machine-api"
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

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "autoscaler.json")
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "Applying resources from template is failed")
	e2e.Logf("The resource is %s", jsonCfg)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", jsonCfg).Execute()
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}
