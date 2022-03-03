//Kata operator tests
package kata

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	snooze time.Duration = 720
)

// author: abhbaner@redhat.com
func createIfNoOperator(oc *exutil.CLI, opNamespace, ns, og, sub string) (status bool) {
	operatorInstall(oc, opNamespace, ns, og, sub)
	return true
}

// author: abhbaner@redhat.com
func createIfNoKataConfig(oc *exutil.CLI, opNamespace, kc, kcName string) (status bool) {
	kataConfigInstall(oc, opNamespace, kc, kcName)
	return true

}

// author: abhbaner@redhat.com
func operatorInstall(oc *exutil.CLI, opNamespace, ns, og, sub string) (status bool) {
	//Installing Operator
	g.By(" (1) INSTALLING sandboxed-operator in 'openshift-sandboxed-containers-operator' namespace")

	//Applying the config of necessary yaml files to create sandbox operator
	g.By("(1.1) Applying namespace yaml")
	msg, err := oc.AsAdmin().Run("apply").Args("-f", ns).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	

	g.By("(1.2)  Applying operatorgroup yaml")
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", og, "-n", opNamespace).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	

	g.By("(1.3) Applying subscription yaml")
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", sub, "-n", opNamespace).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	

	//confirming operator install
	errCheck := wait.Poll(10*time.Second, snooze*time.Second, func() (bool, error) {
		subState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "sandboxed-containers-operator", "-n", opNamespace, "-o=jsonpath={.status.state}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(subState, "AtLatestKnown") == 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("sub sandboxed-containers-operator is not correct status in ns %v", opNamespace))

	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "sandboxed-containers-operator", "-n", opNamespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(csvName).NotTo(o.BeEmpty())
	errCheck = wait.Poll(10*time.Second, snooze*time.Second, func() (bool, error) {
		csvState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", opNamespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(csvState, "Succeeded") == 0 {
			return true, nil
			e2e.Logf("CSV check complete!!!")
		}
		return false, nil

	})
	exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("csv %v is not correct status in ns %v", csvName, opNamespace))

	return true
}

// author: abhbaner@redhat.com
func kataConfigInstall(oc *exutil.CLI, opNamespace, kc, kcName string) (status bool) {
	g.By("Applying kataconfig")
	configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", kc, "-p", "NAME="+kcName).OutputToFile(getRandomString() + "kataconfig-common.json")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the file of resource is %s", configFile)

	oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
	
	g.By("Check if kataconfig is applied")
	errCheck := wait.Poll(10*time.Second, snooze*time.Second, func() (bool, error) {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("kataconfig", kcName, "-o=jsonpath={.status.installationStatus.IsInProgress}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(msg, "false") {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("kataconfig %v is not correct status in ns %v", kcName, opNamespace))
	return true
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

