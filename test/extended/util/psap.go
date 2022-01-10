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
		timeDurationSec = 360
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

// installNFD attempts to install the Node Feature Discovery operator and verify that it is running
func InstallNFD(oc *CLI, nfdNamespace string) {
	var (
		nfd_namespace_file     = FixturePath("testdata", "psap", "nfd", "nfd-namespace.yaml")
		nfd_operatorgroup_file = FixturePath("testdata", "psap", "nfd", "nfd-operatorgroup.yaml")
		nfd_sub_file           = FixturePath("testdata", "psap", "nfd", "nfd-sub.yaml")
	)
	// check if NFD namespace already exists
	nsName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespace", nfdNamespace).Output()
	// if namespace exists, check if NFD is installed - exit if it is, continue with installation otherwise
	// if an error is thrown, namespace does not exist, create and continue with installation
	if strings.Contains(nsName, "NotFound") || strings.Contains(nsName, "No resources") || err != nil {
		e2e.Logf("NFD namespace not found - creating namespace and installing NFD ...")
		CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", nfd_namespace_file)
	} else {
		e2e.Logf("NFD namespace found - checking if NFD is installed ...")
	}

	ogName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("OperatorGroup", "openshift-nfd", "-n", nfdNamespace).Output()
	if strings.Contains(ogName, "NotFound") || strings.Contains(ogName, "No resources") || err != nil {
		// create NFD operator group from template
		ApplyNsResourceFromTemplate(oc, nfdNamespace, "--ignore-unknown-parameters=true", "-f", nfd_operatorgroup_file)
	} else {
		e2e.Logf("NFD operatorgroup found - continue to check subscription ...")
	}

	// get default channel and create subscription from template
	channel, err := GetOperatorPKGManifestDefaultChannel(oc, "nfd", "openshift-marketplace")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Channel: %v", channel)
	// get default channel and create subscription from template
	source, err := GetOperatorPKGManifestSource(oc, "nfd", "openshift-marketplace")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Source: %v", source)

	subName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("Subscription", "-n", nfdNamespace).Output()
	if strings.Contains(subName, "NotFound") || strings.Contains(subName, "No resources") || !strings.Contains(subName, "nfd") || err != nil {
		// create NFD operator group from template
		ApplyNsResourceFromTemplate(oc, nfdNamespace, "--ignore-unknown-parameters=true", "-f", nfd_sub_file, "-p", "CHANNEL="+channel, "SOURCE="+source)
	} else {
		e2e.Logf("NFD subscription found - continue to check pod status ...")
	}

	//Wait for NFD controller manager is ready
	WaitOprResourceReady(oc, "deployment", "nfd-controller-manager", nfdNamespace, false, false)

}

//Create NFD Instance in different namespace
func CreateNFDInstance(oc *CLI, namespace string) {

	var (
		nfd_instance_file = FixturePath("testdata", "psap", "nfd", "nfd-instance.yaml")
	)
	// get cluster version and create NFD instance from template
	clusterVersion, _, err := GetClusterVersion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Cluster Version: %v", clusterVersion)

	nfdinstanceName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("NodeFeatureDiscovery", "nfd-instance", "-n", namespace).Output()
	e2e.Logf("NFD Instance is: %v", nfdinstanceName)
	if strings.Contains(nfdinstanceName, "NotFound") || strings.Contains(nfdinstanceName, "No resources") || err != nil {
		// create NFD operator group from template
		ApplyNsResourceFromTemplate(oc, namespace, "--ignore-unknown-parameters=true", "-f", nfd_instance_file, "-p", "IMAGE=quay.io/openshift/origin-node-feature-discovery:"+clusterVersion, "NAMESPACE="+namespace)
	} else {
		e2e.Logf("NFD instance found - continue to check pod status ...")
	}

	//wait for NFD master and worker is ready
	WaitOprResourceReady(oc, "daemonset", "nfd-master", namespace, false, false)
	WaitOprResourceReady(oc, "daemonset", "nfd-worker", namespace, false, true)
}

//Get operator Packagemanifest source name
func GetOperatorPKGManifestSource(oc *CLI, pkgManifestName, namespace string) (string, error) {
	catalogSourceNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "-n", namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(catalogSourceNames, "qe-app-registry") || err != nil {
		//If the catalogsource qe-app-registry exist, prefer to use qe-app-registry, not use redhat-operators or certificate-operator ...
		return "qe-app-registry", nil
	} else {
		soureName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", pkgManifestName, "-n", namespace, "-o=jsonpath={.status.catalogSource}").Output()
		return soureName, err
	}
}

//Get operator Packagemanifest default channel
func GetOperatorPKGManifestDefaultChannel(oc *CLI, pkgManifestName, namespace string) (string, error) {
	channel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", pkgManifestName, "-n", namespace, "-o", "jsonpath={.status.defaultChannel}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return channel, err
}
