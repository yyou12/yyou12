package operators

import (
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const DEFAULT_STATUS_QUERY = "-o=jsonpath={.status.conditions[0].type}"
const DEFAULT_EXPECTED_BEHAVIOR = "Ready"

var _ = g.Describe("[Suite:openshift/isv] ISV_Operators", func() {
	var (
		oc                     = exutil.NewCLI("operators", exutil.KubeConfigPath())
		intermediateTestsSufix = "[Intermediate]"
	)

	g.It(TestCaseName("amq-streams", intermediateTestsSufix), func() {

		kafkaCR := "Kafka"
		kafkaClusterName := "my-cluster"
		kafkaPackageName := "amq-streams"
		kafkaFile := "kafka.yaml"
		namespace := "amq-streams"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(kafkaPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, kafkaFile, oc)
		CheckCR(currentPackage, kafkaCR, kafkaClusterName, DEFAULT_STATUS_QUERY, DEFAULT_EXPECTED_BEHAVIOR, oc)
		RemoveCR(currentPackage, kafkaCR, kafkaClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It(TestCaseName("mongodb-enterprise", intermediateTestsSufix), func() {

		mongodbPackageName := "mongodb-enterprise"
		mongodbOpsManagerCR := "opsmanagers"
		mongodbOpsManagerClusterName := "ops-manager"
		namespace := "mongodb"

		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(mongodbPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, "mongodb-ops-manager-secret.yaml", oc)
		CreateFromYAML(currentPackage, "mongodb-ops-manager-cr.yaml", oc)
		CheckCR(currentPackage, mongodbOpsManagerCR, mongodbOpsManagerClusterName,
			"{.status.applicationDatabase.phase}", "Running", oc)
		CheckCR(currentPackage, mongodbOpsManagerCR, mongodbOpsManagerClusterName,
			"{.status.opsManager.phase}", "Running", oc)
		RemoveCR(currentPackage, mongodbOpsManagerCR, mongodbOpsManagerClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

})

func CreateFromYAML(p Packagemanifest, filename string, oc *exutil.CLI) {
	buildPruningBaseDir := exutil.FixturePath("testdata", "operators")
	cr := filepath.Join(buildPruningBaseDir, filename)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", cr, "-n", p.Namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}
func RemoveCR(p Packagemanifest, CRName string, instanceName string, oc *exutil.CLI) {
	msg, err := oc.WithoutNamespace().AsAdmin().Run("delete").Args(CRName, instanceName, "-n", p.Namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).To(o.ContainSubstring("deleted"))
}

func CheckCR(p Packagemanifest, CRName string, instanceName string, jsonPath string, expectedMessage string, oc *exutil.CLI) {

	poolErr := wait.Poll(10*time.Second, 600*time.Second, func() (bool, error) {
		msg, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args(CRName, instanceName, "-n", p.Namespace, jsonPath).Output()
		if strings.Contains(msg, expectedMessage) {
			return true, nil
		}
		return false, nil
	})
	if poolErr != nil {
		e2e.Logf("Could not get CR " + CRName + " for " + p.CsvVersion)
		RemoveCR(p, CRName, instanceName, oc)
		RemoveOperatorDependencies(p, oc, false)
		g.Fail("Could not get CR " + CRName + " for " + p.CsvVersion)
	}
}
