package workloads

import (
     "encoding/json"
     "strconv"
     "strings"
     "regexp"
     "time"

     o "github.com/onsi/gomega"
     exutil "github.com/openshift/openshift-tests-private/test/extended/util"
     "k8s.io/apimachinery/pkg/util/wait"
     e2e "k8s.io/kubernetes/test/e2e/framework"
)

type operatorgroup struct {
      name             string
      namespace        string
      template         string
}

type subscription struct {
      name             string
      namespace        string
      channelName      string
      opsrcName        string
      sourceName       string
      template         string
}

type kubedescheduler struct {
      namespace        string
      interSeconds     int
      imageInfo        string
      logLevel         string
      operatorLogLevel string
      profile1         string
      profile2         string
      profile3         string
      template         string
}

type deploynodeaffinity struct {
	dName          string
	namespace      string
	replicaNum     int
	labelKey       string
	labelValue     string
	affinityKey    string
	operatorPolicy string
	affinityValue1 string
	affinityValue2 string
	template       string
}

type deploynodetaint struct {
        dName          string
        namespace      string
        template       string
}

type deployinterpodantiaffinity struct {
        dName            string
        namespace        string
        replicaNum       int
        podAffinityKey   string
        operatorPolicy   string
        podAffinityValue string
        template         string
}

type deployduplicatepods struct {
        dName            string
        namespace        string
        replicaNum       int
        template         string
}

type deploypodtopologyspread struct {
        dName            string
        namespace        string
        template         string
}

func (sub *subscription) createSubscription(oc *exutil.CLI) {
      err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "NAME="+sub.name, "NAMESPACE="+sub.namespace,
		"CHANNELNAME="+sub.channelName, "OPSRCNAME="+sub.opsrcName, "SOURCENAME="+sub.sourceName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
      })
      o.Expect(err).NotTo(o.HaveOccurred())
}

func (sub *subscription) deleteSubscription(oc *exutil.CLI) {
      err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := oc.AsAdmin().WithoutNamespace().Run("delete").Args("subscription", sub.name, "-n", sub.namespace).Execute()
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
      })
      o.Expect(err).NotTo(o.HaveOccurred())
}

func (og *operatorgroup) createOperatorGroup(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (og *operatorgroup) deleteOperatorGroup(oc *exutil.CLI) {
        err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := oc.AsAdmin().WithoutNamespace().Run("delete").Args("operatorgroup", og.name, "-n", og.namespace).Execute()
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
        })
        o.Expect(err).NotTo(o.HaveOccurred())
}

