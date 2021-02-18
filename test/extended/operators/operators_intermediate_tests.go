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

var _ = g.Describe("[sig-operators] ISV_Operators [Suite:openshift/isv]", func() {
	var (
		oc                     = exutil.NewCLI("operators", exutil.KubeConfigPath())
		intermediateTestsSufix = "[Intermediate]"
	)

	g.It(TestCaseName("amq-streams", "Medium-"+CaseIDCertifiedOperators["amq-streams"]+"-"+intermediateTestsSufix), func() {

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

	g.It(TestCaseName("mongodb-enterprise", "Medium-"+CaseIDCertifiedOperators["mongodb-enterprise"]+"-"+intermediateTestsSufix), func() {

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

	g.It(TestCaseName("portworx-certified", "Medium-"+CaseIDCertifiedOperators["portworx-certified"]+"-"+intermediateTestsSufix), func() {

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

	g.It(TestCaseName("couchbase-enterprise-certified", "Medium-"+CaseIDCertifiedOperators["couchbase-enterprise-certified"]+"-"+intermediateTestsSufix), func() {

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

	g.It(TestCaseName("jaeger-product", "Medium-"+CaseIDCertifiedOperators["jaeger-product"]+"-"+intermediateTestsSufix), func() {

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

	g.It(TestCaseName("keycloak-operator", "Medium-"+CaseIDCertifiedOperators["keycloak-operator"]+"-"+intermediateTestsSufix), func() {

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

	g.It(TestCaseName("spark-gcp", "Medium-"+CaseIDCertifiedOperators["spark-gcp"]+"-"+intermediateTestsSufix), func() {
		packageName := "spark-gcp" // spark-operator in OperatorHub
		namespace := "spark-gcp"
		crFile := "spark-gcp-sparkapplication-cr.yaml"
		sparkgcpCR := "sparkapp"
		sparkgcpName := "spark-pi"
		crPodname := "spark-pi-driver"
		jsonPath := "-o=jsonpath={.status.applicationState.state}"
		expectedMsg := "COMPLETE"
		searchMsg := "Pi is roughly "
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, crFile, oc)
		CheckCR(currentPackage, sparkgcpCR, sparkgcpName, jsonPath, expectedMsg, oc)
		msg, err := oc.WithoutNamespace().AsAdmin().Run("logs").Args(crPodname, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring(searchMsg))
		e2e.Logf("STEP PASS %v", searchMsg)
		RemoveCR(currentPackage, sparkgcpCR, sparkgcpName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)
	})

	g.It(TestCaseName("strimzi-kafka-operator", "Medium-"+CaseIDCertifiedOperators["strimzi-kafka-operator"]+"-"+intermediateTestsSufix), func() {

		strimziCR := "Kafka"
		strimziClusterName := "my-cluster"
		strimziPackageName := "strimzi-kafka-operator"
		strimziFile := "strimzi-cr.yaml"
		namespace := "strimzi"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(strimziPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, strimziFile, oc)
		CheckCR(currentPackage, strimziCR, strimziClusterName, DEFAULT_STATUS_QUERY, DEFAULT_EXPECTED_BEHAVIOR, oc)
		RemoveCR(currentPackage, strimziCR, strimziClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It(TestCaseName("resource-locker-operator", "Medium-"+CaseIDCertifiedOperators["resource-locker-operator"]+"-"+intermediateTestsSufix), func() {

		packageName := "resource-locker-operator"
		crdName := "ResourceLocker"
		crName := "locked-configmap-foo-bar-configmap"
		crFile := "resourcelocker-cr.yaml"
		jsonPath := "-o=jsonpath={.status.conditions..reason}"
		expectedMsg := "LastReconcileCycleSucceded"
		rolesFile := "resourcelocker-role.yaml"
		sa := "resource-locker-test-sa"

		g.By("install operator")
		currentPackage := CreateSubscription(packageName, oc, INSTALLPLAN_AUTOMATIC_MODE)
		defer RemoveOperatorDependencies(currentPackage, oc, false)

		defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("sa", sa, "-n", currentPackage.Namespace).Output()

		_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", sa, "-n", currentPackage.Namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check deployment of operator")
		CheckDeployment(currentPackage, oc)

		g.By("create CR")
		defer RemoveFromYAML(currentPackage, rolesFile, oc)
		CreateFromYAML(currentPackage, rolesFile, oc)
		CreateFromYAML(currentPackage, crFile, oc)

		g.By("check CR")
		CheckCR(currentPackage, crdName, crName, jsonPath, expectedMsg, oc)

		g.By("remove CR")
		RemoveCR(currentPackage, crdName, crName, oc)

	})

	g.It(TestCaseName("storageos2", "Medium-"+CaseIDCertifiedOperators["storageos2"]+"-"+intermediateTestsSufix), func() {

		packageName := "storageos2"
		crdName1 := "StorageOSCluster"
		crdName2 := "storageosupgrade"
		crName1 := "storageoscluster-example"
		crName2 := "storageosupgrade-example"
		crFile1 := "storageoscluster-cr.yaml"
		crFile2 := "storageosupgrade-cr.yaml"
		secretFile := "storageos-secret.yaml"
		namespace := "storageos"
		jsonPath := "-o=json"
		expectedMsg1 := "storageoscluster-example"
		expectedMsg2 := "storageosupgrade-example"

		defer RemoveNamespace(namespace, oc)
		g.By("create secret")
		buildPruningBaseDirsecret := exutil.FixturePath("testdata", "operators")
		secret := filepath.Join(buildPruningBaseDirsecret, secretFile)
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", secret).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", "storageos-api-isv", "-n", "openshift-operators").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("install operator")
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		g.By("check deployment of operator")
		CheckDeployment(currentPackage, oc)
		g.By("create CR1")
		CreateFromYAML(currentPackage, crFile1, oc)
		g.By("create CR2")
		CreateFromYAML(currentPackage, crFile2, oc)
		g.By("check CR1")
		CheckCR(currentPackage, crdName1, crName1, jsonPath, expectedMsg1, oc)
		g.By("check CR2")
		CheckCR(currentPackage, crdName2, crName2, jsonPath, expectedMsg2, oc)
		g.By("remvoe operator")
		RemoveCR(currentPackage, crdName1, crName1, oc)
		RemoveCR(currentPackage, crdName2, crName2, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("Medium-27312-[Intermediate] Operator argocd-operator should work properly", func() {
		argoCR := "ArgoCD"
		argoCRName := "example-argocd"
		argoPackageName := "argocd-operator"
		argoFile := "argocd-cr.yaml"
		namespace := "argocd"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(argoPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, argoFile, oc)
		CheckCR(currentPackage, argoCR, argoCRName, "-o=jsonpath={.status.phase}", "Available", oc)
		RemoveCR(currentPackage, argoCR, argoCRName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("Medium-27301-[Intermediate] Operator kiali-ossm should work properly", func() {
		kialiCR := "Kiali"
		kialiCRName := "kiali-27301"
		kialiPackageName := "kiali-ossm"
		kialiFile := "kiali-cr.yaml"
		namespace := "openshift-operators"
		kialiNamespace := "istio-system"
		CreateNamespaceWithoutPrefix(kialiNamespace, oc)
		defer RemoveNamespace(kialiNamespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(kialiPackageName, oc, false, false, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, kialiFile, oc)
		CheckCR(currentPackage, kialiCR, kialiCRName, "-o=jsonpath={.status.conditions..reason}", "Running", oc)
		RemoveCR(currentPackage, kialiCR, kialiCRName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})
})

//the method is to create CR with yaml file in the namespace of the installed operator
func CreateFromYAML(p Packagemanifest, filename string, oc *exutil.CLI) {
	buildPruningBaseDir := exutil.FixturePath("testdata", "operators")
	cr := filepath.Join(buildPruningBaseDir, filename)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", cr, "-n", p.Namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to create CR with yaml file in the namespace of the installed operator
func RemoveFromYAML(p Packagemanifest, filename string, oc *exutil.CLI) {
	buildPruningBaseDir := exutil.FixturePath("testdata", "operators")
	cr := filepath.Join(buildPruningBaseDir, filename)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", cr, "-n", p.Namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to delete CR of kind CRName with name instanceName in the namespace of the installed operator
func RemoveCR(p Packagemanifest, CRName string, instanceName string, oc *exutil.CLI) {
	msg, err := oc.WithoutNamespace().AsAdmin().Run("delete").Args(CRName, instanceName, "-n", p.Namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).To(o.ContainSubstring("deleted"))
}

//the method is to check if the CR is expected.
//the content is got by jsonpath.
//if it is expected, nothing happen
//if it is not expected, it will delete CR and the resource of the installed operator, for example sub, csv and possible ns
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
