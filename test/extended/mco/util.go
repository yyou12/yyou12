package mco

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
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

type PodDisruptionBudget struct {
	name      string
	namespace string
	template  string
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
	mc.name = mc.name + "-" + exutil.GetRandomString()
	params := []string{"--ignore-unknown-parameters=true", "-f", mc.template, "-p", "NAME=" + mc.name, "POOL=" + mc.pool}
	params = append(params, mc.parameters...)
	exutil.CreateClusterResourceFromTemplate(oc, params...)

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
	exutil.CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", kc.template, "-p", "NAME="+kc.name)
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

func (pdb *PodDisruptionBudget) create(oc *exutil.CLI) {
	e2e.Logf("Creating pod disruption budget: %s", pdb.name)
	exutil.CreateNsResourceFromTemplate(oc, pdb.namespace, "--ignore-unknown-parameters=true", "-f", pdb.template, "-p", "NAME="+pdb.name)
}

func (pdb *PodDisruptionBudget) delete(oc *exutil.CLI) {
	e2e.Logf("Deleting pod disruption budget: %s", pdb.name)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("pdb", pdb.name, "-n", pdb.namespace, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (icsp *ImageContentSourcePolicy) create(oc *exutil.CLI) {
	exutil.CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", icsp.template, "-p", "NAME="+icsp.name)
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
	exutil.CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cr.template, "-p", "NAME="+cr.name)
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
	exutil.CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", mcp.template, "-p", "NAME="+mcp.name)
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

func getTimeDifferenceInMinute(oldTimestamp string, newTimestamp string) float64 {
	oldTimeValues := strings.Split(oldTimestamp, ":")
	oldTimeHour, _ := strconv.Atoi(oldTimeValues[0])
	oldTimeMinute, _ := strconv.Atoi(oldTimeValues[1])
	oldTimeSecond, _ := strconv.Atoi(strings.Split(oldTimeValues[2], ".")[0])
	oldTimeNanoSecond, _ := strconv.Atoi(strings.Split(oldTimeValues[2], ".")[1])
	newTimeValues := strings.Split(newTimestamp, ":")
	newTimeHour, _ := strconv.Atoi(newTimeValues[0])
	newTimeMinute, _ := strconv.Atoi(newTimeValues[1])
	newTimeSecond, _ := strconv.Atoi(strings.Split(newTimeValues[2], ".")[0])
	newTimeNanoSecond, _ := strconv.Atoi(strings.Split(newTimeValues[2], ".")[1])
	y, m, d := time.Now().Date()
	oldTime := time.Date(y, m, d, oldTimeHour, oldTimeMinute, oldTimeSecond, oldTimeNanoSecond, time.UTC)
	newTime := time.Date(y, m, d, newTimeHour, newTimeMinute, newTimeSecond, newTimeNanoSecond, time.UTC)
	return newTime.Sub(oldTime).Minutes()
}

func filterTimestampFromLogs(logs string, numberOfTimestamp int) []string {
	return regexp.MustCompile("(?m)[0-9]{1,2}:[0-9]{1,2}:[0-9]{1,2}.[0-9]{1,6}").FindAllString(logs, numberOfTimestamp)
}

// WaitForNumberOfLinesInPodLogs wait and return the pod logs by the specific filter and number of lines
func waitForNumberOfLinesInPodLogs(oc *exutil.CLI, namespace string, container string, podName string, filter string, numberOfLines int) string {
	var logs string
	var err error
	waitErr := wait.Poll(30*time.Second, 20*time.Minute, func() (bool, error) {
		logs, err = exutil.WaitAndGetSpecificPodLogs(oc, namespace, container, podName, filter)
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Count(logs, strings.Trim(filter, "'")) >= numberOfLines {
			e2e.Logf("Filtered pod logs: %v", logs)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("Number of lines in the logs is less than %v", numberOfLines))
	return logs
}

func getMachineConfigDetails(oc *exutil.CLI, mcName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", mcName, "-o", "yaml").Output()
}

func getKubeletConfigDetails(oc *exutil.CLI, kcName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeletconfig", kcName, "-o", "yaml").Output()
}

func getCommitId(oc *exutil.CLI, component string, clusterVersion string) (string, error) {
	outFilePath, ocErr := oc.AsAdmin().WithoutNamespace().Run("adm").Args("release", "info", "--commits", clusterVersion).OutputToFile("commitIdLogs.txt")
	if ocErr != nil {
		return "", ocErr
	}
	commitId, cmdErr := exec.Command("bash", "-c", "cat "+outFilePath+" | grep "+component+" | awk '{print $3}'").Output()
	return strings.TrimSuffix(string(commitId), "\n"), cmdErr
}

func getGoVersion(component string, commitId string) (float64, error) {
	curlOutput, curlErr := exec.Command("bash", "-c", "curl -Lks https://raw.githubusercontent.com/openshift/"+component+"/"+commitId+"/go.mod | egrep '^go'").Output()
	if curlErr != nil {
		return 0, curlErr
	}
	goVersion := string(curlOutput)[3:]
	return strconv.ParseFloat(strings.TrimSuffix(goVersion, "\n"), 64)
}

func getMachineConfigDaemon(oc *exutil.CLI, node string) string {
	machineConfigDaemon, err := exutil.GetPodName(oc, "openshift-machine-config-operator", "k8s-app=machine-config-daemon", node)
	o.Expect(err).NotTo(o.HaveOccurred())
	return machineConfigDaemon
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

func generateTemplateAbsolutePath(fileName string) string {
	mcoBaseDir := exutil.FixturePath("testdata", "mco")
	return filepath.Join(mcoBaseDir, fileName)
}

func getServiceClusterIP(oc *exutil.CLI, svcName string, svcNamespace string) string {
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("svc", svcName, "-n", svcNamespace, "-o", "jsonpath='{.spec.clusterIP}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	return stdout
}

func getServicePort(oc *exutil.CLI, svcName string, svcNamespace string, portName string) string {
	jsonPathArg := fmt.Sprintf("jsonpath='{.spec.ports[?(@.name==\"%s\")].port}'", portName)
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("svc", svcName, "-n", svcNamespace, "-o", jsonPathArg).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	return stdout
}

func getSATokenFromContainer(oc *exutil.CLI, podName string, podNamespace string, container string) string {
	podOut, err := exutil.RemoteShContainer(oc, podNamespace, podName, container, "cat", "/var/run/secrets/kubernetes.io/serviceaccount/token")
	o.Expect(err).NotTo(o.HaveOccurred())

	return podOut
}

func getHostFromRoute(oc *exutil.CLI, routeName string, routeNamespace string) string {
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", routeName, "-n", routeNamespace, "-o", "jsonpath='{.spec.host}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	return stdout
}

func getPrometheusQueryResults(oc *exutil.CLI, query string) string {

	token := getSATokenFromContainer(oc, "prometheus-k8s-0", "openshift-monitoring", "prometheus")

	routeHost := getHostFromRoute(oc, "prometheus-k8s", "openshift-monitoring")
	url := fmt.Sprintf("https://%s/api/v1/query?query=%s", routeHost, query)
	headers := fmt.Sprintf("Authorization: Bearer %s", token)

	curlCmd := fmt.Sprintf("curl -ks -H '%s' %s", headers, url)
	e2e.Logf("curl cmd:\n %s", curlCmd)

	curlOutput, cmdErr := exec.Command("bash", "-c", curlCmd).Output()
	e2e.Logf("curl output:\n%s", curlOutput)
	o.Expect(cmdErr).NotTo(o.HaveOccurred())

	return string(curlOutput)
}
