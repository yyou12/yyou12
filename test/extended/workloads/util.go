package workloads

import (
	o "github.com/onsi/gomega"

	"math/rand"
	"strconv"
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type podNodeSelector struct {
	name       string
	namespace  string
	labelKey   string
	labelValue string
	nodeKey    string
	nodeValue  string
	template   string
}

type podSinglePts struct {
	name       string
	namespace  string
	labelKey   string
	labelValue string
	ptsKeyName string
	ptsPolicy  string
	skewNum    int
	template   string
}

type podSinglePtsNodeSelector struct {
	name       string
	namespace  string
	labelKey   string
	labelValue string
	ptsKeyName string
	ptsPolicy  string
	skewNum    int
	nodeKey    string
	nodeValue  string
	template   string
}

type deploySinglePts struct {
	dName      string
	namespace  string
	replicaNum int
	labelKey   string
	labelValue string
	ptsKeyName string
	ptsPolicy  string
	skewNum    int
	template   string
}

type deployNodeSelector struct {
	dName      string
	namespace  string
	replicaNum int
	labelKey   string
	labelValue string
	nodeKey    string
	nodeValue  string
	template   string
}

type podAffinityRequiredPts struct {
	name           string
	namespace      string
	labelKey       string
	labelValue     string
	ptsKeyName     string
	ptsPolicy      string
	skewNum        int
	affinityMethod string
	keyName        string
	valueName      string
	operatorName   string
	template       string
}

type podAffinityPreferredPts struct {
	name           string
	namespace      string
	labelKey       string
	labelValue     string
	ptsKeyName     string
	ptsPolicy      string
	skewNum        int
	affinityMethod string
	weigthNum      int
	keyName        string
	valueName      string
	operatorName   string
	template       string
}

type podNodeAffinityRequiredPts struct {
	name           string
	namespace      string
	labelKey       string
	labelValue     string
	ptsKeyName     string
	ptsPolicy      string
	skewNum        int
	ptsKey2Name    string
	ptsPolicy2     string
	skewNum2       int
	affinityMethod string
	keyName        string
	valueName      string
	operatorName   string
	template       string
}

type podSingleNodeAffinityRequiredPts struct {
        name           string
        namespace      string
        labelKey       string
        labelValue     string
        ptsKeyName     string
        ptsPolicy      string
        skewNum        int
        affinityMethod string
        keyName        string
        valueName      string
        operatorName   string
        template       string
}

type podTolerate struct {
        namespace      string
        keyName        string
        operatorPolicy string
        valueName      string
        effectPolicy   string
        tolerateTime   int
        template       string
}

func (pod *podNodeSelector) createPodNodeSelector(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"NODEKEY="+pod.nodeKey, "NODEVALUE="+pod.nodeValue, "LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podSinglePts) createPodSinglePts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum))
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podSinglePtsNodeSelector) createPodSinglePtsNodeSelector(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum),
			"NODEKEY="+pod.nodeKey, "NODEVALUE="+pod.nodeValue)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (deploy *deploySinglePts) createDeploySinglePts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deploy.template, "-p", "DNAME="+deploy.dName, "NAMESPACE="+deploy.namespace,
			"REPLICASNUM="+strconv.Itoa(deploy.replicaNum), "LABELKEY="+deploy.labelKey, "LABELVALUE="+deploy.labelValue, "PTSKEYNAME="+deploy.ptsKeyName,
			"PTSPOLICY="+deploy.ptsPolicy, "SKEWNUM="+strconv.Itoa(deploy.skewNum))
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podAffinityRequiredPts) createPodAffinityRequiredPts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum),
			"AFFINITYMETHOD="+pod.affinityMethod, "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podAffinityPreferredPts) createPodAffinityPreferredPts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum),
			"AFFINITYMETHOD="+pod.affinityMethod, "WEIGHTNUM="+strconv.Itoa(pod.weigthNum), "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podSinglePts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podNodeSelector) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podSinglePtsNodeSelector) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podAffinityRequiredPts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podAffinityPreferredPts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.Run("process").Args(parameters...).OutputToFile(getRandomString() + "workload-config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("the file of resource is %s", configFile)
	return oc.WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func describePod(oc *exutil.CLI, namespace string, podName string) string {
	podDescribe, err := oc.WithoutNamespace().Run("describe").Args("pod", "-n", namespace, podName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status is %q", podName, podDescribe)
	return podDescribe
}

func getPodStatus(oc *exutil.CLI, namespace string, podName string) string {
	podStatus, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status is %q", podName, podStatus)
	return podStatus
}

func getPodNodeListByLabel(oc *exutil.CLI, namespace string, labelKey string) []string {
	output, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", labelKey, "-o=jsonpath={.items[*].spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeNameList := strings.Fields(output)
	return nodeNameList
}
func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func (pod *podNodeAffinityRequiredPts) createpodNodeAffinityRequiredPts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum), "PTSKEY2NAME="+pod.ptsKey2Name, "PTSPOLICY2="+pod.ptsPolicy2, "SKEWNUM2="+strconv.Itoa(pod.skewNum2), "AFFINITYMETHOD="+pod.affinityMethod, "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podNodeAffinityRequiredPts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podSingleNodeAffinityRequiredPts) createpodSingleNodeAffinityRequiredPts(oc *exutil.CLI) {
        err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum), "AFFINITYMETHOD="+pod.affinityMethod, "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
        })
        o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podSingleNodeAffinityRequiredPts) getPodNodeName(oc *exutil.CLI) string {
        nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
        return nodeName
}

func (pod *podTolerate) createPodTolerate(oc *exutil.CLI) {
        err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAMESPACE="+pod.namespace,"KEYNAME="+pod.keyName,
                "OPERATORPOLICY="+pod.operatorPolicy, "VALUENAME="+pod.valueName, "EFFECTPOLICY="+pod.effectPolicy, "TOLERATETIME="+strconv.Itoa(pod.tolerateTime))
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
        })
        o.Expect(err).NotTo(o.HaveOccurred())
}

func getPodNodeName(oc *exutil.CLI, namespace string, podName string) string {
        nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.spec.nodeName}").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        e2e.Logf("The pod %s lands on node %q", podName, nodeName)
        return nodeName
}
