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
			"-o=jsonpath={.status.applicationDatabase.phase}", "Running", oc)
		CheckCR(currentPackage, mongodbOpsManagerCR, mongodbOpsManagerClusterName,
			"-o=jsonpath={.status.opsManager.phase}", "Running", oc)
		RemoveCR(currentPackage, mongodbOpsManagerCR, mongodbOpsManagerClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It(TestCaseName("portworx-certified", intermediateTestsSufix), func() {

		packageName := "portworx-certified"
		crdName := "storagenode"
		crName := "storagenode-example"
		crFile := "portworx-snode-cr.yaml"
		namespace := "portworx-certified"
		jsonPath := "-o=json"
		expectedMsg := "storagenode-example"

		defer RemoveNamespace(namespace, oc)
		g.By("install operator")
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		g.By("check deployment of operator")
		CheckDeployment(currentPackage, oc)
		g.By("create CR")
		CreateFromYAML(currentPackage, crFile, oc)
		g.By("check CR")
		CheckCR(currentPackage, crdName, crName, jsonPath, expectedMsg, oc)
		g.By("remvoe operator")
		RemoveCR(currentPackage, crdName, crName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It(TestCaseName("couchbase-enterprise-certified", intermediateTestsSufix), func() {

		packageName := "couchbase-enterprise-certified"
		crdName := "CouchbaseCluster"
		crName := "cb-example"
		crFile := "couchbase-enterprise-cr.yaml"
		namespace := "couchbase-enterprise-certified"
		jsonPath := "-o=json"
		expectedMsg := "Running"

		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, crFile, oc)
		CheckCR(currentPackage, crdName, crName, jsonPath, expectedMsg, oc)
		RemoveCR(currentPackage, crdName, crName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It(TestCaseName("jaeger-product", intermediateTestsSufix), func() {

		jaegerPackageName := "jaeger-product"
		jaegerCR := "Jaeger"
		jaegerCRClusterName := "jaeger-all-in-one-inmemory"
		namespace := "openshift-operators"

		currentPackage := CreateSubscriptionSpecificNamespace(jaegerPackageName, oc, false, false, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, "jaeger.yaml", oc)
		CheckCR(currentPackage, jaegerCR, jaegerCRClusterName,
			"-o=jsonpath={.status.phase}", "Running", oc)
		RemoveCR(currentPackage, jaegerCR, jaegerCRClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It(TestCaseName("keycloak-operator", intermediateTestsSufix), func() {

		keycloakCR := "Keycloak"
		keycloakCRName := "example-keycloak"
		keycloakPackageName := "keycloak-operator"
		keycloakFile := "keycloak-cr.yaml"
		namespace := "keycloak"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(keycloakPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, keycloakFile, oc)
		CheckCR(currentPackage, keycloakCR, keycloakCRName, "-o=jsonpath={.status.ready}", "true", oc)
		RemoveCR(currentPackage, keycloakCR, keycloakCRName, oc)
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
		e2e.Logf(msg)
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