func (dsch *kubedescheduler) createKubeDescheduler(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", dsch.template, "-p", "NAMESPACE="+dsch.namespace, "INTERSECONDS="+strconv.Itoa(dsch.interSeconds),
			"IMAGEINFO="+dsch.imageInfo, "LOGLEVEL="+dsch.logLevel, "OPERATORLOGLEVEL="+dsch.operatorLogLevel,
			"PROFILE1="+dsch.profile1, "PROFILE2="+dsch.profile2, "PROFILE3="+dsch.profile3)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkEvents(oc *exutil.CLI, projectname string, strategyname string, expected string) {
	err := wait.Poll(5*time.Second, 100*time.Second, func() (bool, error) {
		output, err := oc.WithoutNamespace().Run("get").Args("events", "-n", projectname).Output()
		if err != nil {
			e2e.Logf("Can't get events from test project, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString("pod evicted by.*NodeAffinity", output); matched {
			e2e.Logf("Check the %s Strategy succeed:\n", strategyname)
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkAvailable(oc *exutil.CLI, rsKind string, rsName string, namespace string, expected string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(rsKind, rsName, "-n", namespace, "-o=jsonpath={.status.availableReplicas}").Output()
		if err != nil {
			e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString(expected, output); matched {
			e2e.Logf("deploy is up:\n%s", output)
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getImageFromCSV(oc *exutil.CLI, namespace string ) string {
	var csvalm interface{}
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", namespace, "-o=jsonpath={.items[0].metadata.annotations.alm-examples}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out = strings.TrimLeft(out, "[")
		out = strings.TrimRight(out, "]")
		if err := json.Unmarshal([]byte(out), &csvalm); err != nil {
			e2e.Logf("unable to decode version with error: %v", err)
		}
		amlOject := csvalm.(map[string]interface{})
		imageInfo := amlOject["spec"].(map[string]interface{})["image"].(string)
		return imageInfo
}

func waitForAvailableRsRunning(oc *exutil.CLI, rsKind string, rsName string, namespace string, expected string) bool {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(rsKind, rsName, "-n", namespace, "-o=jsonpath={.status.availableReplicas}").Output()
		if err != nil {
			e2e.Logf("object is still inprogress, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString(expected, output); matched {
			e2e.Logf("object is up:\n%s", output)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false
	}
	return true
}

func checkPodsStatusByLabel(oc *exutil.CLI, namespace string, labels string, expectedstatus string) bool {
	out, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l",  labels,   "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podsList := strings.Fields(out)
	for _, pod := range podsList {
		podstatus, err := oc.WithoutNamespace().Run("get").Args("pod", pod, "-n", namespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString(expectedstatus, podstatus); !matched {
			e2e.Logf("%s is not with status:\n%s", pod, expectedstatus)
			return false
		}
	}
	return true
}

func createResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.WithoutNamespace().Run("process").Args(parameters...).OutputToFile(getRandomString() + "workload-config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("the file of resource is %s", configFile)
	return oc.WithoutNamespace().Run("create").Args("-f", configFile).Execute()
}

func checkLogsFromRs(oc *exutil.CLI, projectname string, rsKind string, rsName string, expected string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(rsKind+`/`+rsName, "-n", projectname).Output()
		if err != nil {
			e2e.Logf("Can't get logs from test project, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.Match(expected, []byte(output)); !matched {
			e2e.Logf("Can't find the expected string\n")
			return false, nil
		}else {
			e2e.Logf("Check the logs succeed!!\n")
			return true, nil
		}
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (deploy *deploynodeaffinity) createDeployNodeAffinity(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deploy.template, "-p", "DNAME="+deploy.dName, "NAMESPACE="+deploy.namespace,
			"REPLICASNUM="+strconv.Itoa(deploy.replicaNum), "LABELKEY="+deploy.labelKey, "LABELVALUE="+deploy.labelValue, "AFFINITYKEY="+deploy.affinityKey,
			"OPERATORPOLICY="+deploy.operatorPolicy, "AFFINITYVALUE1="+deploy.affinityValue1, "AFFINITYVALUE2="+deploy.affinityValue2)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (deployn *deploynodetaint) createDeployNodeTaint(oc *exutil.CLI) {
        err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deployn.template, "-p", "DNAME="+deployn.dName, "NAMESPACE="+deployn.namespace)
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
        })
        o.Expect(err).NotTo(o.HaveOccurred())
}

func (deployp *deployinterpodantiaffinity) createDeployInterPodAntiAffinity(oc *exutil.CLI) {
        err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deployp.template, "-p", "DNAME="+deployp.dName, "NAMESPACE="+deployp.namespace,
                        "REPLICASNUM="+strconv.Itoa(deployp.replicaNum), "PODAFFINITYKEY="+deployp.podAffinityKey,
                        "OPERATORPOLICY="+deployp.operatorPolicy, "PODAFFINITYVALUE="+deployp.podAffinityValue)
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
        })
        o.Expect(err).NotTo(o.HaveOccurred())
}

func (deploydp *deployduplicatepods) createDuplicatePods(oc *exutil.CLI) {
    err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deploydp.template, "-p", "DNAME="+deploydp.dName, "NAMESPACE="+deploydp.namespace,
                        "REPLICASNUM="+strconv.Itoa(deploydp.replicaNum))
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
        })
        o.Expect(err).NotTo(o.HaveOccurred())
}

func (deploypts *deploypodtopologyspread) createPodTopologySpread(oc *exutil.CLI) {
    err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
                err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deploypts.template, "-p", "DNAME="+deploypts.dName, "NAMESPACE="+deploypts.namespace)
                if err1 != nil {
                        e2e.Logf("the err:%v, and try next round", err1)
                        return false, nil
                }
                return true, nil
        })
        o.Expect(err).NotTo(o.HaveOccurred())
}
