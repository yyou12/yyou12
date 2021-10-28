package sro

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type ogResource struct {
	name      string
	namespace string
	template  string
}

func (og *ogResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("og", og.name, "-n", og.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No operatorgroup in project: %s, create one: %s", og.namespace, og.name))
		err = applyResource(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		e2e.Logf(fmt.Sprintf("Already exist operatorgroup in project: %s", og.namespace))
	}
}

func (og *ogResource) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", og.name, "-n", og.namespace).Output()
}

type subResource struct {
	name         string
	namespace    string
	channel      string
	installedCSV string
	template     string
	source       string
}

func (sub *subResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		err = applyResource(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "SUBNAME="+sub.name, "SUBNAMESPACE="+sub.namespace, "CHANNEL="+sub.channel, "SOURCE="+sub.source)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(5*time.Second, 240*time.Second, func() (bool, error) {
			state, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}").Output()
			if err != nil {
				e2e.Logf("output is %v, error is %v, and try next", state, getErr)
				return false, nil
			}
			if strings.Compare(state, "AtLatestKnown") == 0 || strings.Compare(state, "UpgradeAvailable") == 0 {
				return true, nil
			}
			e2e.Logf("sub %s state is %s, not AtLatestKnown or UpgradeAvailable", sub.name, state)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sub %s stat is not AtLatestKnown or UpgradeAvailable", sub.name))

		installedCSV, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(installedCSV).NotTo(o.BeEmpty())
		sub.installedCSV = installedCSV
	} else {
		e2e.Logf(fmt.Sprintf("Already exist sub in project: %s", sub.namespace))
	}
}

func (sub *subResource) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("sub", sub.name, "-n", sub.namespace).Output()
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("csv", sub.installedCSV, "-n", sub.namespace).Output()
}

type nsResource struct {
	name     string
	template string
}

func (ns *nsResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", ns.name).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("create one: %s", ns.name))
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", ns.template, "-p", "NAME="+ns.name).OutputToFile("sro-ns.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		e2e.Logf(fmt.Sprintf("Already exist ns: %s", ns.name))
	}
}

func (ns *nsResource) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", ns.name).Output()
}

func applyResource(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile("sro.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

type pkgmanifestinfo struct {
	pkgmanifestname string
	namespace       string
}

func (pkginfo pkgmanifestinfo) getDefaultChannelVersion(oc *exutil.CLI) (string, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", pkginfo.pkgmanifestname, "-n", pkginfo.namespace, "-o=jsonpath={.status.defaultChannel}").Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("The package manifest default channel not found in %s", pkginfo.namespace))
		return "stable", nil
	} else {
		return output, nil
	}
}

func (pkginfo pkgmanifestinfo) getPKGManifestSource(oc *exutil.CLI) (string, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", pkginfo.pkgmanifestname, "-n", pkginfo.namespace, "-o=jsonpath={.status.catalogSource}").Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("The package manifest source not found in %s", pkginfo.namespace))
		return "qe-app-registry", nil
	} else {
		return output, nil
	}
}

type oprResource struct {
	kind      string
	name      string
	namespace string
}

//When will return false if Operator is not installed, and true otherwise
func (opr *oprResource) checkOperatorPOD(oc *exutil.CLI) bool {
	e2e.Logf("Checking if " + opr.name + " pod is succesfully running...")
	podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(opr.kind, "-n", opr.namespace, "--no-headers").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	var (
		activepod int
		totalpod  int
	)
	if strings.Contains(podList, "NotFound") || strings.Contains(podList, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No pod is running in project: %s", opr.namespace))
		return false
	} else {
		runningpod := strings.Count(podList, "Running")
		runningjob := strings.Count(podList, "Completed")
		activepod = runningjob + runningpod
		totalpod = strings.Count(podList, "\n") + 1
	}

	if reflect.DeepEqual(activepod, totalpod) {
		e2e.Logf(fmt.Sprintf("The active pod is :%d\nThe total pod is:%d", activepod, totalpod))
		e2e.Logf(opr.name + " pod is suceessfully running :(")
		return true
	} else {
		e2e.Logf(opr.name + " pod abnormal, please check!")
		return false
	}
}

func (opr *oprResource) applyResourceByYaml(oc *exutil.CLI, yamlfile string) {
	if len(opr.namespace) == 0 {
		//Create cluster-wide resource
		oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", yamlfile).Execute()
	} else {
		//Create namespace-wide resource
		oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", yamlfile, "-n", opr.namespace).Execute()
	}
}

