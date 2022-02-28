//Kata operator tests
package kata

import (
	"fmt"
	"strings"
	"path/filepath"
	"time"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
        e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("[sig-kata] Kata", func() {
	defer g.GinkgoRecover()
     

	var (
		
		opNamespace = "openshift-sandboxed-containers-operator"
		oc = exutil.NewCLI("kata", exutil.KubeConfigPath())
		ns           string 
		og           string 
		sub          string
		testDataDir  string
		iaasPlatform string
		
	)
    
	g.BeforeEach(func() {

        output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		iaasPlatform = strings.ToLower(output)
		testDataDir = exutil.FixturePath("testdata", "kata")
		ns = filepath.Join(testDataDir, "namespace.yaml")
		og = filepath.Join(testDataDir, "operatorgroup.yaml")
		sub = filepath.Join(testDataDir, "subscription.yaml")
		
		
         
		//Installing Operator
        g.By(" (1) INSTALLING sandboxed-operator in 'openshift-sandboxed-containers-operator' namespace")
		
		//Applying the config of necessary yaml files to create sandbox operator
        g.By("(1.1) Applying namespace yaml")
		msg,err := oc.AsAdmin().Run("apply").Args("-f", ns).Output()
		e2e.Logf("err %v, msg %v", err, msg)
		o.Expect(err).NotTo(o.HaveOccurred())
		
		g.By("(1.2)  Applying operatorgroup yaml")
		msg,err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", og,"-n", opNamespace).Output()
		e2e.Logf("err %v, msg %v", err, msg)
		o.Expect(err).NotTo(o.HaveOccurred())
		
		g.By("(1.3) Applying subscription yaml")
		msg,err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", sub,"-n", opNamespace).Output()
		e2e.Logf("err %v, msg %v", err, msg)
		o.Expect(err).NotTo(o.HaveOccurred())
		
		//check of operator
		errCheck := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			subState,err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "sandboxed-containers-operator", "-n", opNamespace, "-o=jsonpath={.status.state}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Compare(subState, "AtLatestKnown") == 0 {
			return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("sub sandboxed-containers-operator is not correct status in ns %v", opNamespace))
	
		csvName, err:= oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "sandboxed-containers-operator", "-n", opNamespace, "-o=jsonpath={.status.installedCSV}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(csvName).NotTo(o.BeEmpty())
		errCheck = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			csvState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", opNamespace, "-o=jsonpath={.status.phase}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Compare(csvState, "Succeeded") == 0 {
				return true, nil
				e2e.Logf("CSV check complete!!!")
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("csv %v is not correct status in ns %v", csvName, opNamespace))

 
	})
	// author: abhbaner@redhat.com
	    g.It("Author:abhbaner-High-39499-Operator installation", func(){
		// May fail other cases in parallel, so run it in serial
			g.By("Checking sandboxed-operator operator installation")
            e2e.Logf("Operator install check successfull as part of setup !!!!!")
			g.By("SUCCESSS - sandboxed-operator operator installed")
		 
				 
		
		})
	
})
