package securityandcompliance

import (
	"fmt"
	"os/exec"
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
	schedule            string
	scanname            string
	scanType            string
	profile             string
	content             string
	contentImage        string
	rule                string
	debug               bool
	noExternalResources bool
	key                 string
	value               string
	operator            string
	nodeSelector        string
	pvAccessModes       string
	size                string
	rotation            int
	storageClassName    string
	tailoringConfigMap  string
	template            string
}

type profileBundleDescription struct {
	name         string
	namespace    string
	contentimage string
	contentfile  string
	template     string
}

type scanSettingDescription struct {
	autoapplyremediations bool
	name                  string
	namespace             string
	roles1                string
	roles2                string
	rotation              int
	schedule              string
	size                  string
	template              string
}

type scanSettingBindingDescription struct {
	name            string
	namespace       string
	profilekind1    string
	profilename1    string
	profilename2    string
	scansettingname string
	template        string
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

type tailoredProfileWithoutVarDescription struct {
	name         string
	namespace    string
	extends      string
	enrulename1  string
	enrulename2  string
	disrulename1 string
	disrulename2 string
	template     string
}

type objectTableRef struct {
	kind      string
	namespace string
	name      string
}

type complianceScanDescription struct {
	name             string
	namespace        string
	scanType         string
	profile          string
	content          string
	contentImage     string
	rule             string
	debug            bool
	key              string
	value            string
	operator         string
	key1             string
	value1           string
	operator1        string
	nodeSelector     string
	pvAccessModes    string
	size             string
	storageClassName string
	template         string
}

type storageClassDescription struct {
	name              string
	provisioner       string
	reclaimPolicy     string
	volumeBindingMode string
	template          string
}

func (csuite *complianceSuiteDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", csuite.template, "-p", "NAME="+csuite.name, "NAMESPACE="+csuite.namespace,
		"SCHEDULE="+csuite.schedule, "SCANNAME="+csuite.scanname, "SCANTYPE="+csuite.scanType, "PROFILE="+csuite.profile, "CONTENT="+csuite.content,
		"CONTENTIMAGE="+csuite.contentImage, "RULE="+csuite.rule, "NOEXTERNALRESOURCES="+strconv.FormatBool(csuite.noExternalResources), "KEY="+csuite.key,
		"VALUE="+csuite.value, "OPERATOR="+csuite.operator, "NODESELECTOR="+csuite.nodeSelector, "PVACCESSMODE="+csuite.pvAccessModes, "STORAGECLASSNAME="+csuite.storageClassName,
		"SIZE="+csuite.size, "ROTATION="+strconv.Itoa(csuite.rotation), "TAILORCONFIGMAPNAME="+csuite.tailoringConfigMap)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "compliancesuite", csuite.name, requireNS, csuite.namespace))
}

func (pb *profileBundleDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pb.template, "-p", "NAME="+pb.name, "NAMESPACE="+pb.namespace,
		"CONTENIMAGE="+pb.contentimage, "CONTENTFILE="+pb.contentfile)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "profilebundle", pb.name, requireNS, pb.namespace))
}

func (ss *scanSettingDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ss.template, "-p", "NAME="+ss.name, "NAMESPACE="+ss.namespace,
		"AUTOAPPLYREMEDIATIONS="+strconv.FormatBool(ss.autoapplyremediations), "SCHEDULE="+ss.schedule, "SIZE="+ss.size, "ROTATION="+strconv.Itoa(ss.rotation),
		"ROLES1="+ss.roles1, "ROLES2="+ss.roles2)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "scansetting", ss.name, requireNS, ss.namespace))
}

func (ssb *scanSettingBindingDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ssb.template, "-p", "NAME="+ssb.name, "NAMESPACE="+ssb.namespace,
		"PROFILENAME1="+ssb.profilename1, "PROFILEKIND1="+ssb.profilekind1, "PROFILENAME2="+ssb.profilename2, "SCANSETTINGNAME="+ssb.scansettingname)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "scansettingbinding", ssb.name, requireNS, ssb.namespace))
}

func (csuite *complianceSuiteDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(csuite.name, "compliancesuite", csuite.namespace)
}