func (opr *oprResource) CleanupResource(oc *exutil.CLI) {
	if len(opr.namespace) == 0 {
		//Delete cluster-wide resource
		oc.AsAdmin().WithoutNamespace().Run("delete").Args(opr.kind, opr.name).Execute()
	} else {
		//Delete namespace-wide resource
		oc.AsAdmin().WithoutNamespace().Run("delete").Args(opr.kind, opr.name, "-n", opr.namespace).Execute()
	}

}

func getClusterNodesBy(oc *exutil.CLI, role string) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

//Check if NFD Installed base on the cluster labels
func checkIfNFDInstalled(oc *exutil.CLI) bool {
	workNode, _ := exutil.GetFirstWorkerNode(oc)
	Output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", workNode, "-o", "jsonpath='{.metadata.annotations}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(Output, "nfd.node.kubernetes.io/feature-labels") {
		e2e.Logf("NFD installed on openshift container platform")
		return true
	}
	return false
}

func (opr *oprResource) waitOprResourceReady(oc *exutil.CLI) {
	waitErr := wait.Poll(20*time.Second, 600*time.Second, func() (bool, error) {
		var (
			output     string
			err        error
			isCreated  bool
			desirednum string
			readynum   string
		)

		//Check if deployment/daemonset is created.
		switch opr.kind {
		case "deployment":
			output, err = oc.AsAdmin().Run("get").Args(opr.kind, opr.name, "-n", opr.namespace, "-o=jsonpath={.status.conditions}").Output()
			if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
				isCreated = false
				output = "Failure"
			} else {
				//deployment has been created, but not running, need to check the string "has successfully progressed" in .status.conditions
				isCreated = true
			}
		case "daemonset":
			daemonsetname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(opr.kind, "-n", opr.namespace, "-oname").Output()
			e2e.Logf("daemonset name is:" + daemonsetname)
			if len(daemonsetname) == 0 || err != nil {
				isCreated = false
			} else {
				//daemonset has been created, but not running, need to compare .status.desiredNumberScheduled and .status.numberReady}
				//if the two value is equal, set output="has successfully progressed"
				isCreated = true
				desirednum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(daemonsetname, "-n", opr.namespace, "-o=jsonpath={.status.desiredNumberScheduled}").Output()
				readynum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(daemonsetname, "-n", opr.namespace, "-o=jsonpath={.status.numberReady}").Output()
			}

			e2e.Logf("desirednum is:" + desirednum + "\nreadynum is:" + readynum)
			//daemonset has been created, but not running, need to compare .status.desiredNumberScheduled and .status.numberReady}
			//if the two value is equal, set output="has successfully progressed"
			if len(daemonsetname) != 0 && desirednum == readynum {
				e2e.Logf("Damonset running normally")
				output = "has successfully progressed"
			} else {
				e2e.Logf("Damonset is abnormally running")
				output = "Failure"
				return false, nil
			}
		default:
			e2e.Logf("Invalid Resource Type")
		}

		e2e.Logf("the %v status is %v", opr.kind, output)
		//if isCreated is true and output contains "has successfully progressed", the pod is ready
		if isCreated && strings.Contains(output, "has successfully progressed") {
			return true, nil
		} else {
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("the pod of %v is not running", opr.name))
}

func assertSimpleKmodeOnNode(oc *exutil.CLI) {
	nodeList, err := getClusterNodesBy(oc, "worker")
	nodeListSize := len(nodeList)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodeListSize).NotTo(o.Equal(0))

	regexpstr, _ := regexp.Compile(`simple.*`)
	waitErr := wait.Poll(15*time.Second, time.Second*300, func() (done bool, err error) {
		e2e.Logf("Check simple-kmod moddule on first worker node")
		firstWokerNode, _ := exutil.GetFirstWorkerNode(oc)
		output, _ := exutil.DebugNodeWithChroot(oc, firstWokerNode, "lsmod")
		match, _ := regexp.MatchString(`simple.*`, output)
		if match {
			//Check all worker nodes and generated full report
			for i := 0; i < nodeListSize; i++ {
				output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "lsmod")
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.ContainSubstring("simple"))
				e2e.Logf("Verify if simple kmod installed on %v", nodeList[i])
				simpleKmod := regexpstr.FindAllString(output, 2)
				e2e.Logf("The result is: %v", simpleKmod)
			}
			return true, nil
		} else {
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(waitErr, "the simple-kmod not found")
}
