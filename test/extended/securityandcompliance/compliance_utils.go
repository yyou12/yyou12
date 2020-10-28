package securityandcompliance

import (
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type complianceSuiteDescription struct {
	name         string
	namespace    string
	scanname     string
	scanType     string
	profile      string
	content      string
	contentImage string
	rule         string
	debug        bool
	nodeSelector string
	template     string
}

func (csuite *complianceSuiteDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", csuite.template, "-p", "NAME="+csuite.name, "NAMESPACE="+csuite.namespace,
		"SCANNAME="+csuite.scanname, "SCANTYPE="+csuite.scanType, "PROFILE="+csuite.profile, "CONTENT="+csuite.content, "CONTENTIMAGE="+csuite.contentImage,
		"RULE="+csuite.rule, "NODESELECTOR="+csuite.nodeSelector)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "compliancesuite", csuite.name, requireNS, csuite.namespace))
}

func (csuite *complianceSuiteDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(csuite.name, "compliancesuite", csuite.namespace)
}

type complianceScanDescription struct {
	name         string
	namespace    string
	scanType     string
	profile      string
	content      string
	contentImage string
	rule         string
	debug        bool
	nodeSelector string
	template     string
}

func (cscan *complianceScanDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cscan.template, "-p", "NAME="+cscan.name,
		"NAMESPACE="+cscan.namespace, "SCANTYPE="+cscan.scanType, "PROFILE="+cscan.profile, "CONTENT="+cscan.content,
		"CONTENTIMAGE="+cscan.contentImage, "RULE="+cscan.rule, "NODESELECTOR="+cscan.nodeSelector)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "compliancescan", cscan.name, requireNS, cscan.namespace))
}

func (cscan *complianceScanDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(cscan.name, "compliancescan", cscan.namespace)
}

func setLabelToNode(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=,node.openshift.io/os_id=rhcos",
		"-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	node := strings.Fields(nodeName)
	for _, v := range node {
		nodeLabel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "--show-labels").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(nodeLabel, "node-role.kubernetes.io/wscan=") {
			continue
		} else {
			_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args("node", fmt.Sprintf("%s", v), "node-role.kubernetes.io/wscan=").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		return nodeLabel
	}
	return nodeName
}

func (subD *subscriptionDescription) scanPodName(oc *exutil.CLI, expected string) {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pods", "--selector=workload=scanner", "-o=jsonpath={.items[*].metadata.name}").Output()
	e2e.Logf("\n%v\n", podName)
	o.Expect(err).NotTo(o.HaveOccurred())
	pods := strings.Fields(podName)
	for _, pod := range pods {
		if strings.Contains(pod, expected) {
			continue
		}
	}
}

func (subD *subscriptionDescription) scanPodStatus(oc *exutil.CLI, expected string) {
	podStat, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pods", "--selector=workload=scanner", "-o=jsonpath={.items[*].status.phase}").Output()
	e2e.Logf("\n%v\n", podStat)
	o.Expect(err).NotTo(o.HaveOccurred())
	lines := strings.Fields(podStat)
	for _, line := range lines {
		if strings.Contains(line, expected) {
			continue
		} else {
			e2e.Failf("Compliance scan failed on one or more nodes")
		}
	}
}

func (subD *subscriptionDescription) complianceSuiteName(oc *exutil.CLI, expected string) {
	scName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancesuite", "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(scName).To(o.ContainSubstring(expected))
	e2e.Logf("\n%v\n\n", scName)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) complianceScanName(oc *exutil.CLI, expected string) {
	scName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancescan", "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(scName).To(o.ContainSubstring(expected))
	e2e.Logf("\n%v\n\n", scName)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) complianceSuiteResult(oc *exutil.CLI, expected string) {
	coStat, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancesuite", "-o=jsonpath={.items[0].status.scanStatuses[0].result}").Output()
	o.Expect(coStat).To(o.ContainSubstring(expected))
	e2e.Logf("\n%v\n\n", coStat)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) complianceScanResult(oc *exutil.CLI, expected string) {
	coStat, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancescan", "-o=jsonpath={.items[0].status.result}").Output()
	o.Expect(coStat).To(o.ContainSubstring(expected))
	e2e.Logf("\n%v\n\n", coStat)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getScanExitCodeFromConfigmap(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		podName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pods", "--selector=workload=scanner", "-o=jsonpath={.items[0].metadata.name}").Output()
		cmCode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", podName, "-n", subD.namespace, "-o=jsonpath={.data.exit-code}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\n%v\n\n", cmCode)
		if strings.Contains(cmCode, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getScanResultFromConfigmap(oc *exutil.CLI, expected string) {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pods", "--selector=workload=scanner", "-o=jsonpath={.items[0].metadata.name}").Output()
	cmMsg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "configmap", podName, "-o=jsonpath={.data.error-msg}").Output()
	e2e.Logf("\n%v\n\n", cmMsg)
	o.Expect(cmMsg).To(o.ContainSubstring(expected))
	o.Expect(err).NotTo(o.HaveOccurred())
}
