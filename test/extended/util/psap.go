package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

//This will check if operator deployment/daemonset is created sucessfully
//will update sro test case to use this common utils later.
//example:
//WaitOprResourceReady(oc, deployment, deployment-name, namespace, true, true)
//WaitOprResourceReady(oc, statefulset, statefulset-name, namespace, false, false)
//WaitOprResourceReady(oc, daemonset, daemonset-name, namespace, false, false)
//If islongduration is true, it will sleep 720s, otherwise 180s
//If excludewinnode is true, skip checking windows nodes daemonset status
//For daemonset or deployment have random name, getting name before use this function

func WaitOprResourceReady(oc *CLI, kind, name, namespace string, islongduration bool, excludewinnode bool) {

	//If islongduration is true, it will sleep 720s, otherwise 180s
	var timeDurationSec int
	if islongduration {
		timeDurationSec = 720
	} else {
		timeDurationSec = 300
	}

	waitErr := wait.Poll(20*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {
		var (
			kindNames  string
			err        error
			isCreated  bool
			desiredNum string
			readyNum   string
		)

		//Check if deployment/daemonset/statefulset is created.
		switch kind {
		case "deployment", "statefulset":
			kindNames, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, name, "-n", namespace, "-oname").Output()
			if strings.Contains(kindNames, "NotFound") || strings.Contains(kindNames, "No resources") || len(kindNames) == 0 || err != nil {
				isCreated = false
			} else {
				//deployment/statefulset has been created, but not running, need to compare .status.readyReplicas and  in .status.replicas
				isCreated = true
				desiredNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.readyReplicas}").Output()
				readyNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.replicas}").Output()
			}
		case "daemonset":
			kindNames, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, name, "-n", namespace, "-oname").Output()
			e2e.Logf("daemonset name is:" + kindNames)
			if len(kindNames) == 0 || err != nil {
				isCreated = false
			} else {
				//daemonset/statefulset has been created, but not running, need to compare .status.desiredNumberScheduled and .status.numberReady}
				//if the two value is equal, set output="has successfully progressed"
				isCreated = true
				desiredNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.desiredNumberScheduled}").Output()
				//If there are windows worker nodes, the desired daemonset should be linux node's num
				_, WindowsNodeNum := CountNodeNumByOS(oc)
				if WindowsNodeNum > 0 && excludewinnode {

					//Exclude windows nodes
					e2e.Logf("%v desiredNum is: %v", kindNames, desiredNum)
					desiredLinuxWorkerNum, _ := strconv.Atoi(desiredNum)
					e2e.Logf("desiredlinuxworkerNum is:%v", desiredLinuxWorkerNum)
					desiredNum = strconv.Itoa(desiredLinuxWorkerNum - WindowsNodeNum)
				}
				readyNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.numberReady}").Output()
			}
		default:
			e2e.Logf("Invalid Resource Type")
		}

		e2e.Logf("desiredNum is: " + desiredNum + " readyNum is: " + readyNum)
		//daemonset/deloyment has been created, but not running, need to compare desiredNum and readynum
		//if isCreate is true and the two value is equal, the pod is ready
		if isCreated && len(kindNames) != 0 && desiredNum == readyNum {
			e2e.Logf("The %v is successfully progressed and running normally", kindNames)
			return true, nil
		} else {
			e2e.Logf("The %v is not ready or running normally", kindNames)
			return false, nil
		}
	})
	AssertWaitPollNoErr(waitErr, fmt.Sprintf("the pod of %v is not running", name))
}

//Check if NFD Installed base on the cluster labels
func IsNodeLabeledByNFD(oc *CLI) bool {
	workNode, _ := GetFirstWorkerNode(oc)
	Output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", workNode, "-o", "jsonpath='{.metadata.annotations}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(Output, "nfd.node.kubernetes.io/feature-labels") {
		e2e.Logf("NFD installed on openshift container platform and labeled nodes")
		return true
	}
	return false
}

func CountNodeNumByOS(oc *CLI) (linuxNum int, windowsNum int) {
	//Count how many windows node and linux node
	linuxNodeNames, err := GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	windowsNodeNames, err := GetAllNodesbyOSType(oc, "windows")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("linuxNodeNames is:%v", linuxNodeNames[:])
	e2e.Logf("windowsNodeNames is:%v", windowsNodeNames[:])
	linuxNum = len(linuxNodeNames)
	windowsNum = len(windowsNodeNames)
	e2e.Logf("Linux node is:%v, windows node is %v", linuxNum, windowsNum)
	return linuxNum, windowsNum
}

func GetFirstLinuxMachineSets(oc *CLI) string {
	machinesets, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-o=jsonpath={.items[*].metadata.name}", "-n", "openshift-machine-api").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	machinesets_array := strings.Split(machinesets, " ")
	//Remove windows machineset
	for i, machineset := range machinesets_array {
		if machineset == "windows" {
			machinesets_array = append(machinesets_array[:i], machinesets_array[i+1:]...)
			e2e.Logf("%T,%v", machinesets, machinesets)
		}
	}
	return machinesets_array[0]
}
