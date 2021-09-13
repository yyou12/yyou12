package mco

import (
	"encoding/json"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type MachineConfig struct {
	name       string
	template   string
	pool       string
	parameters []string
}

type MachineConfigPool struct {
	name     string
	template string
}

type KubeletConfig struct {
	name     string
	template string
}

type ContainerRuntimeConfig struct {
	name     string
	template string
}

type TextToVerify struct {
	textToVerifyForMC   string
	textToVerifyForNode string
	needBash            bool
	needChroot          bool
}

func (mc *MachineConfig) create(oc *exutil.CLI) {
	mc.name = mc.name + "-" + getRandomString()
	params := []string{"--ignore-unknown-parameters=true", "-f", mc.template, "-p", "NAME=" + mc.name, "POOL=" + mc.pool}
	params = append(params, mc.parameters...)
	err := applyResourceFromTemplate(oc, params...)
	o.Expect(err).NotTo(o.HaveOccurred())

	wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc/"+mc.name, "-o", "jsonpath='{.metadata.name}'").Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, mc.name) {
			e2e.Logf("mc %s is created successfully", mc.name)
			return true, nil
		}
		return false, nil
	})

	mcp := MachineConfigPool{name: mc.pool}
	mcp.waitForComplete(oc)
}

func (mc *MachineConfig) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("mc", mc.name).Execute()
	mcp := MachineConfigPool{name: mc.pool}
	mcp.waitForComplete(oc)
}

func (kc *KubeletConfig) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", kc.template, "-p", "NAME="+kc.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp := MachineConfigPool{name: "worker"}
	mcp.waitForComplete(oc)
}

func (kc *KubeletConfig) delete(oc *exutil.CLI) {
	e2e.Logf("deleting kubelet config: %s", kc.name)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", kc.name).Execute()
	mcp := MachineConfigPool{name: "worker"}
	mcp.waitForComplete(oc)
}

func (cr *ContainerRuntimeConfig) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cr.template, "-p", "NAME="+cr.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp := MachineConfigPool{name: "worker"}
	mcp.waitForComplete(oc)
}

func (cr *ContainerRuntimeConfig) delete(oc *exutil.CLI) {
	e2e.Logf("deleting container runtime config: %s", cr.name)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("ctrcfg", cr.name).Execute()
	mcp := MachineConfigPool{name: "worker"}
	mcp.waitForComplete(oc)
}

func (mcp *MachineConfigPool) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", mcp.template, "-p", "NAME="+mcp.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp.waitForComplete(oc)
}

func (mcp *MachineConfigPool) delete(oc *exutil.CLI) {
	e2e.Logf("deleting custom mcp: %s", mcp.name)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("mcp", mcp.name).Execute()
}

func (mcp *MachineConfigPool) waitForComplete(oc *exutil.CLI) {
	err := wait.Poll(1*time.Minute, 25*time.Minute, func() (bool, error) {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcp.name, "-o", "jsonpath='{.status.conditions[?(@.type==\"Updated\")].status}'").Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, "True") {
			// i.e. mcp updated=true, mc is applied successfully
			e2e.Logf("mc operation is completed on mcp %s", mcp.name)
			return true, nil
		}
		return false, nil
	})

	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForNodeDoesNotContain(oc *exutil.CLI, node string, value string) {
	err := wait.Poll(1*time.Minute, 10*time.Minute, func() (bool, error) {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("node/" + node).Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if !strings.Contains(stdout, value) {
			e2e.Logf("node does not contain %s", value)
			return true, nil
		}
		return false, nil
	})

	o.Expect(err).NotTo(o.HaveOccurred())
}

func getMachineConfigDetails(oc *exutil.CLI, mcName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", mcName, "-o", "yaml").Output()
}

func getKubeletConfigDetails(oc *exutil.CLI, kcName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeletconfig", kcName, "-o", "yaml").Output()
}

func getMachineConfigDaemon(oc *exutil.CLI, node string) string {
	args := []string{"pods", "-n", "openshift-machine-config-operator", "-l", "k8s-app=machine-config-daemon",
		"--field-selector", "spec.nodeName=" + node, "-o", "jsonpath='{..metadata.name}'"}
	daemonPod, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.ReplaceAll(daemonPod, "'", "")
}

func getContainerRuntimeConfigDetails(oc *exutil.CLI, crName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("ctrcfg", crName, "-o", "yaml").Output()
}

func getStatusCondition(oc *exutil.CLI, resource string, ctype string) (map[string]interface{}, error) {
	jsonstr, ocerr := oc.AsAdmin().WithoutNamespace().Run("get").Args(resource, "-o", "jsonpath='{.status.conditions[?(@.type==\""+ctype+"\")]}'").Output()
	if ocerr != nil {
		return nil, ocerr
	}
	e2e.Logf("condition info of %v-%v : %v", resource, ctype, jsonstr)
	jsonstr = strings.Trim(jsonstr, "'")
	jsonbytes := []byte(jsonstr)
	var datamap map[string]interface{}
	if jsonerr := json.Unmarshal(jsonbytes, &datamap); jsonerr != nil {
		return nil, jsonerr
	} else {
		e2e.Logf("umarshalled json: %v", datamap)
		return datamap, jsonerr
	}
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
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

func generateTemplateAbsolutePath(fileName string) string {
	mcoBaseDir := exutil.FixturePath("testdata", "mco")
	return filepath.Join(mcoBaseDir, fileName)
}
