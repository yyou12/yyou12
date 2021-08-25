package container_engine_tools

import (
	"math/rand"
	"strings"
	"time"
	"fmt"
	"os/exec"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type podModifyDescription struct {
	name               string
	namespace          string
	mountpath          string
	command            string
	args               string
	restartPolicy      string
	user		   string
	role               string
	level              string
	template           string
}

type ctrcfgDescription struct {
	namespace          string
	pidlimit	   int
	loglevel           string
	overlay            string
	logsizemax         string
	command            string
	configFile         string
	template           string
}

type objectTableRefcscope struct {
	kind              string
	name              string
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

func (podModify *podModifyDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podModify.template, "-p", "NAME="+podModify.name, "NAMESPACE="+podModify.namespace, "MOUNTPATH="+podModify.mountpath, "COMMAND="+podModify.command, "ARGS="+podModify.args, "POLICY="+podModify.restartPolicy, "USER="+podModify.user, "ROLE="+podModify.role, "LEVEL="+podModify.level)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podModify *podModifyDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podModify.namespace, "pod", podModify.name).Execute()
}

func createResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "node-config.json")
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("The resource is %s", jsonCfg)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", jsonCfg).Execute()
}

func podStatusReason(oc *exutil.CLI ) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.initContainerStatuses[*].state.waiting.reason}", "-n", oc.Namespace()).Output()
		e2e.Logf("the status of pod is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "CrashLoopBackOff") {
			e2e.Logf(" Pod failed status reason is :%s", status)
			return true, nil
		}
		return false, nil
	})
}

func podStatusterminatedReason(oc *exutil.CLI ) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.initContainerStatuses[*].state.terminated.reason}", "-n", oc.Namespace()).Output()
		e2e.Logf("the status of pod is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "Error") {
			e2e.Logf(" Pod failed status reason is :%s", status)
			return true, nil
		}
		return false, nil
	})
}

func podStatus(oc *exutil.CLI ) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.phase}", "-n", oc.Namespace()).Output()
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "Running") {
			e2e.Logf("Pod status is : %s", status)
			return true, nil
		}
		return false, nil
	})
}

func volStatus(oc *exutil.CLI ) error {
	e2e.Logf("check content of volume")
	return wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("init-volume", "-c", "hello-pod", "cat", "/init-test/volume-test", "-n", oc.Namespace()).Output()
		e2e.Logf("The content of the vol is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "This is OCP volume test") {
			e2e.Logf(" Init containers with volume work fine \n")
			return true, nil
		}
		return false,nil
	})
}

func ContainerSccStatus(oc *exutil.CLI ) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "hello-pod", "-o=jsonpath={.spec.securityContext.seLinuxOptions.*}", "-n", oc.Namespace()).Output()
		e2e.Logf("The Container SCC Content is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "unconfined_u unconfined_r s0:c25,c968") {
			e2e.Logf("SeLinuxOptions in pod applied to container Sucessfully \n")
			return true, nil
		}
		return false,nil
	})
}

func (ctrcfg *ctrcfgDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ctrcfg.template, "-p", "LOGLEVEL="+ctrcfg.loglevel, "OVERLAY="+ctrcfg.overlay, "LOGSIZEMAX="+ctrcfg.logsizemax )
	o.Expect(err).NotTo(o.HaveOccurred())
}

func cleanupObjectsClusterScope(oc *exutil.CLI, objs ...objectTableRefcscope) {
	for _, v := range objs {
		e2e.Logf("\n Start to remove: %v", v)
		_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, v.name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func (ctrcfg *ctrcfgDescription) checkCtrcfgParameters(oc *exutil.CLI) error {
	return wait.Poll(10*time.Minute, 11*time.Minute, func() (bool, error) {
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		for _, v := range node {
		nodeStatus,err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "-o=jsonpath={.status.conditions[3].type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode %s Status is %s\n", v,nodeStatus)

			if nodeStatus == "Ready"{
			criostatus,err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(`node/`+fmt.Sprintf("%s", v), "--", "chroot", "/host", "crio", "config").OutputToFile("crio.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(`\nCRI-O PARAMETER ON THE WORKER NODE :`+fmt.Sprintf("%s", v))
			e2e.Logf("\ncrio config file path is  %v", criostatus)

				wait.Poll(2*time.Second, 1*time.Minute, func() (bool,error) {
					result, err1 := exec.Command("bash", "-c", "cat "+criostatus+" | egrep 'pids_limit|log_level'").Output()
						if err != nil {
							e2e.Failf("the result of ReadFile:%v", err1)
							return false, nil
						}
						e2e.Logf("\nCtrcfg Parameters is %s", result)
						if strings.Contains(string(result), "debug") && strings.Contains(string(result), "2048"){
							e2e.Logf("\nCtrcfg parameter pod limit and log_level configured successfully")
							return true, nil
						}
						return false, nil
					})
			} else {
				e2e.Logf("\n NODES ARE NOT READY\n ")
			}
		}
		return true, nil
	})
}