func cleanupObjects(oc *exutil.CLI, objs ...objectTableRef) {
	for _, v := range objs {
		e2e.Logf("Start to remove: %v", v)
		_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func (cscan *complianceScanDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cscan.template, "-p", "NAME="+cscan.name,
		"NAMESPACE="+cscan.namespace, "SCANTYPE="+cscan.scanType, "PROFILE="+cscan.profile, "CONTENT="+cscan.content,
		"CONTENTIMAGE="+cscan.contentImage, "RULE="+cscan.rule, "KEY="+cscan.key, "VALUE="+cscan.value, "OPERATOR="+cscan.operator,
		"KEY1="+cscan.key1, "VALUE1="+cscan.value1, "OPERATOR1="+cscan.operator1, "NODESELECTOR="+cscan.nodeSelector,
		"PVACCESSMODE="+cscan.pvAccessModes, "STORAGECLASSNAME="+cscan.storageClassName, "SIZE="+cscan.size)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "compliancescan", cscan.name, requireNS, cscan.namespace))
}

func (cscan *complianceScanDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(cscan.name, "compliancescan", cscan.namespace)
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

func (tprofile *tailoredProfileWithoutVarDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", tprofile.template, "-p", "NAME="+tprofile.name, "NAMESPACE="+tprofile.namespace,
		"EXTENDS="+tprofile.extends, "ENRULENAME1="+tprofile.enrulename1, "ENRULENAME2="+tprofile.enrulename2, "DISRULENAME1="+tprofile.disrulename1,
		"DISRULENAME2="+tprofile.disrulename2)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "tailoredprofile", tprofile.name, requireNS, tprofile.namespace))
}

func (tprofile *tailoredProfileWithoutVarDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(tprofile.name, "tailoredprofile", tprofile.namespace)
}

func (sclass *storageClassDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sclass.template, "-p", "NAME="+sclass.name,
		"PROVISIONER="+sclass.provisioner, "RECLAIMPOLICY="+sclass.reclaimPolicy, "VOLUMEBINDINGMODE="+sclass.volumeBindingMode)
	o.Expect(err).NotTo(o.HaveOccurred())
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

func setLabelToNode(oc *exutil.CLI) {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=,node.openshift.io/os_id=rhcos",
		"-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	node := strings.Fields(nodeName)
	for _, v := range node {
		nodeLabel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "--show-labels").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(nodeLabel, "node-role.kubernetes.io/wscan=") {
			_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args("node", fmt.Sprintf("%s", v), "node-role.kubernetes.io/wscan=").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}
}

func getOneRhcosWorkerNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "--selector=node-role.kubernetes.io/worker=,node.openshift.io/os_id=rhcos",
		"-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of nodename:%v", nodeName)
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

