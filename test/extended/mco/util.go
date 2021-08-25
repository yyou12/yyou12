package mco

import (
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type machineConfig struct {
	name     string
	template string
	pool     string
}

type machineConfigPool struct {
	name string
}

func (mc *machineConfig) create(oc *exutil.CLI) {
	mc.name = mc.name + "-" + getRandomString()
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", mc.template, "-p", "NAME="+mc.name, "POOL="+mc.pool)
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

	mcp := machineConfigPool{name: mc.pool}
	mcp.waitForComplete(oc)
}

func (mc *machineConfig) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("mc", mc.name).Execute()
	mcp := machineConfigPool{name: mc.pool}
	mcp.waitForComplete(oc)
}

func (mcp *machineConfigPool) waitForComplete(oc *exutil.CLI) {
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

func getFirstWorkerNode(oc *exutil.CLI) (string, error) {
	return getClusterNodeBy(oc, "worker", "0")
}

func getFirstMasterNode(oc *exutil.CLI) (string, error) {
	return getClusterNodeBy(oc, "master", "0")
}

func getClusterNodeBy(oc *exutil.CLI, role string, index string) (string, error) {
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items["+index+"].metadata.name}'").Output()
	return strings.Trim(stdout, "'"), err
}

func debugNodeWithChroot(oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, true, cmd...)
}

func debugNode(oc *exutil.CLI, nodeName string, needChroot bool, cmd ...string) (string, error) {
	var cargs []string
	if needChroot {
		cargs = []string{"node/" + nodeName, "--", "chroot", "/host"}
	} else {
		cargs = []string{"node/" + nodeName, "--"}
	}
	cargs = append(cargs, cmd...)
	return oc.AsAdmin().Run("debug").Args(cargs...).Output()
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
