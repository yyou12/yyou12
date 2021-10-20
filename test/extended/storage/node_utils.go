package storage

import (
	"strings"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	o "github.com/onsi/gomega"
)

// Execute command in node
func execCommandInSpecificNode(oc *exutil.CLI, nodeHostName string, command string) (string, error) {
	nodeHostName = "node/" + nodeHostName
	command1 := []string{nodeHostName, "--", "chroot", "/host", "bin/sh", "-c", command}
	msg, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(command1...).Output()
	if err != nil {
		e2e.Logf("Execute command failed with err:%v .", err)
		return msg, err
	} else {
		e2e.Logf("Executed command successfully on node %s with command and msg: %s", nodeHostName, command1, msg)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}

// To check Volume Mount in Node
func checkVolumeMountInNode(oc *exutil.CLI, namespace string, nodeHostName []string, pvcname string, command string) (string, error) {
	volumeName, err := getPersistentVolumeNameFromPersistentVolumeClaim(oc, namespace, pvcname)
	var nodeflag int
	var data string
	command = command + volumeName
	e2e.Logf("Command to be executed on Node %s", command)
	for _, nodeHostName := range nodeHostName {
		msg, err := execCommandInSpecificNode(oc, nodeHostName, command)
		if err != nil {
			nodeflag = 1
		} else {
			data = msg
		}
	}
	if nodeflag == 0 {
		e2e.Logf("Checked volume mount inside node successfully %s\n", data)
		return data, nil
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return volumeName, err
}

// Get the Node List for pod with label
func getNodeListForPodByLabel(oc *exutil.CLI, namespace string, labelName string) ([]string, error) {
	podsList, err := getPodsListByLabel(oc, namespace, labelName)
	var nodeflag int
	var nodeList []string
	for _, pod := range podsList {
		nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", pod, "-n", namespace, "-o=jsonpath={.spec.nodeName}").Output()
		if err != nil {
			e2e.Logf("Get nodeName for the pod failed with  err:%v .", err)
			nodeflag = 1
		} else {
			e2e.Logf("%s is Nodename for pod :%s\n", nodeName, pod)
			if !strings.Contains(strings.Join(nodeList, ","), nodeName) {
				nodeList = append(nodeList, nodeName)
			}
		}
	}
	if nodeflag == 0 {
		e2e.Logf("Node Lists=%s for deployment with labelName=%s\n", nodeList, labelName)
		return nodeList, nil
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return podsList, err
}