func (subD *subscriptionDescription) complianceSuiteResult(oc *exutil.CLI, csuiteNmae string, expected string) {
	csuiteResult, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", subD.namespace, "compliancesuite", csuiteNmae, "-o=jsonpath={.status.result}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of csuiteResult:%v", csuiteResult)
	expectedStrings := strings.Fields(expected)
	lenExpectedStrings := len(strings.Fields(expected))
	switch {
	case lenExpectedStrings == 1, strings.Compare(expected, csuiteResult) == 0:
		e2e.Logf("Case 1: the expected string %v equals csuiteResult %v", expected, expectedStrings)
		return
	case lenExpectedStrings == 2, strings.Compare(expectedStrings[0], csuiteResult) == 0 || strings.Compare(expectedStrings[1], csuiteResult) == 0:
		e2e.Logf("Case 2: csuiteResult %v equals expected string %v or %v", csuiteResult, expectedStrings[0], expectedStrings[1])
		return
	default:
		e2e.Failf("Default: The expected string %v doesn't contain csuiteResult %v", expected, csuiteResult)
	}
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

func assertCoPodNumerEqualNodeNumber(oc *exutil.CLI, namespace string, label string) {

	intNodeNumber := getNodeNumberPerLabel(oc, label)
	podNameString, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace, "--selector=workload=scanner", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	intPodNumber := len(strings.Fields(podNameString))
	e2e.Logf("the result of intNodeNumber:%v", intNodeNumber)
	e2e.Logf("the result of intPodNumber:%v", intPodNumber)
	if intNodeNumber != intPodNumber {
		e2e.Failf("the intNodeNumber and intPodNumber not equal!")
	}
}

func getStorageClassProvisioner(oc *exutil.CLI) string {
	scname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(scname, "nfs") {
		scpro, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass", "nfs", "-o=jsonpath={.provisioner}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the result of StorageClassProvisioner:%v", scpro)
		return scpro
	} else {
		scs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass").OutputToFile(getRandomString() + "isc-config.json")
		e2e.Logf("the result of scs:%v", scs)
		result, err := exec.Command("bash", "-c", "cat "+scs+" | grep \"default\" | awk '{print $3}'; rm -rf "+scs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		res := strings.TrimSpace(string(result))
		e2e.Logf("the result of StorageClassProvisioner:%v", res)
		return res
	}
}

func getStorageClassVolumeBindingMode(oc *exutil.CLI) string {
	scname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(scname, "nfs") {
		scvbm, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass", "nfs", "-o=jsonpath={.volumeBindingMode}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the result of StorageClassVolumeBindingMode:%v", scvbm)
		return scvbm
	} else {
		sclassvbm, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass", "-o=jsonpath={.items[0].volumeBindingMode}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the result of StorageClassVolumeBindingMode:%v", sclassvbm)
		return sclassvbm

	}
}

func getResourceNameWithKeyword(oc *exutil.CLI, rs string, keyword string) string {
	var resourceName string
	rsList, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(rs, "-n", oc.Namespace(), "-o=jsonpath={.items[*].metadata.name}").Output()
	rsl := strings.Fields(rsList)
	for _, v := range rsl {
		resourceName = fmt.Sprintf("%s", v)
		if strings.Contains(resourceName, keyword) {
			break
		}
	}
	if resourceName == "" {
		e2e.Failf("Failed to get resource name!")
	}
	return resourceName
}

func getResourceNameWithKeywordFromResourceList(oc *exutil.CLI, rs string, keyword string) string {
	var result, resourceName string
	err := wait.Poll(1*time.Second, 120*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(rs, "-n", oc.Namespace(), "-o=jsonpath={.items[*].metadata.name}").Output()
		e2e.Logf("the result of output:%v", output)
		if strings.Contains(output, keyword) {
			result = output
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	rsl := strings.Fields(result)
	for _, v := range rsl {
		resourceName = fmt.Sprintf("%s", v)
		if strings.Contains(resourceName, keyword) {
			break
		}
	}
	if resourceName == "" {
		e2e.Failf("Failed to get resource name!")
	}
	return resourceName
}

func checkKeyWordsForRspod(oc *exutil.CLI, podname string, keyword [3]string) {
	var flag bool = true
	var kw string
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podname, "-n", oc.Namespace(), "-o=json").Output()

	o.Expect(err).NotTo(o.HaveOccurred())
	for _, v := range keyword {
		kw = fmt.Sprintf("%s", v)
		if !strings.Contains(output, kw) {
			e2e.Failf("The keyword %kw not exist!", v)
			flag = false
			break
		}
	}
	if flag == false {
		e2e.Failf("The keyword not exist!")
	}
}

func checkResourceNumber(oc *exutil.CLI, exceptedRsNo int, parameters ...string) {
	rs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(parameters...).OutputToFile(getRandomString() + "isc-rs.json")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of rs:%v", rs)
	result, err := exec.Command("bash", "-c", "cat "+rs+" | wc -l").Output()
	r1 := strings.TrimSpace(string(result))
	rsNumber, _ := strconv.Atoi(r1)
	if rsNumber < exceptedRsNo {
		e2e.Failf("The rsNumber %v not equals the exceptedRsNo %v!", rsNumber, exceptedRsNo)
	}
}

func checkWarnings(oc *exutil.CLI, expectedString string, parameters ...string) {
	rs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(parameters...).OutputToFile(getRandomString() + "isc-rs.json")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of rs:%v", rs)
	result, err := exec.Command("bash", "-c", "cat "+rs+" | awk '{print $1}'").Output()
	checkresults := strings.Fields(string(result))
	for _, checkresult := range checkresults {
		e2e.Logf("the result of checkresult:%v", checkresult)
		instructions, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("compliancecheckresult", checkresult, "-n", oc.Namespace(),
			"-o=jsonpath={.warnings}").Output()
		e2e.Logf("the result of instructions:%v", instructions)
		if !strings.Contains(instructions, expectedString) {
			e2e.Failf("The instructions %v don't contain expectedString %v!", instructions, expectedString)
			break
		}
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func checkFipsStatus(oc *exutil.CLI) string {
	mnodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "--selector=node.openshift.io/os_id=rhcos,node-role.kubernetes.io/master=",
		"-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	efips, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", oc.Namespace(), "node/"+mnodeName, "--", "chroot", "/host", "fips-mode-setup", "--check").Output()
	if strings.Contains(efips, "FIPS mode is disabled.") {
		e2e.Logf("Fips is disabled on master node %v ", mnodeName)
	} else {
		e2e.Logf("Fips is enabled on master node %v ", mnodeName)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return efips
}

func checkCisRulesInstruction(oc *exutil.CLI) {
	cisrule, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("compliancecheckresult", "-n", oc.Namespace(), "--selector=compliance.openshift.io/check-status=MANUAL",
		"-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	cisrules := strings.Fields(cisrule)
	for _, cisrule := range cisrules {
		ruleinst, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("compliancecheckresult", cisrule, "-n", oc.Namespace(), "-o=jsonpath={.instructions}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if ruleinst == "" {
			e2e.Failf("This CIS rule '%v' do not have any instruction", cisrule)
		} else {
			e2e.Logf("This CIS rule '%v' has instruction", cisrule)
		}
	}
}

func checkOauthPodsStatus(oc *exutil.CLI) {
	newCheck("expect", asAdmin, withoutNamespace, contain, "Pending", ok, []string{"pods", "-n", "openshift-authentication",
		"-o=jsonpath={.items[*].status.phase}"}).check(oc)
	newCheck("expect", asAdmin, withoutNamespace, contain, "3", ok, []string{"deployment", "oauth-openshift", "-n", "openshift-authentication",
		"-o=jsonpath={.status.readyReplicas}"}).check(oc)
	podnames, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication", "-o=jsonpath={.items[*].metadata.name}").Output()
	podname := strings.Fields(podnames)
	for _, v := range podname {
		newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", v, "-n", "openshift-authentication",
			"-o=jsonpath={.status.phase}"}).check(oc)
	}

}

func checkComplianceSuiteResult(oc *exutil.CLI, namespace string, csuiteNmae string, expected string) {
	csuiteResult, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", namespace, "compliancesuite", csuiteNmae, "-o=jsonpath={.status.result}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of csuiteResult:%v", csuiteResult)
	expectedStrings := strings.Fields(expected)
	lenExpectedStrings := len(strings.Fields(expected))
	switch {
	case lenExpectedStrings == 1, strings.Compare(expected, csuiteResult) == 0:
		e2e.Logf("Case 1: the expected string %v equals csuiteResult %v", expected, expectedStrings)
		return
	case lenExpectedStrings == 2, strings.Compare(expectedStrings[0], csuiteResult) == 0 || strings.Compare(expectedStrings[1], csuiteResult) == 0:
		e2e.Logf("Case 2: csuiteResult %v equals expected string %v or %v", csuiteResult, expectedStrings[0], expectedStrings[1])
		return
	default:
		e2e.Failf("Default: The expected string %v doesn't contain csuiteResult %v", expected, csuiteResult)
	}
}

func getResourceNameWithKeywordForNamespace(oc *exutil.CLI, rs string, keyword string, namespace string) string {
	var resourceName string
	rsList, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(rs, "-n", namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
	rsl := strings.Fields(rsList)
	for _, v := range rsl {
		resourceName = fmt.Sprintf("%s", v)
		e2e.Logf("the result of resourceName:%v", resourceName)
		if strings.Contains(resourceName, keyword) {
			break
		}
	}
	if resourceName == "" {
		e2e.Failf("Failed to get resource name!")
	}
	return resourceName
}

func checkOperatorPodStatus(oc *exutil.CLI, namespace string) string {
	var podname string
	podnames, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace).Output()
	podname = fmt.Sprintf("%s", podnames)
	if strings.Contains(podname, "cluster-logging-operator") {
		podStat, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace, "-o=jsonpath={.items[0].status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		return podStat
	} else {
		return podname
	}
}

func assertCheckAuditLogsForword(oc *exutil.CLI, namespace string, csvname string) {
	var auditlogs string
	podnames, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l logging-infra=fluentdserver", "-n", namespace, "-o=jsonpath={.items[0].metadata.name}").Output()
	auditlog, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", namespace, podnames, "cat", "/fluentd/log/audit.log").OutputToFile(getRandomString() + "isc-audit.json")
	o.Expect(err).NotTo(o.HaveOccurred())
	result, err1 := exec.Command("bash", "-c", "cat "+auditlog+" | grep "+csvname+" |tail -n5; rm -rf "+auditlog).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	auditlogs = fmt.Sprintf("%s", result)
	if strings.Contains(auditlogs, csvname) {
		e2e.Logf("The keyword does match with auditlogs: %v", csvname)
	} else {
		e2e.Failf("The keyword does not match with auditlogs: %v", csvname)
	}
}

func createLoginTemp(oc *exutil.CLI, namespace string) {
	e2e.Logf("Create a login.html template.. !!")
	_, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("create-login-template", "-n", namespace).OutputToFile(getRandomString() + "login.html")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Create a login-secret.. !!")
	_, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", "login-secret", "--from-file=login.html=./login.html", "-n", namespace).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
}
