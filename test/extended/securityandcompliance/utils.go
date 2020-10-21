package securityandcompliance

import (
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
	graceperiod int64
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
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "fileintegrity", fi1.name, requireNS, fi1.namespace))
}

func (fi1 *fileintegrity) createFIOWitConfig(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace,
		"CONFNAME="+fi1.configname, "CONFKEY="+fi1.configkey)
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
