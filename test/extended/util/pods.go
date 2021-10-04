package util

import (
	"fmt"
	exutil "github.com/openshift/openshift-tests/test/extended/util"
	"os/exec"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"
)

// WaitForNoPodsAvailable waits until there are no pods in the
// given namespace
func WaitForNoPodsAvailable(oc *CLI) error {
	return wait.Poll(200*time.Millisecond, 3*time.Minute, func() (bool, error) {
		pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == 0, nil
	})
}

// RemovePodsWithPrefixes deletes pods whose name begins with the
// supplied prefixes
func RemovePodsWithPrefixes(oc *CLI, prefixes ...string) error {
	e2e.Logf("Removing pods from namespace %s with prefix(es): %v", oc.Namespace(), prefixes)
	pods, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	errs := []error{}
	for _, prefix := range prefixes {
		for _, pod := range pods.Items {
			if strings.HasPrefix(pod.Name, prefix) {
				if err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
					e2e.Logf("unable to remove pod %s/%s", oc.Namespace(), pod.Name)
					errs = append(errs, err)
				}
			}
		}
	}
	if len(errs) > 0 {
		return kutilerrors.NewAggregate(errs)
	}
	return nil
}

// CreateCentosExecPodOrFail creates a centos:7 pause pod used as a vessel for kubectl exec commands.
// Pod name is uniquely generated.
func CreateCentosExecPodOrFail(client kubernetes.Interface, ns, generateName string, tweak func(*v1.Pod)) *v1.Pod {
	return pod.CreateExecPodOrFail(client, ns, generateName, func(pod *v1.Pod) {
		pod.Spec.Containers[0].Image = "centos:7"
		pod.Spec.Containers[0].Command = []string{"sh", "-c", "trap exit TERM; while true; do sleep 5; done"}
		pod.Spec.Containers[0].Args = nil

		if tweak != nil {
			tweak(pod)
		}
	})
}

func remoteShPod(oc *CLI, namespace string, podName string, needBash bool, needChroot bool, cmd ...string) (string, error) {
	var cargs []string
	if needBash {
		cargs = []string{"-n", namespace, podName, "bash", "-c"}
	} else if needChroot {
		cargs = []string{"-n", namespace, podName, "chroot", "/rootfs"}
	} else {
		cargs = []string{"-n", namespace, podName}
	}
	cargs = append(cargs, cmd...)
	return oc.AsAdmin().WithoutNamespace().Run("rsh").Args(cargs...).Output()
}

// RemoteShPod creates a remote shell of the pod
func RemoteShPod(oc *CLI, namespace string, podName string, cmd ...string) (string, error) {
	return remoteShPod(oc, namespace, podName, false, false, cmd...)
}

// RemoteShPodWithChroot creates a remote shell of the pod with chroot
func RemoteShPodWithChroot(oc *CLI, namespace string, podName string, cmd ...string) (string, error) {
	return remoteShPod(oc, namespace, podName, false, true, cmd...)
}

// RemoteShPodWithBash creates a remote shell of the pod with bash
func RemoteShPodWithBash(oc *CLI, namespace string, podName string, cmd ...string) (string, error) {
	return remoteShPod(oc, namespace, podName, true, false, cmd...)
}

// WaitAndGetSpecificPodLogs wait and return the pod logs by the specific filter
func WaitAndGetSpecificPodLogs(oc *CLI, namespace string, container string, podName string, filter string) (string, error) {
	logs, err := GetSpecificPodLogs(oc, namespace, container, podName, filter)
	if err != nil {
		waitErr := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			stdout, err := GetSpecificPodLogs(oc, namespace, container, podName, filter)
			if err != nil {
				e2e.Logf("the err:%v, and try next round", err)
				return false, nil
			}
			if strings.Contains(stdout, filter) {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("Pod logs does not contain %s", filter))
	}
	return logs, err
}

// GetSpecificPodLogs returns the pod logs by the specific filter
func GetSpecificPodLogs(oc *CLI, namespace string, container string, podName string, filter string) (string, error) {
	var cargs []string
	if len(container) > 0 {
		cargs = []string{"-n", namespace, "-c", container, podName}
	} else {
		cargs = []string{"-n", namespace, podName}
	}
	podLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(cargs...).OutputToFile("podLogs.txt")
	if err != nil {
		e2e.Logf("unable to get the pod (%s) logs", podName)
		return podLogs, err
	}
	var filterCmd = ""
	if len(filter) > 0 {
		filterCmd = " | grep -i " + filter
	}
	filteredLogs, err := exec.Command("bash", "-c", "cat "+podLogs+filterCmd).Output()
	return string(filteredLogs), err
}

// GetPodName returns the pod name
func GetPodName(oc *CLI, namespace string, podLabel string, node string) (string, error) {
	args := []string{"pods", "-n", namespace, "-l", podLabel,
		"--field-selector", "spec.nodeName=" + node, "-o", "jsonpath='{..metadata.name}'"}
	daemonPod, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Output()
	return strings.ReplaceAll(daemonPod, "'", ""), err
}
