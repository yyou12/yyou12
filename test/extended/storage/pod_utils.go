package storage

import (
	"fmt"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type pod struct {
	name      string
	namespace string
	pvcname   string
	template  string
	image     string
}

// function option mode to change the default values of pod parameters, e.g. name, namespace, persistent volume claim, image etc.
type podOption func(*pod)

// Replace the default value of pod name parameter
func setPodName(name string) podOption {
	return func(this *pod) {
		this.name = name
	}
}

// Replace the default value of pod template parameter
func setPodTemplate(template string) podOption {
	return func(this *pod) {
		this.template = template
	}
}

// Replace the default value of pod namespace parameter
func setPodNamespace(namespace string) podOption {
	return func(this *pod) {
		this.namespace = namespace
	}
}

// Replace the default value of pod persistent volume claim parameter
func setPodPersistentVolumeClaim(pvcname string) podOption {
	return func(this *pod) {
		this.pvcname = pvcname
	}
}

// Replace the default value of pod image parameter
func setPodImage(image string) podOption {
	return func(this *pod) {
		this.image = image
	}
}

//  Create a new customized pod object
func newPod(opts ...podOption) pod {
	defaultPod := pod{
		name:      "mypod-" + getRandomString(),
		template:  "pod-template.yaml",
		namespace: "default",
		pvcname:   "mypvc",
		image:     "quay.io/openshifttest/storage@sha256:a05b96d373be86f46e76817487027a7f5b8b5f87c0ac18a246b018df11529b40",
	}

	for _, o := range opts {
		o(&defaultPod)
	}

	return defaultPod
}

// Create new pod with customized parameters
func (pod *pod) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "PODNAME="+pod.name, "PODNAMESPACE="+pod.namespace, "PVCNAME="+pod.pvcname, "PODIMAGE="+pod.image)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete specified pod
func (pod *pod) delete(oc *exutil.CLI) {
	oc.WithoutNamespace().Run("delete").Args("pod", pod.name, "-n", pod.namespace).Execute()
}

//  Get the phase, status of specified pod
func getPodStatus(oc *exutil.CLI, namespace string, podName string) (string, error) {
	podStatus, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status in namespace %s is %q", podName, namespace, podStatus)
	return podStatus, err
}

//  Check the pod status becomes ready, status is "Running", "Ready" or "Complete"
func checkPodReady(oc *exutil.CLI, namespace string, podName string) (bool, error) {
	podOutPut, err := getPodStatus(oc, namespace, podName)
	status := []string{"Running", "Ready", "Complete"}
	return contains(status, podOutPut), err
}

//  Get the detail info of specified pod
func describePod(oc *exutil.CLI, namespace string, podName string) string {
	podDescribe, err := oc.WithoutNamespace().Run("describe").Args("pod", "-n", namespace, podName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status is %q", podName, podDescribe)
	return podDescribe
}

//  Waiting for the pod becomes ready, such as "Running", "Ready", "Complete"
func waitPodReady(oc *exutil.CLI, namespace string, podName string) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		status, err1 := checkPodReady(oc, namespace, podName)
		if err1 != nil {
			e2e.Logf("the err:%v, wait for pod %v to become ready.", err1, podName)
			return status, err1
		}
		if !status {
			return status, nil
		}
		return status, nil
	})

	if err != nil {
		podDescribe := describePod(oc, namespace, podName)
		e2e.Logf("oc describe pod %v.", podName)
		e2e.Logf(podDescribe)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s not ready %v", podName))
}

//  Specified pod exec the bash CLI
func execCommandInSpecificPod(oc *exutil.CLI, namespace string, podName string, command string) (string, error) {
	e2e.Logf("The command is: %v", command)
	command1 := []string{"-n", namespace, podName, "--", "/bin/sh", "-c", command}
	msg, err := oc.WithoutNamespace().Run("exec").Args(command1...).Output()
	if err != nil {
		e2e.Logf("Execute command failed with  err:%v .", err)
		return msg, err
	} else {
		e2e.Logf("Execute command output is:\"%v\"", msg)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}
