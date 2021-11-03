package storage

import (
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	o "github.com/onsi/gomega"
)

// Execute command in node
func execCommandInSpecificNode(oc *exutil.CLI, nodeHostName string, command string) (string, error) {
	nodeHostName = "node/" + nodeHostName
	command1 := []string{nodeHostName, "-q", "--", "chroot", "/host", "bin/sh", "-c", command}
	msg, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(command1...).Output()
	if err != nil {
		e2e.Logf("Execute \""+command+"\" on node \"%s\" *failed with* : \"%v\".", nodeHostName, err)
		return msg, err
	} else {
		e2e.Logf("Executed \""+command+"\" on node \"%s\" *Successed* ", nodeHostName)
		debugLogf("Executed \""+command+"\" on node \"%s\" *Output is* : \"%v\".", nodeHostName, msg)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}

// Check the Volume mounted in the Node
func checkVolumeMountInNode(oc *exutil.CLI, volumeName string, nodeName string) {
	command := "mount | grep " + volumeName
	msg, err := execCommandInSpecificNode(oc, nodeName, command)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).To(o.ContainSubstring(volumeName))
}

// Check the Volume not mounted in the Node
func checkVolumeNotMountInNode(oc *exutil.CLI, volumeName string, nodeName string) {
	command := "mount | grep " + volumeName
	_, err := execCommandInSpecificNode(oc, nodeName, command)
	o.Expect(err).Should(o.HaveOccurred())
}

// Get the Node List for pod with label
func getNodeListForPodByLabel(oc *exutil.CLI, namespace string, labelName string) ([]string, error) {
	podsList, err := getPodsListByLabel(oc, namespace, labelName)
	o.Expect(err).NotTo(o.HaveOccurred())
	var nodeList []string
	for _, pod := range podsList {
		nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", pod, "-n", namespace, "-o=jsonpath={.spec.nodeName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s is on Node:\"%s\"", pod, nodeName)
		nodeList = append(nodeList, nodeName)
	}
	return nodeList, err
}

func getNodeNameByPod(oc *exutil.CLI, namespace string, podName string) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return nodeName
}
