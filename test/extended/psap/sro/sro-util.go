package sro

import (
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// isNFDInstalled will return false if NFD is not installed, and true otherwise
func isNFDInstalled(oc *exutil.CLI, machineNFDNamespace string) bool {
	e2e.Logf("Checking if NFD operator is installed ...")
	podList, err := oc.AdminKubeClient().CoreV1().Pods(machineNFDNamespace).List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(podList.Items) == 0 {
		e2e.Logf("NFD operator not found :(")
		return false
	}
	e2e.Logf("NFD operator found!")
	return true
}

type ogResource struct {
	name      string
	namespace string
	template  string
}

func (og *ogResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("og", og.name, "-n", og.namespace).Output()
	if strings.Contains(output, "NotFound") || err != nil {
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
	if strings.Contains(output, "NotFound") || err != nil {
		err = applyResource(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "SUBNAME="+sub.name, "SUBNAMESPACE="+sub.namespace, "CHANNEL="+sub.channel, "SOURCE="+sub.source)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
			state, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}").Output()
			if err != nil {
				e2e.Logf("output is %v, error is %v, and try next", state, getErr)
				return false, nil
			}
			if strings.Compare(state, "AtLatestKnown") == 0 {
				return true, nil
			}
			e2e.Logf("sub %s state is %s, not AtLatestKnown", sub.name, state)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sub %s stat is not AtLatestKnown", sub.name))

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
	if strings.Contains(output, "NotFound") || err != nil {
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
	if strings.Contains(output, "NotFound") || err != nil {
		e2e.Logf(fmt.Sprintf("The package manifest default channel not found in %s", pkginfo.namespace))
		return "stable", nil
	} else {
		return output, nil
	}
}

func (pkginfo pkgmanifestinfo) getPKGManifestSource(oc *exutil.CLI) (string, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", pkginfo.pkgmanifestname, "-n", pkginfo.namespace, "-o=jsonpath={.status.catalogSource}").Output()
	if strings.Contains(output, "NotFound") || err != nil {
		e2e.Logf(fmt.Sprintf("The package manifest source not found in %s", pkginfo.namespace))
		return "qe-app-registry", nil
	} else {
		return output, nil
	}
}
