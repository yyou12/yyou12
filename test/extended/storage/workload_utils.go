package storage

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Pod workload related functions
type pod struct {
	name      string
	namespace string
	pvcname   string
	template  string
	image     string
	mountPath string
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

// Replace the default value of pod image parameter
func setPodMountPath(mountPath string) podOption {
	return func(this *pod) {
		this.mountPath = mountPath
	}
}

//  Create a new customized pod object
func newPod(opts ...podOption) pod {
	defaultPod := pod{
		name:      "mypod-" + getRandomString(),
		template:  "pod-template.yaml",
		namespace: "",
		pvcname:   "mypvc",
		image:     "quay.io/openshifttest/storage@sha256:a05b96d373be86f46e76817487027a7f5b8b5f87c0ac18a246b018df11529b40",
		mountPath: "/mnt/storage",
	}

	for _, o := range opts {
		o(&defaultPod)
	}

	return defaultPod
}

// Create new pod with customized parameters
func (pod *pod) create(oc *exutil.CLI) {
	if pod.namespace == "" {
		pod.namespace = oc.Namespace()
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "PODNAME="+pod.name, "PODNAMESPACE="+pod.namespace, "PVCNAME="+pod.pvcname, "PODIMAGE="+pod.image, "PODMOUNTPATH="+pod.mountPath)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Create new pod with extra parameters
func (pod *pod) createWithReadOnlyVolume(oc *exutil.CLI) {
	extraParameters := map[string]interface{}{
		"jsonPath": `items.0.spec.containers.0.volumeMounts.0.`,
		"readOnly": true,
	}
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "PODNAME="+pod.name, "PODNAMESPACE="+pod.namespace, "PVCNAME="+pod.pvcname, "PODIMAGE="+pod.image, "PODMOUNTPATH="+pod.mountPath)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the pod
func (pod *pod) delete(oc *exutil.CLI) {
	err := oc.WithoutNamespace().Run("delete").Args("pod", pod.name, "-n", pod.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the pod use kubeadmin
func (pod *pod) deleteAsAdmin(oc *exutil.CLI) {
	oc.WithoutNamespace().AsAdmin().Run("delete").Args("pod", pod.name, "-n", pod.namespace).Execute()
}

//  Pod exec the bash CLI
func (pod *pod) execCommand(oc *exutil.CLI, command string) (string, error) {
	command1 := []string{"-n", pod.namespace, pod.name, "--", "/bin/sh", "-c", command}
	msg, err := oc.WithoutNamespace().Run("exec").Args(command1...).Output()
	if err != nil {
		e2e.Logf(pod.name+"# "+command+" *failed with* :\"%v\".", err)
		return msg, err
	}
	debugLogf(pod.name+"# "+command+" *Output is* :\"%s\".", msg)
	return msg, nil
}

//  Check the pod mounted volume could read and write
func (pod *pod) checkMountedVolumeCouldRW(oc *exutil.CLI) {
	_, err := execCommandInSpecificPod(oc, pod.namespace, pod.name, "echo \"storage test\" >"+pod.mountPath+"/testfile")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(execCommandInSpecificPod(oc, pod.namespace, pod.name, "cat "+pod.mountPath+"/testfile")).To(o.ContainSubstring("storage test"))
}

//  Check the pod mounted volume have exec right
func (pod *pod) checkMountedVolumeHaveExecRight(oc *exutil.CLI) {
	_, err := execCommandInSpecificPod(oc, pod.namespace, pod.name, "cp hello "+pod.mountPath)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(execCommandInSpecificPod(oc, pod.namespace, pod.name, pod.mountPath+"/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))
}

// Waiting for the Pod ready
func (pod *pod) waitReady(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		status, err1 := checkPodReady(oc, pod.namespace, pod.name)
		if err1 != nil {
			e2e.Logf("the err:%v, wait for pod %v to become ready.", err1, pod.name)
			return status, err1
		}
		if !status {
			return status, nil
		}
		return status, nil
	})

	if err != nil {
		podDescribe := describePod(oc, pod.namespace, pod.name)
		e2e.Logf("oc describe pod %v.", pod.name)
		e2e.Logf(podDescribe)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s not ready", pod.name))
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
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s not ready", podName))
}

//  Specified pod exec the bash CLI
func execCommandInSpecificPod(oc *exutil.CLI, namespace string, podName string, command string) (string, error) {
	command1 := []string{"-n", namespace, podName, "--", "/bin/sh", "-c", command}
	msg, err := oc.WithoutNamespace().Run("exec").Args(command1...).Output()
	if err != nil {
		e2e.Logf(podName+"# "+command+" *failed with* :\"%v\".", err)
		return msg, err
	} else {
		e2e.Logf(podName+"# "+command+" *Output is* :\"%s\".", msg)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}

// Wait for pods selected with selector name to be removed
func WaitUntilPodsAreGoneByLabel(oc *exutil.CLI, namespace string, labelName string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.WithoutNamespace().Run("get").Args("pods", "-l", labelName, "-n", namespace).Output()
		if err != nil {
			return false, err
		} else {
			errstring := fmt.Sprintf("%v", output)
			if strings.Contains(errstring, "No resources found") {
				e2e.Logf(output)
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Error waiting for pods to be removed using labelName  %s", labelName))
}

// Get the pod details
func getPodDetailsByLabel(oc *exutil.CLI, namespace string, labelName string) (string, error) {
	output, err := oc.WithoutNamespace().Run("get").Args("pods", "-l", labelName, "-n", namespace).Output()
	if err != nil {
		e2e.Logf("Get pod details failed with  err:%v .", err)
		return output, err
	} else {
		e2e.Logf("Get pod details output is:\"%v\"", output)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return output, nil
}

// Get the pods List by label
func getPodsListByLabel(oc *exutil.CLI, namespace string, labelName string) ([]string, error) {
	podsOp, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", labelName, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Fields(podsOp), err
}

// Get the pod status by label, Checking status for n numbers of deployments
func checkPodStatusByLabel(oc *exutil.CLI, namespace string, labelName string, expectedstatus string) {
	var podDescribe string
	podsList, _ := getPodsListByLabel(oc, namespace, labelName)
	e2e.Logf("PodLabelName \"%s\", expected status is \"%s\", podsList=%s", labelName, expectedstatus, podsList)
	err := wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		podflag := 0
		for _, pod := range podsList {
			podstatus, err := oc.WithoutNamespace().Run("get").Args("pod", pod, "-n", namespace, "-o=jsonpath={.status.phase}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if matched, _ := regexp.MatchString(expectedstatus, podstatus); !matched {
				podDescribe = describePod(oc, namespace, pod)
				podflag = 1
			}
		}
		if podflag == 1 {
			return false, nil
		} else {
			e2e.Logf("%s is with expected status: \"%s\"", podsList, expectedstatus)
			return true, nil
		}
	})
	if err != nil && podDescribe != "" {
		e2e.Logf(podDescribe)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod with label %s not ready", labelName))
}

//  Specified pod exec the bash CLI
func execCommandInSpecificPodWithLabel(oc *exutil.CLI, namespace string, labelName string, command string) (string, error) {
	podsList, err := getPodsListByLabel(oc, namespace, labelName)
	e2e.Logf("Pod List is %s.", podsList)
	podflag := 0
	var data, podDescribe string
	for _, pod := range podsList {
		command1 := []string{pod, "-n", namespace, "--", "/bin/sh", "-c", command}
		msg, err := oc.WithoutNamespace().Run("exec").Args(command1...).Output()
		if err != nil {
			e2e.Logf("Execute command failed with  err: %v.", err)
			podDescribe = describePod(oc, namespace, pod)
			podflag = 1
		} else {
			e2e.Logf("Executed \"%s\" on pod \"%s\" result: %s", command1, pod, msg)
			data = msg
		}
	}
	if podflag == 0 {
		e2e.Logf("Executed commands on Pods labeled %s successfully", labelName)
		return data, nil
	}
	if err != nil && podDescribe != "" {
		e2e.Logf(podDescribe)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Join(podsList, " "), err
}

// Deployment workload related functions
type deployment struct {
	name       string
	namespace  string
	replicasno string
	applabel   string
	mpath      string
	pvcname    string
	template   string
	volumetype string
	typepath   string
}

// function option mode to change the default value of deployment parameters,eg. name, replicasno, mpath
type deployOption func(*deployment)

// Replace the default value of Deployment name parameter
func setDeploymentName(name string) deployOption {
	return func(this *deployment) {
		this.name = name
	}
}

// Replace the default value of Deployment template parameter
func setDeploymentTemplate(template string) deployOption {
	return func(this *deployment) {
		this.template = template
	}
}

// Replace the default value of Deployment namespace parameter
func setDeploymentNamespace(namespace string) deployOption {
	return func(this *deployment) {
		this.namespace = namespace
	}
}

// Replace the default value of Deployment replicasno parameter
func setDeploymentReplicasNumber(replicasno string) deployOption {
	return func(this *deployment) {
		this.replicasno = replicasno
	}
}

// Replace the default value of Deployment app label
func setDeploymentApplabel(applabel string) deployOption {
	return func(this *deployment) {
		this.applabel = applabel
	}
}

// Replace the default value of Deployment mountpath parameter
func setDeploymentMountpath(mpath string) deployOption {
	return func(this *deployment) {
		this.mpath = mpath
	}
}

// Replace the default value of Deployment pvcname parameter
func setDeploymentPVCName(pvcname string) deployOption {
	return func(this *deployment) {
		this.pvcname = pvcname
	}
}

// Replace the default value of Deployment volume type parameter
func setDeploymentVolumeType(volumetype string) deployOption {
	return func(this *deployment) {
		this.volumetype = volumetype
	}
}

// Replace the default value of Deployment volume type path parameter
func setDeploymentVolumeTypePath(typepath string) deployOption {
	return func(this *deployment) {
		this.typepath = typepath
	}
}

//  Create a new customized Deployment object
func newDeployment(opts ...deployOption) deployment {
	defaultDeployment := deployment{
		name:       "my-dep-" + getRandomString(),
		template:   "dep-template.yaml",
		namespace:  "",
		replicasno: "1",
		applabel:   "myapp-" + getRandomString(),
		mpath:      "/mnt/storage",
		pvcname:    "my-pvc",
		volumetype: "volumeMounts",
		typepath:   "mountPath",
	}

	for _, o := range opts {
		o(&defaultDeployment)
	}

	return defaultDeployment
}

// Create new Deployment with customized parameters
func (dep *deployment) create(oc *exutil.CLI) {
	if dep.namespace == "" {
		dep.namespace = oc.Namespace()
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", dep.template, "-p", "DNAME="+dep.name, "DNAMESPACE="+dep.namespace, "PVCNAME="+dep.pvcname, "REPLICASNUM="+dep.replicasno, "DLABEL="+dep.applabel, "MPATH="+dep.mpath, "VOLUMETYPE="+dep.volumetype, "TYPEPATH="+dep.typepath)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete Deployment from the namespace
func (dep *deployment) delete(oc *exutil.CLI) {
	err := oc.WithoutNamespace().Run("delete").Args("deployment", dep.name, "-n", dep.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete Deployment from the namespace
func (dep *deployment) deleteAsAdmin(oc *exutil.CLI) {
	oc.WithoutNamespace().AsAdmin().Run("delete").Args("deployment", dep.name, "-n", dep.namespace).Execute()
}

// Get deployment pod list
func (dep *deployment) getPodList(oc *exutil.CLI) []string {
	output, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", dep.namespace, "-l", "app="+dep.applabel, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(output, " ")
}

// Scale Replicas for the Deployment
func (dep *deployment) scaleReplicas(oc *exutil.CLI, replicasno string) {
	err := oc.WithoutNamespace().Run("scale").Args("deployment", dep.name, "--replicas="+replicasno, "-n", dep.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	dep.replicasno = replicasno
}

// Check the deployment ready
func (dep *deployment) checkReady(oc *exutil.CLI) (bool, error) {
	readyReplicas, err := oc.WithoutNamespace().Run("get").Args("deployment", dep.name, "-n", dep.namespace, "-o", "jsonpath={.status.availableReplicas}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if dep.replicasno == "0" && readyReplicas == "" {
		readyReplicas = "0"
	}
	return strings.EqualFold(dep.replicasno, readyReplicas), err
}

// Describe the deployment
func (dep *deployment) describe(oc *exutil.CLI) string {
	deploymentDescribe, err := oc.WithoutNamespace().Run("describe").Args("deployment", dep.name, "-n", dep.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return deploymentDescribe
}

// Waiting the deployment become ready
func (dep *deployment) waitReady(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		deploymentReady, err := dep.checkReady(oc)
		if err != nil {
			return deploymentReady, err
		}
		if !deploymentReady {
			return deploymentReady, nil
		}
		e2e.Logf(dep.name + " availableReplicas is as expected")
		return deploymentReady, nil
	})

	if err != nil {
		e2e.Logf("oc describe pod %v.", dep.name)
		e2e.Logf(dep.describe(oc))
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Deployment %s not ready", dep.name))
}

// Check the deployment mounted volume could read and write
func (dep *deployment) checkPodMountedVolumeCouldRW(oc *exutil.CLI) {
	for _, podinstance := range dep.getPodList(oc) {
		content := "storage test " + getRandomString()
		randomFileName := "/testfile_" + getRandomString()
		_, err := execCommandInSpecificPod(oc, dep.namespace, podinstance, "echo "+content+">"+dep.mpath+randomFileName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(execCommandInSpecificPod(oc, dep.namespace, podinstance, "cat "+dep.mpath+randomFileName)).To(o.ContainSubstring(content))
	}
}

// Get the deployment data written from checkPodMountedVolumeCouldRW
func (dep *deployment) getPodMountedVolumeData(oc *exutil.CLI) {
	for _, podinstance := range dep.getPodList(oc) {
		o.Expect(execCommandInSpecificPod(oc, dep.namespace, podinstance, "cat "+dep.mpath+"/testfile_*")).To(o.ContainSubstring("storage test"))
	}
}

// Check the deployment mounted volume have exec right
func (dep *deployment) checkPodMountedVolumeHaveExecRight(oc *exutil.CLI) {
	for _, podinstance := range dep.getPodList(oc) {
		_, err := execCommandInSpecificPod(oc, dep.namespace, podinstance, "cp hello "+dep.mpath)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(execCommandInSpecificPod(oc, dep.namespace, podinstance, dep.mpath+"/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))
	}
}

// Check the deployment mounted volume type
func (dep *deployment) checkPodMountedVolumeContain(oc *exutil.CLI, content string) {
	for _, podinstance := range dep.getPodList(oc) {
		output, err := execCommandInSpecificPod(oc, dep.namespace, podinstance, "mount | grep "+dep.mpath)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(content))
	}
}

// Write data in block level
func writeDataBlockType(oc *exutil.CLI, dep deployment) {
	e2e.Logf("Writing the data as Block level")
	_, err := execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "/bin/dd  if=/dev/null of="+dep.mpath+" bs=512 count=1")
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "echo 'test data' > "+dep.mpath)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Check data written
func checkDataBlockType(oc *exutil.CLI, dep deployment) {
	_, err := execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "/bin/dd if="+dep.mpath+" of=/tmp/testfile bs=512 count=1")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "cat /tmp/testfile | grep 'test data' ")).To(o.ContainSubstring("matches"))
}
