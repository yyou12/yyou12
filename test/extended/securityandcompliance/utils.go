package securityandcompliance

import (
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type fileintegrity struct {
	name        string
	namespace   string
	configname  string
	configkey   string
	graceperiod int
	debug       bool
	template    string
}

type podModify struct {
	name      string
	namespace string
	nodeName  string
	args      string
	template  string
}

func (fi1 *fileintegrity) checkFileintegrityStatus(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", fi1.namespace, "-l app=aide-ds-example-fileintegrity",
			"-o=jsonpath={.items[*].status.containerStatuses[*].state}").Output()
		e2e.Logf("the result of checkFileintegrityStatus:%v", output)
		if strings.Contains(output, expected) && (!(strings.Contains(strings.ToLower(output), "error"))) && (!(strings.Contains(strings.ToLower(output), "crashLoopbackOff"))) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	fileintegrityYamle := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegrity", "-n", fi1.namespace, "-o yaml")
	e2e.Logf("the result of fileintegrityYamle:%v", fileintegrityYamle)
}

func (fi1 *fileintegrity) getConfigmapFromFileintegritynodestatus(oc *exutil.CLI, nodeName string) string {
	var cmName string
	var i int
	if i < 40 {
		cmName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", "-n", fi1.namespace, fi1.name+"-"+nodeName,
			"-o=jsonpath={.results[-1].resultConfigMapName}").Output()
		e2e.Logf("the result of cmName:%v", cmName)
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(cmName, "failed") {
			return cmName
		}
		time.Sleep(time.Duration(5) * time.Second)
		i++
	}
	return cmName
}

func (fi1 *fileintegrity) getDataFromConfigmap(oc *exutil.CLI, cmName string, expected string) {
	e2e.Logf("the result of cmName:%v", cmName)
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		aideResult, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap/"+cmName, "-n", fi1.namespace, "-o=jsonpath={.data}").Output()
		e2e.Logf("the result of aideResult:%v", aideResult)
		if strings.Contains(aideResult, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getOneWorkerNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l node-role.kubernetes.io/worker=",
		"-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of nodename:%v", nodeName)
	return nodeName
}

func (fi1 *fileintegrity) getOneFioPodName(oc *exutil.CLI) string {
	fioPodName, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-l file-integrity.openshift.io/pod=",
		"-n", fi1.namespace, "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	e2e.Logf("the result of fioPodName:%v", fioPodName)
	if strings.Compare(fioPodName, "") != 0 {
		return fioPodName
	}
	return fioPodName
}

func (fi1 *fileintegrity) checkKeywordNotExistInLog(oc *exutil.CLI, podName string, expected string) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		logs, err1 := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podName, "-n", fi1.namespace).Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		e2e.Logf("the result of logs:%v", logs)
		if strings.Compare(logs, "") != 0 && !strings.Contains(logs, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (fi1 *fileintegrity) checkKeywordExistInLog(oc *exutil.CLI, podName string, expected string) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		logs, err1 := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podName, "-n", fi1.namespace).Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		e2e.Logf("the result of logs:%v", logs)
		if strings.Contains(logs, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (fi1 *fileintegrity) checkArgsInPod(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		fioPodArgs, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-l file-integrity.openshift.io/pod=",
			"-n", fi1.namespace, "-o=jsonpath={.items[0].spec.containers[].args}").Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		e2e.Logf("the result of fioPodArgs: %v", fioPodArgs)
		if strings.Contains(fioPodArgs, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podModify) doActionsOnNode(oc *exutil.CLI, expected string, dr describerResrouce) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"NODENAME="+pod.nodeName, "PARAC="+pod.args)
		o.Expect(err1).NotTo(o.HaveOccurred())
		podModifyresult, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.status.phase}").Output()
		e2e.Logf("the result of pod %s: %v", pod.name, podModifyresult)
		if strings.Contains(podModifyresult, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (fi1 *fileintegrity) createFIOWithoutConfig(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace,
		"GRACEPERIOD="+strconv.Itoa(fi1.graceperiod), "DEBUG="+strconv.FormatBool(fi1.debug))
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "fileintegrity", fi1.name, requireNS, fi1.namespace))
}

func (fi1 *fileintegrity) createFIOWithoutKeyword(oc *exutil.CLI, itName string, dr describerResrouce, keyword string) {
	err := applyResourceFromTemplateWithoutKeyword(oc, keyword, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace,
		"CONFNAME="+fi1.configname, "CONFKEY="+fi1.configkey, "DEBUG="+strconv.FormatBool(fi1.debug))
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "fileintegrity", fi1.name, requireNS, fi1.namespace))
}

func (fi1 *fileintegrity) createFIOWithConfig(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace,
		"GRACEPERIOD="+strconv.Itoa(fi1.graceperiod), "DEBUG="+strconv.FormatBool(fi1.debug), "CONFNAME="+fi1.configname, "CONFKEY="+fi1.configkey)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "fileintegrity", fi1.name, requireNS, fi1.namespace))
}

func (sub *subscriptionDescription) checkPodFioStatus(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", sub.namespace, "-l", "name=file-integrity-operator",
			"-o=jsonpath={.items[*].status.containerStatuses[*].state}").Output()
		e2e.Logf("the result of checkPodFioStatus:%v", output)
		if strings.Contains(strings.ToLower(output), expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (fi1 *fileintegrity) createConfigmapFromFile(oc *exutil.CLI, itName string, dr describerResrouce, cmName string, aideKey string, aideFile string, expected string) (bool, error) {
	output, _ := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", cmName, "-n", fi1.namespace, "--from-file="+aideKey+"="+aideFile).Output()
	dr.getIr(itName).add(newResource(oc, "configmap", cmName, requireNS, fi1.namespace))
	e2e.Logf("the result of checkPodFioStatus:%v", output)
	if strings.Contains(strings.ToLower(output), expected) {
		return true, nil
	}
	return false, nil
}

func (fi1 *fileintegrity) checkConfigmapCreated(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", fi1.configname, "-n", fi1.namespace).Output()
		e2e.Logf("the result of checkConfigmapCreated:%v", output)
		if strings.Contains(output, fi1.configname) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (fi1 *fileintegrity) checkFileintegritynodestatus(oc *exutil.CLI, nodeName string, expected string) {
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", "-n", fi1.namespace, fi1.name+"-"+nodeName,
			"-o=jsonpath={.results[-1].condition}").Output()
		e2e.Logf("the result of checkFileintegritynodestatus:%v", output)
		if strings.Contains(output, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (fi1 *fileintegrity) checkOnlyOneDaemonset(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		daemonsetPodNumber, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("daemonset", "-n", fi1.namespace, "-o=jsonpath={.items[].status.numberReady}").Output()
		e2e.Logf("the result of daemonsetPodNumber:%v", daemonsetPodNumber)
		podNameString, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-l file-integrity.openshift.io/pod=", "-n", fi1.namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		e2e.Logf("the result of podNameString:%v", podNameString)
		intDaemonsetPodNumber, _ := strconv.Atoi(daemonsetPodNumber)
		intPodNumber := len(strings.Fields(podNameString))
		e2e.Logf("the result of intPodNumber:%v", intPodNumber)
		if intPodNumber == intDaemonsetPodNumber {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}
