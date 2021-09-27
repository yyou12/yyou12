package mco

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type MachineConfig struct {
	name           string
	template       string
	pool           string
	parameters     []string
	skipWaitForMcp bool
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

type ImageContentSourcePolicy struct {
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

	pollerr := wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
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
	exutil.AssertWaitPollNoErr(pollerr, fmt.Sprintf("create machine config %v failed", mc.name))

	if !mc.skipWaitForMcp {
		mcp := MachineConfigPool{name: mc.pool}
		mcp.waitForComplete(oc)
	}

}

func (mc *MachineConfig) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("mc", mc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
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
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", kc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp := MachineConfigPool{name: "worker"}
	mcp.waitForComplete(oc)
}

func (icsp *ImageContentSourcePolicy) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", icsp.template, "-p", "NAME="+icsp.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp := MachineConfigPool{name: "worker"}
	mcp.waitForComplete(oc)
	mcp.name = "master"
	mcp.waitForComplete(oc)
}

func (icsp *ImageContentSourcePolicy) delete(oc *exutil.CLI) {
	e2e.Logf("deleting icsp config: %s", icsp.name)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("imagecontentsourcepolicy", icsp.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp := MachineConfigPool{name: "worker"}
	mcp.waitForComplete(oc)
	mcp.name = "master"
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
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("ctrcfg", cr.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
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
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("mcp", mcp.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (mcp *MachineConfigPool) pause(oc *exutil.CLI, enable bool) {
	e2e.Logf("patch mcp %v, change spec.paused to %v", mcp.name, enable)
	err := oc.AsAdmin().Run("patch").Args("mcp", mcp.name, "--type=merge", "-p", `{"spec":{"paused": `+strconv.FormatBool(enable)+`}}`).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (mcp *MachineConfigPool) getConfigNameOfSpec(oc *exutil.CLI) (string, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcp.name, "-o", "jsonpath='{.spec.configuration.name}'").Output()
	e2e.Logf("spec.configuration.name of mcp/%v is %v", mcp.name, output)
	return output, err
}

func (mcp *MachineConfigPool) getConfigNameOfStatus(oc *exutil.CLI) (string, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcp.name, "-o", "jsonpath='{.status.configuration.name}'").Output()
	e2e.Logf("status.configuration.name of mcp/%v is %v", mcp.name, output)
	return output, err
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

	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("mc operation is not completed on mcp %s", mcp.name))
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

	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("node contains %s", value))
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
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
}

func containsMultipleStrings(sourceString string, expectedStrings []string) bool {
	o.Expect(sourceString).NotTo(o.BeEmpty())
	o.Expect(expectedStrings).NotTo(o.BeEmpty())

	var count int
	for _, element := range expectedStrings {
		if strings.Contains(sourceString, element) {
			count++
		}
	}
	return len(expectedStrings) == count
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
