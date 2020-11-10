package securityandcompliance

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type complianceSuiteDescription struct {
	name                string
	namespace           string
	scanname            string
	scanType            string
	profile             string
	content             string
	contentImage        string
	rule                string
	debug               bool
	noExternalResources bool
	nodeSelector        string
	size                string
	tailoringConfigMap  string
	template            string
}

func (csuite *complianceSuiteDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", csuite.template, "-p", "NAME="+csuite.name, "NAMESPACE="+csuite.namespace,
		"SCANNAME="+csuite.scanname, "SCANTYPE="+csuite.scanType, "PROFILE="+csuite.profile, "CONTENT="+csuite.content, "CONTENTIMAGE="+csuite.contentImage,
		"RULE="+csuite.rule, "NOEXTERNALRESOURCES="+strconv.FormatBool(csuite.noExternalResources), "NODESELECTOR="+csuite.nodeSelector, "SIZE="+csuite.size,
		"TAILORCONFIGMAPNAME="+csuite.tailoringConfigMap)
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
	size         string
	template     string
}

func (cscan *complianceScanDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cscan.template, "-p", "NAME="+cscan.name,
		"NAMESPACE="+cscan.namespace, "SCANTYPE="+cscan.scanType, "PROFILE="+cscan.profile, "CONTENT="+cscan.content,
		"CONTENTIMAGE="+cscan.contentImage, "RULE="+cscan.rule, "NODESELECTOR="+cscan.nodeSelector, "SIZE="+cscan.size)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "compliancescan", cscan.name, requireNS, cscan.namespace))
}

func (cscan *complianceScanDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(cscan.name, "compliancescan", cscan.namespace)
}

type tailoredProfileDescription struct {
	name         string
	namespace    string
	extends      string
	enrulename1  string
	disrulename1 string
	disrulename2 string
	varname      string
	value        string
	template     string
}

func (tprofile *tailoredProfileDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", tprofile.template, "-p", "NAME="+tprofile.name, "NAMESPACE="+tprofile.namespace,
		"EXTENDS="+tprofile.extends, "ENRULENAME1="+tprofile.enrulename1, "DISRULENAME1="+tprofile.disrulename1, "DISRULENAME2="+tprofile.disrulename2,
		"VARNAME="+tprofile.varname, "VALUE="+tprofile.value)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "tailoredprofile", tprofile.name, requireNS, tprofile.namespace))
}

func (tprofile *tailoredProfileDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(tprofile.name, "tailoredprofile", tprofile.namespace)
}

