package networking

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

//struct for sriovnetworknodepolicy and sriovnetwork
type sriovNetResource struct {
	name      string
	namespace string
	tempfile  string
	kind      string
}

//struct for sriov pod
type sriovPod struct {
	name         string
	tempfile     string
	namespace    string
	ipv4addr     string
	ipv6addr     string
	intfname     string
	intfresource string
}

//delete sriov resource
func (rs *sriovNetResource) delete(oc *exutil.CLI) {
	e2e.Logf("delete %s %s in namespace %s", rs.kind, rs.name, rs.namespace)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args(rs.kind, rs.name, "-n", rs.namespace).Execute()
}

//create sriov resource
func (rs *sriovNetResource) create(oc *exutil.CLI, parameters ...string) {
	var configFile string
	cmd := []string{"-f", rs.tempfile, "--ignore-unknown-parameters=true", "-p"}
	for _, para := range parameters {
		cmd = append(cmd, para)
	}
	e2e.Logf("parameters list is %s\n", cmd)
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(cmd...).OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process sriov resource %v", cmd))
	e2e.Logf("the file of resource is %s\n", configFile)

	_, err1 := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile, "-n", rs.namespace).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
}

//porcess sriov pod template and get a configuration file
func (pod *sriovPod) processPodTemplate(oc *exutil.CLI) string {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args("-f", pod.tempfile, "--ignore-unknown-parameters=true", "-p", "PODNAME="+pod.name, "SRIOVNETNAME="+pod.intfresource,
			"IPV4_ADDR="+pod.ipv4addr, "IPV6_ADDR="+pod.ipv6addr, "-o=jsonpath={.items[0]}").OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process pod resource %v", pod.name))
	e2e.Logf("the file of resource is %s\n", configFile)
	return configFile
}

//create pod
func (pod *sriovPod) createPod(oc *exutil.CLI) string {
	configFile := pod.processPodTemplate(oc)
	podsLog, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("--loglevel=10", "-f", configFile, "-n", pod.namespace).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	return podsLog
}

//delete pod
func (pod *sriovPod) deletePod(oc *exutil.CLI) {
	e2e.Logf("delete pod %s in namespace %s", pod.name, pod.namespace)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod.name, "-n", pod.namespace).Execute()
}

// check pods of openshift-sriov-network-operator are running
func chkSriovOperatorStatus(oc *exutil.CLI, ns string) {
	e2e.Logf("check if openshift-sriov-network-operator pods are running properly")
	chkPodsStatus(oc, ns, "app=network-resources-injector")
	chkPodsStatus(oc, ns, "app=operator-webhook")
	chkPodsStatus(oc, ns, "app=sriov-network-config-daemon")
	chkPodsStatus(oc, ns, "name=sriov-network-operator")

}

// check specified pods are running
func chkPodsStatus(oc *exutil.CLI, ns, lable string) {
	podsStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", ns, "-l", lable, "-o=jsonpath={.items[*].status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podsStatus = strings.TrimSpace(podsStatus)
	statusList := strings.Split(podsStatus, " ")
	for _, podStat := range statusList {
		o.Expect(podStat).Should(o.MatchRegexp("Running"))
	}
	e2e.Logf("All pods with lable %s in namespace %s are Running", lable, ns)
}

//clear specified sriovnetworknodepolicy
func rmSriovNetworkPolicy(oc *exutil.CLI, kind, policyname, pf, ns string) {
	sriovPolicyList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(sriovPolicyList, policyname) {
		_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(kind, policyname, "-n", ns).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForSriovPolicyReady(oc, ns)
	}
	res, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args(kind, "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	// should no sriovnetworknodepolicy has used this pf
	o.Expect(res).ShouldNot(o.MatchRegexp(pf))

}

//clear specified sriovnetwork
func rmSriovNetwork(oc *exutil.CLI, kind, netname, ns string) {
	sriovNetList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(sriovNetList, netname) {
		_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(kind, netname, "-n", ns).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// Wait for Pod ready
func (pod *sriovPod) waitForPodReady(oc *exutil.CLI) {
	res := false
	err := wait.Poll(5*time.Second, 15*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", pod.name, "-n", pod.namespace, "-o=jsonpath={.status.phase}").Output()
		e2e.Logf("the status of pod is %v", status)
		if strings.Contains(status, "NotFound") {
			e2e.Logf("the pod was created fail.")
			res = false
			return true, nil
		}
		if err != nil {
			e2e.Logf("failed to get pod status: %v, retrying...", err)
			return false, nil
		}
		if strings.Contains(status, "Running") {
			e2e.Logf("the pod is Ready.")
			res = true
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sriov pod %v is not ready", pod.name))
	o.Expect(res).To(o.Equal(true))
}

// Wait for sriov network policy ready
func waitForSriovPolicyReady(oc *exutil.CLI, ns string) bool {
	res := false
	err := wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sriovnetworknodestates", "-n", ns, "-o=jsonpath={.items[*].status.syncStatus}").Output()
		e2e.Logf("the status of sriov policy is %v", status)
		if err != nil {
			e2e.Logf("failed to get sriov policy status: %v, retrying...", err)
			return false, nil
		}
		nodesStatus := strings.TrimSpace(status)
		statusList := strings.Split(nodesStatus, " ")
		for _, nodeStat := range statusList {
			if nodeStat != "Succeeded" {
				return false, nil
			}
		}
		res = true
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "sriovnetworknodestates is not ready")
	return res
}

//check interface on pod
func (pod *sriovPod) getSriovIntfonPod(oc *exutil.CLI) string {
	msg, err := oc.WithoutNamespace().AsAdmin().Run("exec").Args(pod.name, "-n", pod.namespace, "-i", "--", "ip", "address").Output()
	if err != nil {
		e2e.Logf("Execute ip address command failed with  err:%v .", err)
	}
	e2e.Logf("Get ip address info as:%v", msg)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).NotTo(o.BeEmpty())
	return msg
}

//create pod via HTTP request
func (pod *sriovPod) sendHTTPRequest(oc *exutil.CLI, user, cmd string) {
	//generate token for service acount
	testToken, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", user, "-n", pod.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(testToken).NotTo(o.BeEmpty())

	configFile := pod.processPodTemplate(oc)

	curlCmd := cmd + " -k " + " -H " + fmt.Sprintf("\"Authorization: Bearer %v\"", testToken) + " -d " + "@" + configFile

	e2e.Logf("Send curl request to create new pod: %s\n", curlCmd)

	res, err := exec.Command("bash", "-c", curlCmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(res).NotTo(o.BeEmpty())

}