func (csuite *complianceSuiteDescription) checkComplianceSuiteStatus(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", csuite.namespace, "compliancesuite", csuite.name, "-o=jsonpath={.status.phase}").Output()
		e2e.Logf("the result of complianceSuite:%v", output)
		if strings.Contains(output, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
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
	csuiteName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancesuite", "-o=jsonpath={.items[*].metadata.name}").Output()
	lines := strings.Fields(csuiteName)
	for _, line := range lines {
		if strings.Contains(line, expected) {
			e2e.Logf("\n%v\n\n", line)
			break
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) complianceScanName(oc *exutil.CLI, expected string) {
	cscanName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancescan", "-o=jsonpath={.items[*].metadata.name}").Output()
	lines := strings.Fields(cscanName)
	for _, line := range lines {
		if strings.Contains(line, expected) {
			e2e.Logf("\n%v\n\n", line)
			break
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) complianceSuiteResult(oc *exutil.CLI, expected string) {
	csuiteResult, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancesuite", "-o=jsonpath={.items[*].status.scanStatuses[0].result}").Output()
	lines := strings.Fields(csuiteResult)
	for _, line := range lines {
		if strings.Compare(line, expected) == 0 {
			e2e.Logf("\n%v\n\n", line)
			return
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) complianceScanResult(oc *exutil.CLI, expected string) {
	cscanResult, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancescan", "-o=jsonpath={.items[*].status.result}").Output()
	lines := strings.Fields(cscanResult)
	for _, line := range lines {
		if strings.Compare(line, expected) == 0 {
			e2e.Logf("\n%v\n\n", line)
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func (subD *subscriptionDescription) getScanExitCodeFromConfigmap(oc *exutil.CLI, expected string) {
	podName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pods", "--selector=workload=scanner", "-o=jsonpath={.items[*].metadata.name}").Output()
	lines := strings.Fields(podName)
	for _, line := range lines {
		cmCode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", line, "-n", subD.namespace, "-o=jsonpath={.data.exit-code}").Output()
		e2e.Logf("\n%v\n\n", cmCode)
		if strings.Contains(cmCode, expected) {
			break
		}
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func (subD *subscriptionDescription) getScanResultFromConfigmap(oc *exutil.CLI, expected string) {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pods", "--selector=workload=scanner", "-o=jsonpath={.items[0].metadata.name}").Output()
	cmMsg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "configmap", podName, "-o=jsonpath={.data.error-msg}").Output()
	e2e.Logf("\n%v\n\n", cmMsg)
	o.Expect(cmMsg).To(o.ContainSubstring(expected))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getPVCName(oc *exutil.CLI, expected string) {
	pvcName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pvc", "-o=jsonpath={.items[*].metadata.name}").Output()
	lines := strings.Fields(pvcName)
	for _, line := range lines {
		if strings.Contains(line, expected) {
			e2e.Logf("\n%v\n\n", line)
			break
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getPVCSize(oc *exutil.CLI, expected string) {
	pvcSize, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "pvc", "-o=jsonpath={.items[*].status.capacity.storage}").Output()
	lines := strings.Fields(pvcSize)
	for _, line := range lines {
		if strings.Contains(line, expected) {
			e2e.Logf("\n%v\n\n", line)
			break
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getRuleStatus(oc *exutil.CLI, expected string) {
	ruleName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancecheckresult", "-o=jsonpath={.items[0:5].metadata.name}").Output()
	lines := strings.Fields(ruleName)
	for _, line := range lines {
		e2e.Logf("\n%v\n\n", line)
		ruleStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("compliancecheckresult", line, "-n", subD.namespace, "-o=jsonpath={.status}").Output()
		if strings.Contains(ruleStatus, expected) {
			e2e.Logf("\n%v\n\n", ruleStatus)
			continue
		}
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func (subD *subscriptionDescription) getProfileBundleNameandStatus(oc *exutil.CLI, expected string) {
	pbName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "profilebundles", "-o=jsonpath={.items[*].metadata.name}").Output()
	lines := strings.Fields(pbName)
	for _, line := range lines {
		if strings.Compare(line, expected) == 0 {
			e2e.Logf("\n%v\n\n", line)
			// verify profilebundle status
			pbStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "profilebundles", line, "-o=jsonpath={.status.dataStreamStatus}").Output()
			o.Expect(pbStatus).To(o.ContainSubstring("VALID"))
			o.Expect(err).NotTo(o.HaveOccurred())
			return
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getTailoredProfileNameandStatus(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		tpName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "tailoredprofile", "-o=jsonpath={.items[*].metadata.name}").Output()
		lines := strings.Fields(tpName)
		for _, line := range lines {
			if strings.Compare(line, expected) == 0 {
				e2e.Logf("\n%v\n\n", line)
				// verify tailoredprofile status
				tpStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "tailoredprofile", line, "-o=jsonpath={.status.state}").Output()
				e2e.Logf("\n%v\n\n", tpStatus)
				o.Expect(tpStatus).To(o.ContainSubstring("READY"))
				o.Expect(err).NotTo(o.HaveOccurred())
				return true, nil
			}
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getProfileName(oc *exutil.CLI, expected string) {
	pName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "profile.compliance", "-o=jsonpath={.items[*].metadata.name}").Output()
	lines := strings.Fields(pName)
	for _, line := range lines {
		if strings.Compare(line, expected) == 0 {
			e2e.Logf("\n%v\n\n", line)
			return
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (subD *subscriptionDescription) getARFreportFromPVC(oc *exutil.CLI, expected string) {
	commands := []string{"exec", "pod/pv-extract", "--", "ls", "/workers-scan-results/0"}
	arfReport, err := oc.AsAdmin().Run(commands...).Args().Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	lines := strings.Fields(arfReport)
	for _, line := range lines {
		if strings.Contains(line, expected) {
			e2e.Logf("\n%v\n\n", line)
			break
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}
