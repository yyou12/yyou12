package operators

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"

	"github.com/google/go-github/github"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	opm "github.com/openshift/openshift-tests-private/test/extended/opm"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	db "github.com/openshift/openshift-tests-private/test/extended/util/db"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: jiazha@redhat.com
	// add `Serial` label since this etcd-operator are subscribed for cluster-scoped,
	// that means may leads to other etcd-opertor subscription fail if in Parallel
	g.It("ConnectedOnly-Author:jiazha-High-37826-use an PullSecret for the private Catalog Source image [Serial]", func() {
		g.By("1) Create a pull secert for CatalogSource")
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		dockerConfig := filepath.Join(buildPruningBaseDir, "dockerconfig.json")
		_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift-marketplace", "secret", "generic", "secret-37826", fmt.Sprintf("--from-file=.dockerconfigjson=%s", dockerConfig), "--type=kubernetes.io/dockerconfigjson").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-marketplace", "secret", "secret-37826").Execute()

		g.By("2) Install this private CatalogSource in the openshift-marketplace project")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		cs := catalogSourceDescription{
			name:        "cs-37826",
			namespace:   "openshift-marketplace",
			displayName: "OLM QE Operators",
			publisher:   "Jian",
			sourceType:  "grpc",
			address:     "quay.io/olmqe/cs:private",
			template:    csImageTemplate,
			secret:      "secret-37826",
		}
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		cs.create(oc, itName, dr)
		defer cs.delete(itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", "openshift-marketplace", "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("4) Install the etcdoperator v0.9.4 from this private image")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		sub := subscriptionDescription{
			subName:                "sub-37826",
			namespace:              "openshift-operators",
			catalogSourceName:      "cs-37826",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "clusterwide-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.4-clusterwide",
			singleNamespace:        true,
			template:               subTemplate,
		}
		defer sub.delete(itName, dr)
		sub.create(oc, itName, dr)
		defer sub.deleteCSV(itName, dr)

		// get the InstallPlan name
		ipName := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}")
		if strings.Contains(ipName, "NotFound") {
			e2e.Failf("!!!Fail to get the InstallPlan of sub: %s/%s", sub.namespace, sub.subName)
		}
		// get the unpack job name
		manifest := getResource(oc, asAdmin, withoutNamespace, "ip", "-n", sub.namespace, ipName, "-o=jsonpath={.status.plan[0].resource.manifest}")
		valid := regexp.MustCompile(`name":"(\S+)","namespace"`)
		job := valid.FindStringSubmatch(manifest)
		g.By("5) Only check if the job pod works well")
		// in this test case, we don't need to care about if the operator pods works well.
		// more details: https://bugzilla.redhat.com/show_bug.cgi?id=1909992#c5
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"-n", "openshift-marketplace", "pods", "-l", fmt.Sprintf("job-name=%s", string(job[1])), "-o=jsonpath={.items[0].status.phase}"}).check(oc)

	})

	// author: jiazha@redhat.com
	g.It("Author:jiazha-High-37442-create a Conditions CR for each Operator it installs", func() {
		g.By("1) Install the OperatorGroup in a random project")
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		oc.SetupProject()
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		og := operatorGroupDescription{
			name:      "og-37442",
			namespace: oc.Namespace(),
			template:  ogSingleTemplate,
		}
		og.createwithCheck(oc, itName, dr)

		g.By("2) Install the etcdoperator v0.9.4 with Automatic approval")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		sub := subscriptionDescription{
			subName:                "sub-37442",
			namespace:              oc.Namespace(),
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.4",
			singleNamespace:        true,
			template:               subTemplate,
		}
		sub.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("3) Check if OperatorCondition generated well")
		newCheck("expect", asAdmin, withoutNamespace, compare, "etcd-operator", ok, []string{"operatorcondition", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.spec.deployments[0]}"}).check(oc)
		// three containers: etcd-operator etcd-backup-operator etcd-restore-operator
		newCheck("expect", asAdmin, withoutNamespace, compare, "etcdoperator.v0.9.4 etcdoperator.v0.9.4 etcdoperator.v0.9.4", ok, []string{"deployment", "etcd-operator", "-n", oc.Namespace(), "-o=jsonpath={.spec.template.spec.containers[*].env[?(@.name==\"OPERATOR_CONDITION_NAME\")].value}"}).check(oc)
		// this etcdoperator.v0.9.4 role should be owned by OperatorCondition
		newCheck("expect", asAdmin, withoutNamespace, compare, "OperatorCondition", ok, []string{"role", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.metadata.ownerReferences[0].kind}"}).check(oc)
		// this etcdoperator.v0.9.4 role should be added to etcd-operator SA
		newCheck("expect", asAdmin, withoutNamespace, compare, "etcd-operator", ok, []string{"rolebinding", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.subjects[0].name}"}).check(oc)

		g.By("4) delete the operator so that can check the related resource in next step")
		sub.delete(itName, dr)
		sub.deleteCSV(itName, dr)

		g.By("5) Check if the related resources are removed successfully")
		newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"operatorcondition", "etcdoperator.v0.9.4", "-n", oc.Namespace()}).check(oc)
		newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"role", "etcdoperator.v0.9.4", "-n", oc.Namespace()}).check(oc)
		newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"rolebinding", "etcdoperator.v0.9.4", "-n", oc.Namespace()}).check(oc)

	})

	// author: jiazha@redhat.com
	g.It("ConnectedOnly-Author:jiazha-Medium-37710-supports the Upgradeable Supported Condition", func() {
		g.By("1) Install the OperatorGroup in a random project")
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		oc.SetupProject()
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		og := operatorGroupDescription{
			name:      "og-37710",
			namespace: oc.Namespace(),
			template:  ogSingleTemplate,
		}
		og.createwithCheck(oc, itName, dr)

		g.By("2) Install the etcdoperator v0.9.2 with Manual approval")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		sub := subscriptionDescription{
			subName:                "sub-37710",
			namespace:              oc.Namespace(),
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Manual",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.2",
			singleNamespace:        true,
			template:               subTemplate,
		}
		sub.create(oc, itName, dr)

		g.By("3) Apprrove this etcdoperator.v0.9.2, it should be in Complete state")
		sub.approveSpecificIP(oc, itName, dr, "etcdoperator.v0.9.2", "Complete")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "etcdoperator.v0.9.2", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("4) Creates a proxy server or application-level gateway between localhost and the Kubernetes API Server")
		// Create a random port during 10000 - 65535
		rand.Seed(time.Now().UnixNano())
		port := rand.Intn(65535-10000) + 10000
		// Release this port after test complete
		defer func() {
			outPut, _ := exec.Command("lsof", fmt.Sprintf("-i:%d", port)).CombinedOutput()
			valid := regexp.MustCompile("oc\\s+\\d+")
			pid := valid.FindAllString(string(outPut[:]), -1)
			exec.Command("kill", "-9", pid[0][3:]).CombinedOutput()
		}()
		// Use another termianl do run this server
		go func() {
			defer g.GinkgoRecover()
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("proxy").Args(fmt.Sprintf("--port=%d", port), "&").Output()
			if strings.Contains(msg, "address already in use") {
				e2e.Failf("!!!Fail to run this server")
			}
		}()

		g.By("5) Patch the status of the Upgradeable to False")
		// wait 3 seconds to waiting for the proxy server ready
		time.Sleep(3 * time.Second)
		// In proxy cluster, get ERROR: The requested URL could not be retrieved
		serverPath := fmt.Sprintf("localhost:%d/apis/operators.coreos.com/v1/namespaces/%s/operatorconditions/etcdoperator.v0.9.2/status", port, oc.Namespace())
		outPut, err := exec.Command("curl", "-X", "PATCH", "-H", "Content-Type: application/merge-patch+json", "--data", "{\"status\":{\"conditions\":[{\"lastTransitionTime\":\"2020-12-17T15:39:01Z\",\"message\":\"Test\",\"reason\":\"NotUpgradeable\",\"status\":\"False\",\"type\":\"Upgradeable\"}]}}", serverPath).CombinedOutput()
		e2e.Logf("!!! command output:\n%s", string(outPut[:]))
		o.Expect(err).NotTo(o.HaveOccurred())

		newCheck("expect", asAdmin, withoutNamespace, compare, "Upgradeable", ok, []string{"operatorcondition", "etcdoperator.v0.9.2", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[0].type}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "False", ok, []string{"operatorcondition", "etcdoperator.v0.9.2", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[0].status}"}).check(oc)

		g.By("6) Apprrove this etcdoperator.v0.9.4, the corresponding CSV should be in Pending state")
		sub.approveSpecificIP(oc, itName, dr, "etcdoperator.v0.9.4", "Complete")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Pending", ok, []string{"csv", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("7) Check the CSV message, the operator is not upgradeable")
		err = wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", oc.Namespace(), "csv", "etcdoperator.v0.9.4", "-o=jsonpath={.status.message}").Output()
			if !strings.Contains(msg, "operator is not upgradeable") {
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Medium-37631-Allow cluster admin to overwrite the OperatorCondition", func() {
		g.By("1) Install the OperatorGroup in a random project")
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		oc.SetupProject()
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		og := operatorGroupDescription{
			name:      "og-37631",
			namespace: oc.Namespace(),
			template:  ogSingleTemplate,
		}
		og.createwithCheck(oc, itName, dr)

		g.By("2) Install the etcdoperator v0.9.2 with Manual approval")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		sub := subscriptionDescription{
			subName:                "sub-37631",
			namespace:              oc.Namespace(),
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Manual",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.2",
			singleNamespace:        true,
			template:               subTemplate,
		}
		sub.create(oc, itName, dr)

		g.By("3) Apprrove this etcdoperator.v0.9.2, it should be in Complete state")
		sub.approveSpecificIP(oc, itName, dr, "etcdoperator.v0.9.2", "Complete")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "etcdoperator.v0.9.2", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("4) Patch the OperatorCondition to set the Upgradeable to False")
		patchResource(oc, asAdmin, withoutNamespace, "-n", oc.Namespace(), "operatorcondition", "etcdoperator.v0.9.2", "-p", "{\"spec\": {\"overrides\": [{\"type\": \"Upgradeable\", \"status\": \"False\", \"reason\": \"upgradeIsNotSafe\", \"message\": \"Disbale the upgrade\"}]}}", "--type=merge")

		g.By("5) Apprrove this etcdoperator.v0.9.4, the corresponding CSV should be in Pending state")
		sub.approveSpecificIP(oc, itName, dr, "etcdoperator.v0.9.4", "Complete")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Pending", ok, []string{"csv", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("6) Check the CSV message, the operator is not upgradeable")
		err := wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", oc.Namespace(), "csv", "etcdoperator.v0.9.4", "-o=jsonpath={.status.message}").Output()
			if !strings.Contains(msg, "operator is not upgradeable") {
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7) Change the Upgradeable of the OperatorCondition to True")
		patchResource(oc, asAdmin, withoutNamespace, "-n", oc.Namespace(), "operatorcondition", "etcdoperator.v0.9.2", "-p", "{\"spec\": {\"overrides\": [{\"type\": \"Upgradeable\", \"status\": \"True\", \"reason\": \"upgradeIsNotSafe\", \"message\": \"Disbale the upgrade\"}]}}", "--type=merge")

		g.By("8) the etcdoperator.v0.9.2 should be upgraded to etcdoperator.v0.9.4 successfully")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)
	})

	// author: jiazha@redhat.com
	g.It("ConnectedOnly-Author:jiazha-Medium-33450-Operator upgrades can delete existing CSV before completion", func() {
		g.By("1) Install a customization CatalogSource CR")
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		cs := catalogSourceDescription{
			name:        "cs-33450",
			namespace:   "openshift-marketplace",
			displayName: "OLM QE Operators",
			publisher:   "Jian",
			sourceType:  "grpc",
			// use the digest in case wrong updates. quay.io/openshifttest/etcd-index:0.9.4-sa
			address:  "quay.io/openshifttest/etcd-index@sha256:f804adfbae165834acdfc83aaf94e1b7ff53246dca607459cdadd4653228cac6",
			template: csImageTemplate,
		}
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		cs.create(oc, itName, dr)
		defer cs.delete(itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", "openshift-marketplace", "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("2) Subscribe to the etcd operator with Manual approval")
		oc.SetupProject()
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")

		og := operatorGroupDescription{
			name:      "og-33450",
			namespace: oc.Namespace(),
			template:  ogSingleTemplate,
		}
		og.createwithCheck(oc, itName, dr)

		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		sub := subscriptionDescription{
			subName:                "sub-33450",
			namespace:              oc.Namespace(),
			catalogSourceName:      "cs-33450",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "alpha",
			ipApproval:             "Manual",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.2",
			singleNamespace:        true,
			template:               subTemplate,
		}
		defer sub.delete(itName, dr)
		defer sub.deleteCSV(itName, dr)
		sub.create(oc, itName, dr)
		g.By("3) Apprrove the etcdoperator.v0.9.2, it should be in Complete state")
		sub.approveSpecificIP(oc, itName, dr, "etcdoperator.v0.9.2", "Complete")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "etcdoperator.v0.9.2", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("4) Apprrove the etcdoperator.v0.9.4, it should be in Failed state")
		sub.approveSpecificIP(oc, itName, dr, "etcdoperator.v0.9.4", "Failed")

		g.By("5) The etcdoperator.v0.9.4 CSV should be in Pending status")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Pending", ok, []string{"csv", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("6) The SA should be owned by the etcdoperator.v0.9.2")
		err := wait.Poll(3*time.Second, 10*time.Second, func() (bool, error) {
			saOwner := getResource(oc, asAdmin, withoutNamespace, "sa", "etcd-operator", "-n", sub.namespace, "-o=jsonpath={.metadata.ownerReferences[0].name}")
			if strings.Compare(saOwner, "etcdoperator.v0.9.2") != 0 {
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	// author: jiazha@redhat.com
	g.It("ConnectedOnly-Author:jiazha-High-37260-should allow to create the default CatalogSource [Disruptive]", func() {
		g.By("1) Disable the default OperatorHub")
		patchResource(oc, asAdmin, withoutNamespace, "operatorhub", "cluster", "-p", "{\"spec\": {\"disableAllDefaultSources\": true}}", "--type=merge")
		defer patchResource(oc, asAdmin, withoutNamespace, "operatorhub", "cluster", "-p", "{\"spec\": {\"disableAllDefaultSources\": false}}", "--type=merge")
		g.By("1-1) Check if the default CatalogSource resource are removed")
		err := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			res, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "redhat-operators", "-n", "openshift-marketplace").Output()
			if strings.Contains(res, "not found") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2) Create a CatalogSource with a default CatalogSource name")
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		oc.SetupProject()
		cs := catalogSourceDescription{
			name:        "redhat-operators",
			namespace:   "openshift-marketplace",
			displayName: "OLM QE",
			publisher:   "OLM QE",
			sourceType:  "grpc",
			address:     "quay.io/openshift-qe-optional-operators/ocp4-index:latest",
			template:    csImageTemplate,
		}
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		cs.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)
		g.By("2-1) Check if this custom CatalogSource resource works well")
		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			res, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest").Output()
			if strings.Contains(res, "OLM QE") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("3) Delete the Marketplace pods and check if the custome CatalogSource still works well")
		g.By("3-1) get the marketplace-operator pod's name")
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", "name=marketplace-operator", "-o=jsonpath={.items..metadata.name}", "-n", "openshift-marketplace").Output()
		if err != nil {
			e2e.Failf("Failed to get the marketplace pods")
		}
		g.By("3-2) delete/recreate the marketplace-operator pod")
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("pods", podName, "-n", "openshift-marketplace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// time.Sleep(30 * time.Second)
		// waiting for the new marketplace pod ready
		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			res, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", "name=marketplace-operator", "-o=jsonpath={.items..status.phase}", "-n", "openshift-marketplace").Output()
			if strings.Contains(res, "Running") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("3-3) check if the custom CatalogSource still there")
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)
		err = wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
			res, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest").Output()
			if strings.Contains(res, "OLM QE") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4) Enable the default OperatorHub")
		patchResource(oc, true, true, "operatorhub", "cluster", "-p", "{\"spec\": {\"disableAllDefaultSources\": false}}", "--type=merge")
		g.By("4-1) Check if the default CatalogSource resource are back")
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", "redhat-operators", "-n", "openshift-marketplace", "-o=jsonpath={.status..lastObservedState}"}).check(oc)
		g.By("4-2) Check if the default CatalogSource works and the custom one are removed")
		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			res, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest").Output()
			if strings.Contains(res, "Red Hat Operators") && !strings.Contains(res, "OLM QE") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Medium-25922-Support spec.config.volumes and volumemount in Subscription", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		oc.SetupProject()
		og := operatorGroupDescription{
			name:      "test-og-25922",
			namespace: oc.Namespace(),
			template:  ogSingleTemplate,
		}
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		g.By(fmt.Sprintf("1) create the OperatorGroup in project: %s", oc.Namespace()))
		og.createwithCheck(oc, itName, dr)

		g.By("2) install etcd operator")
		sub := subscriptionDescription{
			subName:                "etcd-sub",
			namespace:              oc.Namespace(),
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			singleNamespace:        true,
			template:               subTemplate,
		}
		sub.create(oc, itName, dr)

		g.By("3) create a ConfigMap")
		cmTemplate := filepath.Join(buildPruningBaseDir, "cm-template.yaml")

		cm := configMapDescription{
			name:      "special-config",
			namespace: oc.Namespace(),
			template:  cmTemplate,
		}
		cm.create(oc, itName, dr)

		g.By("4) Patch this ConfigMap a volume")
		sub.patch(oc, "{\"spec\": {\"channel\":\"singlenamespace-alpha\",\"config\":{\"volumeMounts\":[{\"mountPath\":\"/test\",\"name\":\"config-volume\"}],\"volumes\":[{\"configMap\":{\"name\":\"special-config\"},\"name\":\"config-volume\"}]},\"name\":\"etcd\",\"source\":\"community-operators\",\"sourceNamespace\":\"openshift-marketplace\"}}")
		err := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			podName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "name=etcd-operator-alm-owned", "-o=jsonpath={.items[0].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("4-1) Get etcd operator pod name:%s", podName)
			result, err := oc.AsAdmin().Run("exec").Args(podName, "--", "cat", "/test/special.how").Output()
			e2e.Logf("4-2) Check if the ConfigMap mount well")
			if strings.Contains(result, "very") {
				e2e.Logf("4-3) The ConfigMap: special-config mount well")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("5) Patch a non-exist volume")
		sub.patch(oc, "{\"spec\":{\"channel\":\"singlenamespace-alpha\",\"config\":{\"volumeMounts\":[{\"mountPath\":\"/test\",\"name\":\"volume1\"}],\"volumes\":[{\"persistentVolumeClaim\":{\"claimName\":\"claim1\"},\"name\":\"volume1\"}]},\"name\":\"etcd\",\"source\":\"community-operators\",\"sourceNamespace\":\"openshift-marketplace\"}}")
		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			for i := 0; i < 2; i++ {
				g.By("5-1) Check the pods status")
				podStatus, err := oc.AsAdmin().Run("get").Args("pods", "-l", "name=etcd-operator-alm-owned", fmt.Sprintf("-o=jsonpath={.items[%d].status.phase}", i)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if podStatus == "Pending" {
					g.By("5-2) The pod status is Pending as expected")
					return true, nil
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Medium-35631-Remove OperatorSource API", func() {
		g.By("1) Check the operatorsource resource")
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("operatorsource").Output()
		e2e.Logf("Get the expected error: %s", msg)
		o.Expect(msg).To(o.ContainSubstring("the server doesn't have a resource type"))

		// for current disconnected env, only have the default community CatalogSource CRs
		g.By("2) Check the default Community CatalogSource CRs")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "-n", "openshift-marketplace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Get the installed CatalogSource CRs:\n %s", msg)
		// o.Expect(msg).To(o.ContainSubstring("certified-operators"))
		o.Expect(msg).To(o.ContainSubstring("community-operators"))
		// o.Expect(msg).To(o.ContainSubstring("redhat-marketplace"))
		// o.Expect(msg).To(o.ContainSubstring("redhat-operators"))
		g.By("3) Check the Packagemanifest")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).NotTo(o.ContainSubstring("No resources found"))
	})

	// author: bandrade@redhat.com
	g.It("ConnectedOnly-Author:bandrade-Medium-31693-Check CSV information on the PackageManifest", func() {
		g.By("1) The relatedImages should exist")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "prometheus", "-o=jsonpath={.status.channels[?(.name=='beta')].currentCSVDesc.relatedImages}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("quay.io/coreos/prometheus-operator"))

		g.By("2) The minKubeVersion should exist")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "prometheus", "-o=jsonpath={.status.channels[?(.name=='beta')].currentCSVDesc.minKubeVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("1.11.0"))

		g.By("3) In this case, nativeAPI is optional, and prometheus does not have any nativeAPIs, which is ok.")
		oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "prometheus", "-o=jsonpath={.status.channels[?(.name=='beta')].currentCSVDesc.nativeAPIs}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Medium-24850-Allow users to edit the deployment of an active CSV", func() {

		oc.SetupProject()

		g.By("1)Start to subscribe the Etcd operator")
		etcdPackage := CreateSubscriptionSpecificNamespace("etcd", oc, false, true, oc.Namespace(), INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(etcdPackage, oc)

		g.By("2) Patch the deploy object by adding an environment variable")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("deploy/etcd-operator", "--type=json", "--patch", "[{\"op\": \"add\",\"path\": \"/spec/template/spec/containers/0/env/-\", \"value\": { \"name\": \"a\",\"value\": \"b\"} }]", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("3) Check if the pod is restared")
		var waitErr = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace()).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "etcd-operator") && strings.Contains(msg, "Terminating") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred())

	})
	// author: jiazha@redhat.com
	g.It("Author:jiazha-ConnectedOnly-Medium-33902-Catalog Weighting", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")

		// the priority ranking is bucket-test1 > bucket-test2 > community-operators(-400 default)
		csObjects := []struct {
			name     string
			address  string
			priority int
		}{
			{"ocs-cs", "quay.io/olmqe/ocs-index:4.3.0", 0},
			{"bucket-test1", "quay.io/olmqe/bucket-index:1.0.0", 20},
			{"bucket-test2", "quay.io/olmqe/bucket-index:1.0.0", -1},
		}

		// create the openshift-storage project
		project := projectDescription{
			name: "openshift-storage",
		}

		// create the OperatorGroup resource
		og := operatorGroupDescription{
			name:      "test-og-33902",
			namespace: "openshift-storage",
			template:  ogSingleTemplate,
		}

		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		for i, v := range csObjects {
			g.By(fmt.Sprintf("%d) start to create the %s CatalogSource", i+1, v.name))
			cs := catalogSourceDescription{
				name:        v.name,
				namespace:   "openshift-marketplace",
				displayName: "Priority Test",
				publisher:   "OLM QE",
				sourceType:  "grpc",
				address:     v.address,
				template:    csImageTemplate,
				priority:    v.priority,
			}
			cs.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status.connectionState.lastObservedState}"}).check(oc)
		}

		g.By("4) create the openshift-storage project")
		project.createwithCheck(oc, itName, dr)

		g.By("5) create the OperatorGroup")
		og.createwithCheck(oc, itName, dr)

		g.By("6) start to subscribe to the OCS operator")
		sub := subscriptionDescription{
			subName:                "ocs-sub",
			namespace:              "openshift-storage",
			catalogSourceName:      "ocs-cs",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "4.3.0",
			ipApproval:             "Automatic",
			operatorPackage:        "ocs-operator",
			singleNamespace:        true,
			template:               subTemplate,
		}
		sub.create(oc, itName, dr)

		g.By("7) check the dependce operator's subscription")
		depSub := subscriptionDescription{
			subName:                "lib-bucket-provisioner-4.3.0-bucket-test1-openshift-marketplace",
			namespace:              "openshift-storage",
			catalogSourceName:      "bucket-test1",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "4.3.0",
			ipApproval:             "Automatic",
			operatorPackage:        "lib-bucket-provisioner",
			singleNamespace:        true,
			template:               subTemplate,
		}
		// The dependence is lib-bucket-provisioner-4.3.0, it should from the bucket-test1 CatalogSource since its priority is the highest.
		dr.getIr(itName).add(newResource(oc, "sub", depSub.subName, requireNS, depSub.namespace))
		depSub.findInstalledCSV(oc, itName, dr)

		g.By(fmt.Sprintf("8) Remove subscription:%s, %s", sub.subName, depSub.subName))
		sub.delete(itName, dr)
		sub.deleteCSV(itName, dr)
		depSub.delete(itName, dr)
		depSub.getCSV().delete(itName, dr)

		for _, v := range csObjects {
			g.By(fmt.Sprintf("9) Remove the %s CatalogSource", v.name))
			cs := catalogSourceDescription{
				name:        v.name,
				namespace:   "openshift-marketplace",
				displayName: "Priority Test",
				publisher:   "OLM QE",
				sourceType:  "grpc",
				address:     v.address,
				template:    csImageTemplate,
				priority:    v.priority,
			}
			cs.delete(itName, dr)
		}

	})

	// author: jiazha@redhat.com
	g.It("ConnectedOnly-Author:jiazha-Medium-32560-Unpacking bundle in InstallPlan fails", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		g.By("Start to create the CatalogSource CR")
		cs := catalogSourceDescription{
			name:        "bug-1798645-cs",
			namespace:   "openshift-marketplace",
			displayName: "OLM QE",
			publisher:   "OLM QE",
			sourceType:  "grpc",
			address:     "quay.io/olmtest/single-bundle-index:1.0.0",
			template:    csImageTemplate,
		}
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		cs.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("Start to subscribe the Kiali operator")
		sub := subscriptionDescription{
			subName:                "bug-1798645-sub",
			namespace:              "openshift-operators",
			catalogSourceName:      "bug-1798645-cs",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "stable",
			ipApproval:             "Automatic",
			operatorPackage:        "kiali",
			singleNamespace:        false,
			template:               subTemplate,
		}
		sub.create(oc, itName, dr)
		newCheck("expect", asAdmin, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Remove catalog and sub")
		sub.delete(itName, dr)
		sub.deleteCSV(itName, dr)
		cs.delete(itName, dr)
	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Medium-24771-OLM should support for user defined ServiceAccount for OperatorGroup", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		saRoles := filepath.Join(buildPruningBaseDir, "scoped-sa-roles.yaml")
		oc.SetupProject()
		namespace := oc.Namespace()
		ogSAtemplate := filepath.Join(buildPruningBaseDir, "operatorgroup-serviceaccount.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		csv := "etcdoperator.v0.9.4"
		sa := "scoped-24771"

		// create the namespace
		project := projectDescription{
			name: namespace,
		}

		// create the OperatorGroup resource
		og := operatorGroupDescription{
			name:               "test-og-24771",
			namespace:          namespace,
			serviceAccountName: sa,
			template:           ogSAtemplate,
		}

		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		g.By("1) Create the namespace")
		project.createwithCheck(oc, itName, dr)

		g.By("2) Create the OperatorGroup")
		og.createwithCheck(oc, itName, dr)

		g.By("3) Create the service account")
		_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", sa, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4) Create a Subscription")
		sub := subscriptionDescription{
			subName:                "etcd",
			namespace:              namespace,
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			singleNamespace:        true,
			template:               subTemplate,
			startingCSV:            csv,
		}
		sub.createWithoutCheck(oc, itName, dr)

		g.By("5) The install plan is Failed")
		var installPlan string
		waitErr := wait.Poll(3*time.Second, 240*time.Second, func() (bool, error) {
			installPlan, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}").Output()
			if strings.Compare(installPlan, "") == 0 || err != nil {
				return false, nil
			}
			return true, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred())
		o.Expect(installPlan).NotTo(o.BeEmpty())
		newCheck("expect", asAdmin, withoutNamespace, compare, "Failed", ok, []string{"ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("6) Grant the proper permissions to the service account")
		_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", saRoles, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7) Recreate the Subscription")
		sub.delete(itName, dr)
		sub.deleteCSV(itName, dr)
		sub.createWithoutCheck(oc, itName, dr)

		g.By("8) Checking the state of CSV")
		newCheck("expect", asUser, withNamespace, compare, "Succeeded", ok, []string{"csv", csv, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Medium-24772-OLM should support for user defined ServiceAccount for OperatorGroup with fine grained permission", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		saRoles := filepath.Join(buildPruningBaseDir, "scoped-sa-fine-grained-roles.yaml")
		oc.SetupProject()
		namespace := oc.Namespace()
		ogSAtemplate := filepath.Join(buildPruningBaseDir, "operatorgroup-serviceaccount.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		csv := "etcdoperator.v0.9.4"
		sa := "scoped-24772"

		// create the namespace
		project := projectDescription{
			name: namespace,
		}

		// create the OperatorGroup resource
		og := operatorGroupDescription{
			name:               "test-og-24772",
			namespace:          namespace,
			serviceAccountName: sa,
			template:           ogSAtemplate,
		}

		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		g.By("1) Create the namespace")
		project.createwithCheck(oc, itName, dr)

		g.By("2) Create the OperatorGroup")
		og.createwithCheck(oc, itName, dr)

		g.By("3) Create the service account")
		_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", sa, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4) Create a Subscription")
		sub := subscriptionDescription{
			subName:                "etcd",
			namespace:              namespace,
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			singleNamespace:        true,
			template:               subTemplate,
			startingCSV:            csv,
		}
		sub.createWithoutCheck(oc, itName, dr)

		g.By("5) The install plan is Failed")
		var installPlan string
		err = wait.Poll(3*time.Second, 240*time.Second, func() (bool, error) {
			installPlan, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}").Output()
			if strings.Compare(installPlan, "") == 0 || err != nil {
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", asAdmin, withoutNamespace, compare, "Failed", ok, []string{"ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("6) Grant the proper permissions to the service account")
		_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", saRoles, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7) Recreate the Subscription")
		sub.delete(itName, dr)
		sub.deleteCSV(itName, dr)
		sub.createWithoutCheck(oc, itName, dr)

		g.By("8) Checking the state of CSV")
		newCheck("expect", asUser, withNamespace, compare, "Succeeded", ok, []string{"csv", csv, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Medium-24886-OLM should support for user defined ServiceAccount permission changes", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		saRoles := filepath.Join(buildPruningBaseDir, "scoped-sa-etcd.yaml")
		oc.SetupProject()
		namespace := oc.Namespace()
		ogTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		ogSAtemplate := filepath.Join(buildPruningBaseDir, "operatorgroup-serviceaccount.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		csv := "etcdoperator.v0.9.4"
		sa := "scoped-24886"

		// create the namespace
		project := projectDescription{
			name: namespace,
		}

		// create the OperatorGroup resource
		og := operatorGroupDescription{
			name:      "test-og-24886",
			namespace: namespace,
			template:  ogTemplate,
		}

		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		g.By("1) Create the namespace")
		project.createwithCheck(oc, itName, dr)

		g.By("2) Create the OperatorGroup without service account")
		og.createwithCheck(oc, itName, dr)

		g.By("3) Create a Subscription")
		sub := subscriptionDescription{
			subName:                "etcd",
			namespace:              namespace,
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			singleNamespace:        true,
			template:               subTemplate,
			startingCSV:            csv,
		}
		sub.create(oc, itName, dr)

		g.By("4) Checking the state of CSV")
		newCheck("expect", asUser, withNamespace, compare, "Succeeded", ok, []string{"csv", csv, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("5) Delete the Operator Group")
		og.delete(itName, dr)

		// create the OperatorGroup resource
		ogSA := operatorGroupDescription{
			name:               "test-og-24886",
			namespace:          namespace,
			serviceAccountName: sa,
			template:           ogSAtemplate,
		}
		g.By("6) Create the OperatorGroup with service account")
		ogSA.createwithCheck(oc, itName, dr)

		g.By("7) Create the service account")
		_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", sa, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("9) Grant the proper permissions to the service account")
		_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", saRoles, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("10) Recreate the Subscription")
		sub.delete(itName, dr)
		sub.deleteCSV(itName, dr)
		sub.createWithoutCheck(oc, itName, dr)

		g.By("11) Checking the state of CSV")
		newCheck("expect", asUser, withNamespace, compare, "Succeeded", ok, []string{"csv", csv, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

	})
	// author: bandrade@redhat.com
	g.It("ConnectedOnly-Author:bandrade-Medium-30765-Operator-version based dependencies metadata", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")

		oc.SetupProject()
		g.By("Start to create the CatalogSource CR")
		cs := catalogSourceDescription{
			name:        "prometheus-dependency-cs",
			namespace:   "openshift-marketplace",
			displayName: "OLM QE",
			publisher:   "OLM QE",
			sourceType:  "grpc",
			address:     "quay.io/olmqe/etcd-prometheus-dependency-index:8.0",
			template:    csImageTemplate,
		}
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		defer cs.delete(itName, dr)
		cs.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("Start to subscribe the Etcd operator")
		etcdPackage := CreateSubscriptionSpecificNamespace("etcd-prometheus", oc, false, true, oc.Namespace(), INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(etcdPackage, oc)

		g.By("Assert that prometheus dependency is resolved")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("prometheus"))

	})

	// author: bandrade@redhat.com
	g.It("ConnectedOnly-Author:bandrade-Medium-27680-OLM Bundle support for Prometheus Types", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")

		oc.SetupProject()
		g.By("Start to create the CatalogSource CR")
		cs := catalogSourceDescription{
			name:        "prometheus-dependency1-cs",
			namespace:   "openshift-marketplace",
			displayName: "OLM QE",
			publisher:   "OLM QE",
			sourceType:  "grpc",
			address:     "quay.io/olmqe/etcd-prometheus-dependency-index:9.0",
			template:    csImageTemplate,
		}
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		defer cs.delete(itName, dr)
		cs.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("Start to subscribe the Etcd operator")
		etcdPackage := CreateSubscriptionSpecificNamespace("etcd-service-monitor", oc, false, true, oc.Namespace(), INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(etcdPackage, oc)

		g.By("Assert that prometheus dependency is resolved")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("prometheus"))

		g.By("Assert that ServiceMonitor is created")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ServiceMonitor", "my-servicemonitor", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("my-servicemonitor"))

		g.By("Assert that PrometheusRule is created")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("PrometheusRule", "my-prometheusrule", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("my-prometheusrule"))

	})

	// author: bandrade@redhat.com
	g.It("ConnectedOnly-Author:bandrade-Medium-24916-Operators in AllNamespaces should be granted namespace list", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		g.By("Start to subscribe the AMQ-Streams operator")
		sub := subscriptionDescription{
			subName:                "jaeger-product",
			namespace:              "openshift-operators",
			catalogSourceName:      "redhat-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "stable",
			ipApproval:             "Automatic",
			operatorPackage:        "jaeger-product",
			singleNamespace:        false,
			template:               subTemplate,
		}

		defer sub.delete(itName, dr)

		sub.create(oc, itName, dr)
		defer sub.deleteCSV(itName, dr)
		newCheck("expect", asAdmin, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)
		msg, err := oc.AsAdmin().WithoutNamespace().Run("policy").Args("who-can", "list", "namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("system:serviceaccount:openshift-operators:jaeger-operator"))
	})
	// author: jiazha@redhat.com
	g.It("Author:jiazha-High-32559-catalog operator crashed", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csImageTemplate := filepath.Join(buildPruningBaseDir, "cs-without-image.yaml")
		csTypes := []struct {
			name        string
			csType      string
			expectedMSG string
		}{
			{"cs-noimage", "grpc", "image and address unset"},
			{"cs-noimage-cm", "configmap", "configmap name unset"},
		}
		for _, t := range csTypes {
			g.By(fmt.Sprintf("test the %s type CatalogSource", t.csType))
			cs := catalogSourceDescription{
				name:        t.name,
				namespace:   "openshift-marketplace",
				displayName: "OLM QE",
				publisher:   "OLM QE",
				sourceType:  t.csType,
				template:    csImageTemplate,
			}
			dr := make(describerResrouce)
			itName := g.CurrentGinkgoTestDescription().TestText
			dr.addIr(itName)
			cs.create(oc, itName, dr)

			err := wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "catalogsource", cs.name, "-o=jsonpath={.status.message}").Output()
				if err != nil {
					e2e.Logf("Fail to get CatalogSource: %s, error: %s and try again", cs.name, err)
					return false, nil
				}
				if strings.Contains(output, t.expectedMSG) {
					e2e.Logf("Get expected message: %s", t.expectedMSG)
					return true, nil
				}
				return false, nil
			})

			o.Expect(err).NotTo(o.HaveOccurred())

			status, err := oc.AsAdmin().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "pods", "-l", "app=catalog-operator", "-o=jsonpath={.items[0].status.phase}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if status != "Running" {
				e2e.Failf("The status of the CatalogSource: %s pod is: %s", cs.name, status)
			}
		}

		//destroy the two CatalogSource CRs
		for _, t := range csTypes {
			_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-marketplace", "catalogsource", t.name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Critical-22070-support grpc sourcetype [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		csTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		oc.SetupProject()

		g.By("Start to create the CatalogSource CR")
		cs := catalogSourceDescription{
			name:        "cs-22070",
			namespace:   "openshift-marketplace",
			displayName: "OLM QE",
			publisher:   "OLM QE",
			sourceType:  "grpc",
			// use the quay.io/openshifttest/etcd-index:auto as index image
			address:  "quay.io/openshifttest/etcd-index@sha256:6cd5cb26dd37c25d432c5b2fe7334f695f680a1810b9dfb0ac5de6be5619fcda",
			template: csTemplate,
		}

		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		defer cs.delete(itName, dr)
		cs.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("Start to subscribe this etcd operator")
		sub := subscriptionDescription{
			subName:                "sub-22070",
			namespace:              "openshift-operators",
			catalogSourceName:      "cs-22070",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "clusterwide-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			singleNamespace:        false,
			template:               subTemplate,
		}
		defer sub.delete(itName, dr)
		defer sub.deleteCSV(itName, dr)
		sub.create(oc, itName, dr)
		newCheck("expect", asAdmin, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Assert that etcd dependency is resolved")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("0.9.4-clusterwide"))
	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Medium-21130-Fetching non-existent `PackageManifest` should return 404", func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "--all-namespaces", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		packageserverLines := strings.Split(msg, "\n")
		if len(packageserverLines) > 0 {
			raw, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "a_package_that_not_exists", "-o yaml", "--loglevel=8").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(raw).To(o.ContainSubstring("\"code\": 404"))
		} else {
			e2e.Failf("No packages to evaluate if 404 works when a PackageManifest does not exists")
		}
	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Low-24057-Have terminationMessagePolicy defined as FallbackToLogsOnError", func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-o=jsonpath={range .items[*].spec}{.containers[*].name}{\"\t\"}", "-n", "openshift-operator-lifecycle-manager").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		amountOfContainers := len(strings.Split(msg, "\t"))

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-o=jsonpath={range .items[*].spec}{.containers[*].terminationMessagePolicy}{\"t\"}", "-n", "openshift-operator-lifecycle-manager").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		regexp := regexp.MustCompile("FallbackToLogsOnError")
		amountOfContainersWithFallbackToLogsOnError := len(regexp.FindAllStringIndex(msg, -1))
		o.Expect(amountOfContainers).To(o.Equal(amountOfContainersWithFallbackToLogsOnError))
		if amountOfContainers != amountOfContainersWithFallbackToLogsOnError {
			e2e.Failf("OLM does not have all containers definied with FallbackToLogsOnError terminationMessagePolicy")
		}
	})

	// author: bandrade@redhat.com
	g.It("ConnectedOnly-Author:bandrade-High-32613-Operators won't install if the CSV dependency is already installed", func() {

		namespace := "kogito"
		infinispanPackage := CreateSubscriptionSpecificNamespace("infinispan", oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(infinispanPackage, oc)
		keycloakPackage := CreateSubscriptionSpecificNamespace("keycloak-operator", oc, false, false, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(keycloakPackage, oc)
		kogitoPackage := CreateSubscriptionSpecificNamespace("kogito-operator", oc, false, false, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(kogitoPackage, oc)
		RemoveOperatorDependencies(kogitoPackage, oc, false)
		RemoveNamespace(namespace, oc)

	})

	// author: bandrade@redhat.com
	g.It("ConnectedOnly-Author:bandrade-Low-24055-Check for defaultChannel mandatory field when having multiple channels", func() {
		olmBaseDir := exutil.FixturePath("testdata", "olm")
		cmMapWithoutDefaultChannel := filepath.Join(olmBaseDir, "configmap-without-defaultchannel.yaml")
		cmMapWithDefaultChannel := filepath.Join(olmBaseDir, "configmap-with-defaultchannel.yaml")
		csNamespaced := filepath.Join(olmBaseDir, "catalogsource-namespace.yaml")

		namespace := "scenario3"
		defer RemoveNamespace(namespace, oc)
		g.By("1) Creating a namespace")
		_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("ns", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2) Creating a ConfigMap without a default channel")
		_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", cmMapWithoutDefaultChannel).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("3) Creating a CatalogSource")
		_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", csNamespaced).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4) Checking CatalogSource error statement due to the absense of a default channel")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l olm.catalogSource=scenario3", "-n", "scenario3").Output()
			if err != nil {
				return false, nil
			}
			if strings.Contains(output, "CrashLoopBackOff") {

				return true, nil
			}
			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("5) Changing the CatalogSource to include default channel for each package")
		_, err = oc.WithoutNamespace().AsAdmin().Run("apply").Args("-f", cmMapWithDefaultChannel).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("6) Checking the state of CatalogSource(Running)")
		err = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l olm.catalogSource=scenario3", "-n", "scenario3").Output()
			if err != nil {
				return false, nil
			}
			if strings.Contains(output, "Running") {

				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Medium-20981-contain the source commit id", func() {
		sameCommit := ""
		subPods := []string{"catalog-operator", "olm-operator", "packageserver"}

		for _, v := range subPods {
			podName, err := oc.AsAdmin().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "pods", "-l", fmt.Sprintf("app=%s", v), "-o=jsonpath={.items[0].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("get pod name:%s", podName)

			g.By(fmt.Sprintf("get olm version from the %s pod", v))
			commands := []string{"-n", "openshift-operator-lifecycle-manager", "exec", podName, "--", "olm", "--version"}
			olmVersion, err := oc.AsAdmin().Run(commands...).Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			idSlice := strings.Split(olmVersion, ":")
			gitCommitID := strings.TrimSpace(idSlice[len(idSlice)-1])
			e2e.Logf("olm source git commit ID:%s", gitCommitID)
			if len(gitCommitID) != 40 {
				e2e.Failf(fmt.Sprintf("the length of the git commit id is %d, != 40", len(gitCommitID)))
			}

			if sameCommit == "" {
				sameCommit = gitCommitID
				g.By("checking this commitID in the operator-lifecycle-manager repo")
				ctx, tc := githubClient()
				client := github.NewClient(tc)
				// OLM downstream repo has been changed to: https://github.com/openshift/operator-framework-olm
				_, _, err := client.Git.GetCommit(ctx, "openshift", "operator-framework-olm", gitCommitID)
				if err != nil {
					e2e.Failf("Git.GetCommit returned error: %v", err)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

			} else if gitCommitID != sameCommit {
				e2e.Failf("These commitIDs inconformity!!!")
			}
		}
	})

	// author: yhui@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-High-30206-Medium-30242-can include secrets and configmaps in the bundle", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		operatorGroup := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		catsrcImage := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		cockroachdbSub := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")

		g.By("create new catalogsource")
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catsrcImage, "-p", "NAME=cockroachdb-catalog-30206", "NAMESPACE=openshift-marketplace", "ADDRESS=quay.io/olmqe/cockroachdb-index:2.0.9new", "DISPLAYNAME=OLMCOCKROACHDB-30206", "PUBLISHER=QE", "SOURCETYPE=grpc").OutputToFile("config-30206.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("catalogsource", "cockroachdb-catalog-30206", "-n", "openshift-marketplace").Execute()

		g.By("create new namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-operators-30206").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-operators-30206").Execute()

		g.By("create operatorGroup")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", "NAMESPACE=test-operators-30206").OutputToFile("config-30206.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create subscription")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cockroachdbSub, "-p", "SUBNAME=test-operator", "SUBNAMESPACE=test-operators-30206", "CHANNEL=alpha", "APPROVAL=Automatic", "OPERATORNAME=cockroachdb", "SOURCENAME=cockroachdb-catalog-30206", "SOURCENAMESPACE=openshift-marketplace", "STARTINGCSV=cockroachdb.v2.0.9").OutputToFile("config-30206.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check secrets")
		err = wait.Poll(30*time.Second, 240*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "test-operators-30206", "secrets", "mysecret").Execute()
			if err != nil {
				e2e.Logf("Failed to create secrets, error:%v", err)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check configmaps")
		err = wait.Poll(30*time.Second, 240*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "test-operators-30206", "configmaps", "my-config-map").Execute()
			if err != nil {
				e2e.Logf("Failed to create secrets, error:%v", err)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("start to test OCP-30242")
		g.By("delete csv")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "test-operators-30206", "csv", "cockroachdb.v2.0.9").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check secrets has been deleted")
		err = wait.Poll(20*time.Second, 120*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "test-operators-30206", "secrets", "mysecret").Execute()
			if err != nil {
				e2e.Logf("The secrets has been deleted")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check configmaps has been deleted")
		err = wait.Poll(20*time.Second, 120*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "test-operators-30206", "configmaps", "my-config-map").Execute()
			if err != nil {
				e2e.Logf("The configmaps has been deleted")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: yhui@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-30312-can allow admission webhook definitions in CSV", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		operatorGroup := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		validatingCsv := filepath.Join(buildPruningBaseDir, "validatingwebhook-csv.yaml")

		g.By("create new namespace")
		newNamespace := "test-operators-30312"
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", newNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", newNamespace).Execute()

		g.By("create operatorGroup")
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", newNamespace)).OutputToFile("config-30312.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create csv")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", validatingCsv, "-p", fmt.Sprintf("NAMESPACE=%s", newNamespace), "OPERATION=CREATE").OutputToFile("config-30312.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(20*time.Second, 180*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("validatingwebhookconfiguration", "-l", "olm.owner.namespace=test-operators-30312").Execute()
			if err != nil {
				e2e.Logf("The validatingwebhookconfiguration is not created:%v", err)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("update csv")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", validatingCsv, "-p", fmt.Sprintf("NAMESPACE=%s", newNamespace), "OPERATION=DELETE").OutputToFile("config-30312.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		validatingwebhookName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("validatingwebhookconfiguration", "-l", "olm.owner.namespace=test-operators-30312", "-o=jsonpath={.items[0].metadata.name}").Output()
		err = wait.Poll(20*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("validatingwebhookconfiguration", validatingwebhookName, "-o=jsonpath={..operations}").Output()
			e2e.Logf(output)
			if err != nil {
				e2e.Logf("DELETE operations cannot be found:%v", err)
				return false, nil
			}
			if strings.Contains(output, "DELETE") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: yhui@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-30317-can allow mutating admission webhook definitions in CSV", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		operatorGroup := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		mutatingCsv := filepath.Join(buildPruningBaseDir, "mutatingwebhook-csv.yaml")

		g.By("create new namespace")
		newNamespace := "test-operators-30317"
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", newNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", newNamespace).Execute()

		g.By("create operatorGroup")
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", newNamespace)).OutputToFile("config-30317.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create csv")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", mutatingCsv, "-p", fmt.Sprintf("NAMESPACE=%s", newNamespace), "OPERATION=CREATE").OutputToFile("config-30317.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(20*time.Second, 180*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mutatingwebhookconfiguration", "-l", "olm.owner.namespace=test-operators-30317").Execute()
			if err != nil {
				e2e.Logf("The mutatingwebhookconfiguration is not created:%v", err)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Start to test 30374")
		g.By("update csv")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", mutatingCsv, "-p", fmt.Sprintf("NAMESPACE=%s", newNamespace), "OPERATION=DELETE").OutputToFile("config-30317.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		mutatingwebhookName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mutatingwebhookconfiguration", "-l", "olm.owner.namespace=test-operators-30317", "-o=jsonpath={.items[0].metadata.name}").Output()
		err = wait.Poll(20*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mutatingwebhookconfiguration", mutatingwebhookName, "-o=jsonpath={..operations}").Output()
			e2e.Logf(output)
			if err != nil {
				e2e.Logf("DELETE operations cannot be found:%v", err)
				return false, nil
			}
			if strings.Contains(output, "DELETE") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: yhui@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-30319-Admission Webhook Configuration names should be unique", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		operatorGroup := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		validatingCsv := filepath.Join(buildPruningBaseDir, "validatingwebhook-csv.yaml")

		var validatingwebhookName1, validatingwebhookName2 string
		for i := 1; i < 3; i++ {
			istr := strconv.Itoa(i)
			g.By("create new namespace")
			newNamespace := "test-operators-30319-"
			newNamespace += istr
			err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", newNamespace).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", newNamespace).Execute()

			g.By("create operatorGroup")
			configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", newNamespace)).OutputToFile("config-30319.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create csv")
			configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", validatingCsv, "-p", fmt.Sprintf("NAMESPACE=%s", newNamespace), "OPERATION=CREATE").OutputToFile("config-30319.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = wait.Poll(20*time.Second, 180*time.Second, func() (bool, error) {
				err := oc.AsAdmin().WithoutNamespace().Run("get").Args("validatingwebhookconfiguration", "-l", fmt.Sprintf("olm.owner.namespace=%s", newNamespace)).Execute()
				if err != nil {
					e2e.Logf("The validatingwebhookconfiguration is not created:%v", err)
					return false, nil
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			validatingwebhookName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("validatingwebhookconfiguration", "-l", fmt.Sprintf("olm.owner.namespace=%s", newNamespace), "-o=jsonpath={.items[0].metadata.name}").Output()
			if i == 1 {
				validatingwebhookName1 = validatingwebhookName
			}
			if i == 2 {
				validatingwebhookName2 = validatingwebhookName
			}
		}
		if validatingwebhookName1 != validatingwebhookName2 {
			e2e.Logf("The test case pass")
		} else {
			err := "The test case fail"
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// author: yhui@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-High-34181-can add conversion webhooks for singleton operators", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		catsrcImage := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		cockroachdbSub := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		crwebhook := filepath.Join(buildPruningBaseDir, "cr-webhookTest.yaml")

		g.By("create new catalogsource")
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catsrcImage, "-p", "NAME=webhook-operator-catalog-34181", "NAMESPACE=openshift-marketplace",
			"ADDRESS=quay.io/olmqe/webhook-operator-index:0.0.3", "DISPLAYNAME=WebhookOperatorCatalog-34181", "PUBLISHER=QE", "SOURCETYPE=grpc").OutputToFile("config-34181.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("catalogsource", "webhook-operator-catalog-34181", "-n", "openshift-marketplace").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create subscription")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cockroachdbSub, "-p", "SUBNAME=test-operator-34181",
			"SUBNAMESPACE=openshift-operators", "CHANNEL=alpha", "APPROVAL=Automatic", "OPERATORNAME=webhook-operator", "SOURCENAME=webhook-operator-catalog-34181",
			"SOURCENAMESPACE=openshift-marketplace").OutputToFile("config-34181.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("sub", "test-operator-34181", "-n", "openshift-operators").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("csv", "webhook-operator.v0.0.1", "-n", "openshift-operators").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", "webhooktests.webhook.operators.coreos.io", "-n", "openshift-operators").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(15*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("api-resources").Args("-o", "name").Output()
			if err != nil {
				e2e.Logf("There is no WebhookTest, err:%v", err)
				return false, nil
			}
			if strings.Contains(output, "webhooktests.webhook.operators.coreos.io") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check invalid CR")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", crwebhook, "-p", "NAME=webhooktest-34181",
			"NAMESPACE=openshift-operators", "VALID=false").OutputToFile("config-34181.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(15*time.Second, 180*time.Second, func() (bool, error) {
			erra := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
			if erra == nil {
				e2e.Logf("expect fail and try next")
				oc.AsAdmin().WithoutNamespace().Run("delete").Args("WebhookTest", "webhooktest-34181", "-n", "openshift-operators").Execute()
				return false, nil
			}
			e2e.Logf("err:%v", err)
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check valid CR")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", crwebhook, "-p", "NAME=webhooktest-34181",
			"NAMESPACE=openshift-operators", "VALID=true").OutputToFile("config-34181.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("WebhookTest", "webhooktest-34181", "-n", "openshift-operators").Execute()
		err = wait.Poll(15*time.Second, 300*time.Second, func() (bool, error) {
			erra := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
			if erra != nil {
				e2e.Logf("try next, err:%v", err)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: yhui@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-High-29809-can complete automatical updates based on replaces", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		operatorGroup := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		catsrcImage := filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		cockroachdbSub := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")

		g.By("create new catalogsource")
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catsrcImage, "-p", "NAME=cockroachdb-catalog-29809",
			"NAMESPACE=openshift-marketplace", "ADDRESS=quay.io/olmqe/cockroachdb-index:2.1.11", "DISPLAYNAME=OLMCOCKROACHDB-REPLACE", "PUBLISHER=QE", "SOURCETYPE=grpc").OutputToFile("config-29809.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("catalogsource", "cockroachdb-catalog-29809", "-n", "openshift-marketplace").Execute()

		g.By("create new namespace")
		newNamespace := "test-operators-29809"
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", newNamespace).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", newNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create operatorGroup")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p",
			"NAME=test-operator", "NAMESPACE="+newNamespace).OutputToFile("config-29809.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create subscription")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cockroachdbSub, "-p", "SUBNAME=test-operator", "SUBNAMESPACE="+newNamespace,
			"CHANNEL=alpha", "APPROVAL=Automatic", "OPERATORNAME=cockroachdb", "SOURCENAME=cockroachdb-catalog-29809", "SOURCENAMESPACE=openshift-marketplace", "STARTINGCSV=cockroachdb.v2.0.9").OutputToFile("config-29809.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(15*time.Second, 480*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", newNamespace, "csv", "cockroachdb.v2.1.11", "-o=jsonpath={.spec.replaces}").Output()
			e2e.Logf(output)
			if err != nil {
				e2e.Logf("The csv is not created, error:%v", err)
				return false, nil
			}
			if strings.Contains(output, "cockroachdb.v2.1.1") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: scolange@redhat.com
	g.It("ConnectedOnly-Author:scolange-Medium-24738-CRD should update if previously defined schemas do not change [Disruptive]", func() {
		var buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		var cfgMap = filepath.Join(buildPruningBaseDir, "configmap-etcd.yaml")
		var patchCfgMap = filepath.Join(buildPruningBaseDir, "configmap-ectd-alpha-beta.yaml")
		var catSource = filepath.Join(buildPruningBaseDir, "catalogsource-configmap.yaml")
		var og = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		var Sub = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		var etcdCluster = filepath.Join(buildPruningBaseDir, "etcd-cluster.yaml")
		var operatorWait = 150 * time.Second

		g.By("check precondition and prepare env")
		if isPresentResource(oc, asAdmin, withoutNamespace, present, "crd", "etcdclusters.etcd.database.coreos.com") && isPresentResource(oc, asAdmin, withoutNamespace, present, "EtcdCluster", "-A") {
			e2e.Logf("It is distruptive case and the resources exists, do not destory it. exit")
			return
		}
		oc.AsAdmin().Run("delete").Args("crd", "etcdclusters.etcd.database.coreos.com").Output()
		oc.AsAdmin().Run("delete").Args("crd", "etcdbackups.etcd.database.coreos.com").Output()
		oc.AsAdmin().Run("delete").Args("crd", "etcdrestores.etcd.database.coreos.com").Output()

		defer oc.AsAdmin().Run("delete").Args("ns", "test-automation-24738").Execute()
		defer oc.AsAdmin().Run("delete").Args("ns", "test-automation-24738-1").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("configmap", "installed-community-24738-global-operators", "-n", "openshift-marketplace").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("catalogsource", "installed-community-24738-global-operators", "-n", "openshift-marketplace").Execute()

		g.By("create new namespace")
		var err = oc.AsAdmin().Run("create").Args("ns", "test-automation-24738").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create ConfigMap")
		createCfgMap, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", cfgMap, "-p", "NAME=installed-community-24738-global-operators", "NAMESPACE=openshift-marketplace").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("TEST1********** %v", createCfgMap)

		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCfgMap, "-n", "openshift-marketplace").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create CatalogSource")
		createCatSource, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catSource, "-p", "NAME=installed-community-24738-global-operators", "NAMESPACE=openshift-marketplace", "ADDRESS=installed-community-24738-global-operators", "DISPLAYNAME=Community Operators", "PUBLISHER=Community", "SOURCETYPE=internal").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("TEST2oc get ********** %v", createCatSource)

		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCatSource, "-n", "openshift-marketplace").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		createOg, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", og, "-p", "NAME=test-operators-og", "NAMESPACE=test-automation-24738").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createOg).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(60*time.Second, operatorWait, func() (bool, error) {
			checkCatSource, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "installed-community-24738-global-operators", "-n", "openshift-marketplace", "-o", "jsonpath={.status.connectionState.lastObservedState}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			//o.Expect(checkCatSource).To(o.Equal("READY"))
			if checkCatSource == "READY" {
				e2e.Logf("Installed catalogsource")
				return true, nil
			} else {
				e2e.Logf("FAIL - Installed catalogsource ")
				return false, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		createImgSub, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", Sub, "-p", "SUBNAME=etcd-etcdoperator.v0.9.2", "SUBNAMESPACE=test-automation-24738", "CHANNEL=alpha", "APPROVAL=Automatic", "OPERATORNAME=etcd-update", "SOURCENAME=installed-community-24738-global-operators", "SOURCENAMESPACE=openshift-marketplace", "STARTINGCSV=etcdoperator.v0.9.2").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AsAdmin().Run("delete").Args("crd", "etcdclusters.etcd.database.coreos.com").Output()
			oc.AsAdmin().Run("delete").Args("crd", "etcdbackups.etcd.database.coreos.com").Output()
			oc.AsAdmin().Run("delete").Args("crd", "etcdrestores.etcd.database.coreos.com").Output()
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createImgSub).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(60*time.Second, operatorWait, func() (bool, error) {
			checknameCsv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "etcdoperator.v0.9.2", "-n", "test-automation-24738", "-o", "jsonpath={.status.phase}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(checknameCsv)
			if checknameCsv == "Succeeded" {
				e2e.Logf("CSV Installed ")
				return true, nil
			} else {
				e2e.Logf("CSV not installed  ")
				return false, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		createEtcdCluster, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", etcdCluster, "-p", "NAME=example", "NAMESPACE=test-automation-24738").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createEtcdCluster).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(30*time.Second, operatorWait, func() (bool, error) {
			checkCreateEtcdCluster, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "test-automation-24738").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(checkCreateEtcdCluster)

			lines := strings.Split(checkCreateEtcdCluster, "\n")
			var count = 0
			for _, line := range lines {
				e2e.Logf(line)
				if strings.Contains(line, "example") {
					count++
				}
				e2e.Logf(line)
			}
			if count == 3 {
				e2e.Logf("EtcCluster Create Installed ")
				return true, nil
			} else {
				e2e.Logf("EtcCluster Not Installed ")
				return false, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create new namespace")
		err = oc.AsAdmin().Run("create").Args("ns", "test-automation-24738-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		createOg1, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", og, "-p", "NAME=test-operators-og", "NAMESPACE=test-automation-24738-1").OutputToFile("cconfig-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createOg1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		createImgSub1, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", Sub, "-p", "SUBNAME=etcd-etcdoperator.v0.9.2", "SUBNAMESPACE=test-automation-24738-1", "CHANNEL=alpha", "APPROVAL=Automatic", "OPERATORNAME=etcd-update", "SOURCENAME=installed-community-24738-global-operators", "SOURCENAMESPACE=openshift-marketplace", "STARTINGCSV=etcdoperator.v0.9.2").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createImgSub1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		createEtcdCluster1, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", etcdCluster, "-p", "NAME=example", "NAMESPACE=test-automation-24738-1").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createEtcdCluster1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(30*time.Second, operatorWait, func() (bool, error) {
			checkCreateEtcdCluster, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "test-automation-24738-1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(checkCreateEtcdCluster)

			lines := strings.Split(checkCreateEtcdCluster, "\n")
			var count = 0
			for _, line := range lines {
				e2e.Logf(line)
				if strings.Contains(line, "example") {
					count++
				}
				e2e.Logf(line)
			}
			if count == 3 {
				e2e.Logf("EtcCluster Create Installed ")
				return true, nil
			} else {
				e2e.Logf("EtcCluster Not Installed ")
				return false, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("update ConfigMap")
		createPatchCfgMap, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", patchCfgMap, "-p", "NAME=installed-community-24738-global-operators", "NAMESPACE=openshift-marketplace").OutputToFile("config-24738.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("TEST1********** %v", createPatchCfgMap)

		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", createPatchCfgMap, "-n", "openshift-marketplace").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		patchIP, err2 := oc.AsAdmin().WithoutNamespace().Run("patch").Args("sub", "etcd-etcdoperator.v0.9.2", "-n", "test-automation-24738-1", "--type=json", "-p", "[{\"op\": \"replace\" , \"path\" : \"/spec/channel\", \"value\":beta}]").Output()
		e2e.Logf(patchIP)
		o.Expect(err2).NotTo(o.HaveOccurred())
		o.Expect(patchIP).To(o.ContainSubstring("patched"))

		err = wait.Poll(30*time.Second, operatorWait, func() (bool, error) {
			checkCreateEtcdCluster, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ip", "-n", "test-automation-24738-1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(checkCreateEtcdCluster)

			lines := strings.Split(checkCreateEtcdCluster, "\n")
			var count = 0
			for _, line := range lines {
				e2e.Logf(line)
				if strings.Contains(line, "install") {
					count++
				}
				e2e.Logf(line)
			}
			if count == 2 {
				e2e.Logf("Ip Channel created")
				return true, nil
			} else {
				e2e.Logf("Ip Channel NOT created")
				return false, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(60*time.Second, operatorWait, func() (bool, error) {
			checknameCsv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "etcdoperator.v0.9.4", "-n", "test-automation-24738-1", "-o", "jsonpath={.status.phase}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(checknameCsv)
			if checknameCsv == "Succeeded" {
				e2e.Logf("CSV Installed ")
				return true, nil
			} else {
				e2e.Logf("CSV not installed  ")
				return false, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	// author: scolange@redhat.com
	// only community operator ready for the disconnected env now
	g.It("ConnectedOnly-Author:scolange-Medium-32862-Pods found with invalid container images not present in release payload", func() {

		g.By("Verify the version of marketplace_operator")
		pods, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-marketplace", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		lines := strings.Split(pods, "\n")
		for _, line := range lines {
			e2e.Logf("line: %v", line)
			if strings.Contains(line, "certified-operators") || strings.Contains(line, "community-operators") || strings.Contains(line, "marketplace-operator") || strings.Contains(line, "redhat-marketplace") || strings.Contains(line, "redhat-operators") && strings.Contains(line, "1/1") {
				name := strings.Split(line, " ")
				checkRel, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(name[0], "-n", "openshift-marketplace", "--", "cat", "/etc/redhat-release").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(checkRel).To(o.ContainSubstring("Red Hat"))
			}
		}

	})

	// author: scolange@redhat.com

	// author: scolange@redhat.com
	g.It("Author:scolange-Medium-23673-Installplan can be created while Install and uninstall operators via Marketplace for 5 times [Slow]", func() {
		nsName := "test23673"
		var count = 0
		for i := 0; i < 5; i++ {
			count++
			etcdPackage := CreateSubscriptionSpecificNamespace("etcd", oc, true, true, nsName, INSTALLPLAN_AUTOMATIC_MODE)
			defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", nsName).Execute()
			CheckDeployment(etcdPackage, oc)
			RemoveOperatorDependencies(etcdPackage, oc, false)

		}
	})

	// author: scolange@redhat.com
	g.It("Author:scolange-Medium-24586-Prevent Operator Conflicts in OperatorHub", func() {

		var buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		var catSource = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		var Sub = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		var og = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")

		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("catalogsource", "test-prometer-24586", "-n", "openshift-marketplace").Execute()
		defer oc.AsAdmin().Run("delete").Args("ns", "test24586").Execute()

		g.By("create new namespace")
		var err = oc.AsAdmin().Run("create").Args("ns", "test24586").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create new catalogsource")
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catSource, "-p", "NAME=test-prometer-24586",
			"NAMESPACE=openshift-marketplace", "ADDRESS=quay.io/olmqe/mktplc-367", "DISPLAYNAME=prometer-24586", "PUBLISHER=QE", "SOURCETYPE=grpc").OutputToFile("config-24586.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		createOg, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", og, "-p", "NAME=test-operators-og", "NAMESPACE=test24586").OutputToFile("config-24586.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createOg).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		createImgSub, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", Sub, "-p", "SUBNAME=prometheus", "SUBNAMESPACE=test24586",
			"CHANNEL=alpha", "APPROVAL=Automatic", "OPERATORNAME=prometheus", "SOURCENAME=community-operators", "SOURCENAMESPACE=openshift-marketplace").OutputToFile("config-24586.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", createImgSub).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		createImgSub2, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", Sub, "-p", "SUBNAME=prometheus2", "SUBNAMESPACE=test24586",
			"CHANNEL=alpha", "APPROVAL=Automatic", "OPERATORNAME=prometheus", "SOURCENAME=test-prometer-24586", "SOURCENAMESPACE=openshift-marketplace").OutputToFile("config-24586.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", createImgSub2).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	// author: scolange@redhat.com OCP-40316
	g.It("Author:scolange-Medium-40316-OLM enters infinite loop if Pending CSV replaces itself [Serial]", func() {

		var buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		var operatorGroup = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		var pkgServer = filepath.Join(buildPruningBaseDir, "packageserver.yaml")
		//var operatorWait = 180 * time.Second
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test40316").Execute()

		g.By("create new namespace")
		var err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test40316").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create new OperatorGroup")
		ogFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", "NAMESPACE=test40316").OutputToFile("config-40316.json")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", ogFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", pkgServer, "-p", "NAME=packageserver", "NAMESPACE=test40316").OutputToFile("config-40316.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		statusCsv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "test40316").Output()
		e2e.Logf("CSV prometheus %v", statusCsv)
		o.Expect(err).NotTo(o.HaveOccurred())

		pods, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-operator-lifecycle-manager").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(pods)

		lines := strings.Split(pods, "\n")
		for _, line := range lines {
			e2e.Logf("line: %v", line)
			if strings.Contains(line, "olm-operator") {
				name := strings.Split(line, " ")
				checkRel, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "pods", name[0], "-n", "openshift-operator-lifecycle-manager").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				lines1 := strings.Split(checkRel, " ")
				for _, line1 := range lines1 {
					if strings.Contains(line1, "m") {
						e2e.Logf("line1: %v", line1)
						cpu := strings.Split(line1, "m")
						if cpu[0] > "98" {
							e2e.Logf("cpu: %v", cpu[0])
							e2e.Failf("CPU Limit usate more the 99%: %v", checkRel, line1, cpu[0])
						}
					}
				}

			}
		}
	})

	// author: scolange@redhat.com
	g.It("ConnectedOnly-Author:scolange-Medium-24075-The couchbase packagemanifest labels provider value should not be MongoDB Inc ", func() {
		NameCouchBase, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "couchbase-enterprise-certified", "-n", "openshift-marketplace", "-o", "jsonpath={.status.provider.name}").Output()
		e2e.Logf(NameCouchBase)
		o.Expect(err1).NotTo(o.HaveOccurred())
		o.Expect(NameCouchBase).To(o.Equal("Couchbase"))
	})

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Medium-21126-OLM Subscription status says CSV is installed when it is not", func() {
		g.By("1) Install the OperatorGroup in a random project")
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		oc.SetupProject()
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		og := operatorGroupDescription{
			name:      "og-21126",
			namespace: oc.Namespace(),
			template:  ogSingleTemplate,
		}
		og.createwithCheck(oc, itName, dr)

		g.By("2) Install the etcdoperator v0.9.4 with Manual approval")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		sub := subscriptionDescription{
			subName:                "sub-21126",
			namespace:              oc.Namespace(),
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Manual",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.4",
			singleNamespace:        true,
			template:               subTemplate,
		}
		defer sub.delete(itName, dr)
		sub.create(oc, itName, dr)
		g.By("3) Check the etcdoperator v0.9.4 related resources")
		// the installedCSV should be NULL
		newCheck("expect", asAdmin, withoutNamespace, compare, "", ok, []string{"sub", "sub-21126", "-n", oc.Namespace(), "-o=jsonpath={.status.installedCSV}"}).check(oc)
		// the state should be UpgradePending
		newCheck("expect", asAdmin, withoutNamespace, compare, "UpgradePending", ok, []string{"sub", "sub-21126", "-n", oc.Namespace(), "-o=jsonpath={.status.state}"}).check(oc)
		// the InstallPlan should not approved
		newCheck("expect", asAdmin, withoutNamespace, compare, "false", ok, []string{"ip", sub.getIP(oc), "-n", oc.Namespace(), "-o=jsonpath={.spec.approved}"}).check(oc)
		// should no etcdoperator.v0.9.4 CSV found
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "etcdoperator.v0.9.4", "-n", oc.Namespace()).Output()
		if !strings.Contains(msg, "not found") {
			e2e.Failf("still found the etcdoperator.v0.9.4 in namespace:%s, msg:%v", oc.Namespace(), msg)
		}
	})
})

var _ = g.Describe("[sig-operators] OLM for an end user use", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-23440", exutil.KubeConfigPath())
	)

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-Low-24058-components should have resource limits defined", func() {
		olmUnlimited := 0
		olmNames := []string{""}
		olmNamespace := "openshift-operator-lifecycle-manager"
		olmJpath := "-o=jsonpath={range .items[*]}{@.metadata.name}{','}{@.spec.containers[0].resources.requests.*}{'\\n'}"
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", olmNamespace, olmJpath).Output()
		if err != nil {
			e2e.Failf("Unable to get pod -n %v %v.", olmNamespace, olmJpath)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).NotTo(o.ContainSubstring("No resources found"))
		lines := strings.Split(msg, "\n")
		for _, line := range lines {
			name := strings.Split(line, ",")
			// e2e.Logf("Line is %v, len %v, len name %v, name0 %v, name1 %v\n", line, len(line), len(name), name[0], name[1])
			if strings.Contains(line, "packageserver") {
				continue
			} else {
				if len(line) > 1 {
					if len(name) > 1 && len(name[1]) < 1 {
						olmUnlimited++
						olmNames = append(olmNames, name[0])
					}
				}
			}
		}
		if olmUnlimited > 0 && len(olmNames) > 0 {
			e2e.Failf("There are no limits set on %v of %v OLM components: %v", olmUnlimited, len(lines), olmNames)
		}
	})

})

var _ = g.Describe("[sig-operators] OLM for an end user handle common object", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-common-"+getRandomString(), exutil.KubeConfigPath())

		dr = make(describerResrouce)
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.getIr(itName).cleanup()
		dr.rmIr(itName)
	})

	// It will cover test case: OCP-22259, author: kuiwang@redhat.com
	g.It("Author:kuiwang-Medium-22259-marketplace operator CR status on a running cluster [Serial]", func() {

		g.By("check marketplace status")
		newCheck("expect", asAdmin, withoutNamespace, compare, "TrueFalseFalse", ok, []string{"clusteroperator", "marketplace",
			"-o=jsonpath={.status.conditions[?(@.type==\"Available\")].status}{.status.conditions[?(@.type==\"Progressing\")].status}{.status.conditions[?(@.type==\"Degraded\")].status}"}).check(oc)

		g.By("get pod of marketplace")
		podName := getResource(oc, asAdmin, withoutNamespace, "pod", "--selector=name=marketplace-operator", "-n", "openshift-marketplace",
			"-o=jsonpath={...metadata.name}")
		o.Expect(podName).NotTo(o.BeEmpty())

		g.By("delete pod of marketplace")
		_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "pod", podName, "-n", "openshift-marketplace")
		o.Expect(err).NotTo(o.HaveOccurred())

		exec.Command("bash", "-c", "sleep 10").Output()

		g.By("pod of marketplace restart")
		newCheck("expect", asAdmin, withoutNamespace, compare, "TrueFalseFalse", ok, []string{"clusteroperator", "marketplace",
			"-o=jsonpath={.status.conditions[?(@.type==\"Available\")].status}{.status.conditions[?(@.type==\"Progressing\")].status}{.status.conditions[?(@.type==\"Degraded\")].status}"}).check(oc)
	})

	// It will cover test case: OCP-24076, author: kuiwang@redhat.com
	g.It("ProdrunBoth-StagerunBoth-Author:kuiwang-Medium-24076-check the version of olm operator is appropriate in ClusterOperator", func() {
		var (
			olmClusterOperatorName = "operator-lifecycle-manager"
		)

		g.By("get the version of olm operator")
		olmVersion := getResource(oc, asAdmin, withoutNamespace, "clusteroperator", olmClusterOperatorName, "-o=jsonpath={.status.versions[?(@.name==\"operator\")].version}")
		o.Expect(olmVersion).NotTo(o.BeEmpty())

		g.By("Check if it is appropriate in ClusterOperator")
		newCheck("expect", asAdmin, withoutNamespace, compare, olmVersion, ok, []string{"clusteroperator", fmt.Sprintf("-o=jsonpath={.items[?(@.metadata.name==\"%s\")].status.versions[?(@.name==\"operator\")].version}", olmClusterOperatorName)}).check(oc)
	})

	// It will cover test case: OCP-29775 and OCP-29786, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-29775-Medium-29786-as oc user on linux to mirror catalog image", func() {
		var (
			bundleIndex1         = "quay.io/kuiwang/operators-all:v1"
			bundleIndex2         = "quay.io/kuiwang/operators-dockerio:v1"
			operatorAllPath      = "operators-all-manifests-" + getRandomString()
			operatorDockerioPath = "operators-dockerio-manifests-" + getRandomString()
		)
		defer exec.Command("bash", "-c", "rm -fr ./"+operatorAllPath).Output()
		defer exec.Command("bash", "-c", "rm -fr ./"+operatorDockerioPath).Output()

		g.By("mirror to quay.io/kuiwang")
		output, err := oc.AsAdmin().WithoutNamespace().Run("adm", "catalog", "mirror").Args("--manifests-only", "--to-manifests="+operatorAllPath, bundleIndex1, "quay.io/kuiwang").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("operators-all-manifests"))

		g.By("check mapping.txt")
		result, err := exec.Command("bash", "-c", "cat ./"+operatorAllPath+"/mapping.txt|grep -E \"atlasmap-atlasmap-operator:0.1.0|quay.io/kuiwang/jmckind-argocd-operator:[a-z0-9][a-z0-9]|redhat-cop-cert-utils-operator:latest\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("atlasmap-atlasmap-operator:0.1.0"))
		o.Expect(result).To(o.ContainSubstring("redhat-cop-cert-utils-operator:latest"))
		o.Expect(result).To(o.ContainSubstring("quay.io/kuiwang/jmckind-argocd-operator"))

		g.By("check icsp yaml")
		result, err = exec.Command("bash", "-c", "cat ./"+operatorAllPath+"/imageContentSourcePolicy.yaml | grep -E \"quay.io/kuiwang/strimzi-operator|docker.io/strimzi/operator$\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("- quay.io/kuiwang/strimzi-operator"))
		o.Expect(result).To(o.ContainSubstring("source: docker.io/strimzi/operator"))

		g.By("mirror to localhost:5000")
		output, err = oc.AsAdmin().WithoutNamespace().Run("adm", "catalog", "mirror").Args("--manifests-only", "--to-manifests="+operatorDockerioPath, bundleIndex2, "localhost:5000").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("operators-dockerio-manifests"))

		g.By("check mapping.txt to localhost:5000")
		result, err = exec.Command("bash", "-c", "cat ./"+operatorDockerioPath+"/mapping.txt|grep -E \"localhost:5000/atlasmap/atlasmap-operator:0.1.0|localhost:5000/strimzi/operator:[a-z0-9][a-z0-9]\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("localhost:5000/atlasmap/atlasmap-operator:0.1.0"))
		o.Expect(result).To(o.ContainSubstring("localhost:5000/strimzi/operator"))

		g.By("check icsp yaml to localhost:5000")
		result, err = exec.Command("bash", "-c", "cat ./"+operatorDockerioPath+"/imageContentSourcePolicy.yaml | grep -E \"localhost:5000/strimzi/operator|docker.io/strimzi/operator$\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("- localhost:5000/strimzi/operator"))
		o.Expect(result).To(o.ContainSubstring("source: docker.io/strimzi/operator"))
		o.Expect(result).NotTo(o.ContainSubstring("docker.io/atlasmap/atlasmap-operator"))
	})

	// It will cover test case: OCP-33452, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-33452-oc adm catalog mirror does not mirror the index image itself", func() {
		var (
			bundleIndex1 = "quay.io/olmqe/olm-api@sha256:71cfd4deaa493d31cd1d8255b1dce0fb670ae574f4839c778f2cfb1bf1f96995"
			manifestPath = "manifests-olm-api-" + getRandomString()
		)
		defer exec.Command("bash", "-c", "rm -fr ./"+manifestPath).Output()

		g.By("mirror to localhost:5000/test")
		output, err := oc.AsAdmin().WithoutNamespace().Run("adm", "catalog", "mirror").Args("--manifests-only", "--to-manifests="+manifestPath, bundleIndex1, "localhost:5000/test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("manifests-olm-api"))

		g.By("check mapping.txt to localhost:5000")
		result, err := exec.Command("bash", "-c", "cat ./"+manifestPath+"/mapping.txt|grep -E \"quay.io/olmqe/olm-api\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("quay.io/olmqe/olm-api"))

		g.By("check icsp yaml to localhost:5000")
		result, err = exec.Command("bash", "-c", "cat ./"+manifestPath+"/imageContentSourcePolicy.yaml | grep -E \"quay.io/olmqe/olm-api\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("quay.io/olmqe/olm-api"))
	})

	// It will cover test case: OCP-21825, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-21825-Certs for packageserver can be rotated successfully", func() {
		var (
			packageserverName = "packageserver"
		)

		g.By("Get certsRotateAt and APIService name")
		resources := strings.Fields(getResource(oc, asAdmin, withoutNamespace, "csv", packageserverName, "-n", "openshift-operator-lifecycle-manager", fmt.Sprintf("-o=jsonpath={.status.certsRotateAt}{\" \"}{.status.requirementStatus[?(@.kind==\"%s\")].name}", "APIService")))
		o.Expect(resources).NotTo(o.BeEmpty())
		apiServiceName := resources[1]
		certsRotateAt, err := time.Parse(time.RFC3339, resources[0])
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get caBundle")
		caBundle := getResource(oc, asAdmin, withoutNamespace, "apiservices", apiServiceName, "-o=jsonpath={.spec.caBundle}")
		o.Expect(caBundle).NotTo(o.BeEmpty())

		g.By("Change caBundle")
		patchResource(oc, asAdmin, withoutNamespace, "apiservices", apiServiceName, "-p", fmt.Sprintf("{\"spec\":{\"caBundle\":\"test%s\"}}", caBundle))

		g.By("Check updated certsRotataAt")
		err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
			updatedCertsRotateAt, err := time.Parse(time.RFC3339, getResource(oc, asAdmin, withoutNamespace, "csv", packageserverName, "-n", "openshift-operator-lifecycle-manager", "-o=jsonpath={.status.certsRotateAt}"))
			if err != nil {
				e2e.Logf("the get error is %v, and try next", err)
				return false, nil
			}
			if !updatedCertsRotateAt.After(certsRotateAt) {
				e2e.Logf("wait update, and try next")
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		newCheck("expect", asAdmin, withoutNamespace, contain, "redhat-operators", ok, []string{"packagemanifest", fmt.Sprintf("--selector=catalog=%s", "redhat-operators"), "-o=jsonpath={.items[*].status.catalogSource}"}).check(oc)

	})

})

var _ = g.Describe("[sig-operators] OLM for an end user handle within a namespace", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-a-"+getRandomString(), exutil.KubeConfigPath())
		dr = make(describerResrouce)
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {})

	// It will cover test case: OCP-29231 and OCP-29277, author: kuiwang@redhat.com
	g.It("Author:kuiwang-Medium-29231-Medium-29277-label to target namespace of group", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			og1                 = operatorGroupDescription{
				name:      "og1-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			og2 = operatorGroupDescription{
				name:      "og2-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og1.namespace = oc.Namespace()
		og2.namespace = oc.Namespace()

		g.By("Create og1 and check the label of target namespace of og1 is created")
		og1.create(oc, itName, dr)
		og1Uid := getResource(oc, asAdmin, withNamespace, "og", og1.name, "-o=jsonpath={.metadata.uid}")
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+og1Uid, ok,
			[]string{"ns", og1.namespace, "-o=jsonpath={.metadata.labels}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+og1Uid, nok,
			[]string{"ns", "openshift-operators", "-o=jsonpath={.metadata.labels}"}).check(oc)

		g.By("Delete og1 and check the label of target namespace of og1 is removed")
		og1.delete(itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+og1Uid, nok,
			[]string{"ns", og1.namespace, "-o=jsonpath={.metadata.labels}"}).check(oc)

		g.By("Create og2 and recreate og1 and check the label")
		og2.create(oc, itName, dr)
		og2Uid := getResource(oc, asAdmin, withNamespace, "og", og2.name, "-o=jsonpath={.metadata.uid}")
		og1.create(oc, itName, dr)
		og1Uid = getResource(oc, asAdmin, withNamespace, "og", og1.name, "-o=jsonpath={.metadata.uid}")
		labelNs := getResource(oc, asAdmin, withoutNamespace, "ns", og1.namespace, "-o=jsonpath={.metadata.labels}")
		o.Expect(labelNs).To(o.ContainSubstring(og2Uid))
		o.Expect(labelNs).To(o.ContainSubstring(og1Uid))

		//OCP-29277
		g.By("Check no label of global operator group ")
		globalOgUID := getResource(oc, asAdmin, withoutNamespace, "og", "global-operators", "-n", "openshift-operators", "-o=jsonpath={.metadata.uid}")
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+globalOgUID, nok,
			[]string{"ns", "default", "-o=jsonpath={.metadata.labels}"}).check(oc)

	})

	// It will cover test case: OCP-23170, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-23170-API labels should be hash", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			ogD                 = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			subD = subscriptionDescription{
				subName:                "hawtio-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "hawtio-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			og  = ogD
			sub = subD
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create operator")
		sub.create(oc, itName, dr)

		g.By("Check the API labes should be hash")
		apiLabels := getResource(oc, asUser, withNamespace, "csv", sub.installedCSV, "-o=jsonpath={.metadata.labels}")
		o.Expect(len(apiLabels)).NotTo(o.BeZero())

		for _, v := range strings.Split(strings.Trim(apiLabels, "{}"), ",") {
			if strings.Contains(v, "olm.api") {
				hash := strings.Trim(strings.Split(strings.Split(v, ":")[0], ".")[2], "\"")
				match, err := regexp.MatchString(`^[a-fA-F0-9]{16}$|^[a-fA-F0-9]{15}$`, hash)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(match).To(o.BeTrue())
			}
		}
	})

	// It will cover test case: OCP-20979, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-20979-only one IP is generated", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			ogD                 = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			subD = subscriptionDescription{
				subName:                "hawtio-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "hawtio-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			og  = ogD
			sub = subD
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create operator")
		sub.create(oc, itName, dr)
		newCheck("expect", asUser, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Check there is only one ip")
		ips := getResource(oc, asAdmin, withoutNamespace, "ip", "-n", sub.namespace, "--no-headers")
		ipList := strings.Split(ips, "\n")
		for _, ip := range ipList {
			name := strings.Fields(ip)[0]
			getResource(oc, asAdmin, withoutNamespace, "ip", name, "-n", sub.namespace, "-o=json")
		}
		o.Expect(strings.Count(ips, sub.installedCSV)).To(o.Equal(1))
	})

	// It will cover test case: OCP-25757 and 22656, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-25757-High-22656-manual approval strategy apply to subsequent releases", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			ogD                 = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			subD = subscriptionDescription{
				subName:                "hawtio-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "hawtio-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			og  = ogD
			sub = subD
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("prepare for manual approval")
		sub.ipApproval = "Manual"
		sub.startingCSV = "hawtio-operator.v0.1.0"

		g.By("Create Sub which apply manual approve install plan")
		sub.create(oc, itName, dr)

		g.By("the install plan is RequiresApproval")
		installPlan := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}")
		o.Expect(installPlan).NotTo(o.BeEmpty())
		newCheck("expect", asAdmin, withoutNamespace, compare, "RequiresApproval", ok, []string{"ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("manually approve sub")
		sub.approve(oc, itName, dr)

		g.By("the target CSV is created with upgrade")
		o.Expect(strings.Compare(sub.installedCSV, sub.startingCSV) != 0).To(o.BeTrue())
	})

	// It will cover test case: OCP-24438, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-24438-check subscription CatalogSource Status", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			ogD                 = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			subD = subscriptionDescription{
				subName:                "hawtio-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "hawtio-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}

			catsrc = catalogSourceDescription{
				name:        "catsrc-test-operator",
				namespace:   "",
				displayName: "Test Catsrc Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "",
				template:    catsrcImageTemplate,
			}
			og  = ogD
			sub = subD
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()

		catsrc.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("create sub with the above catalogsource")
		sub.createWithoutCheck(oc, itName, dr)

		g.By("check its condition is UnhealthyCatalogSourceFound")
		newCheck("expect", asUser, withoutNamespace, contain, "UnhealthyCatalogSourceFound", ok, []string{"sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.conditions[*].reason}"}).check(oc)

		g.By("create catalogsource")
		imageAddress := getResource(oc, asAdmin, withoutNamespace, "catsrc", "community-operators", "-n", "openshift-marketplace", "-o=jsonpath={.spec.image}")
		o.Expect(imageAddress).NotTo(o.BeEmpty())
		catsrc.address = imageAddress
		catsrc.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", catsrc.name, "-n", catsrc.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("check its condition is AllCatalogSourcesHealthy and csv is created")
		newCheck("expect", asUser, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, []string{"sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.conditions[*].reason}"}).check(oc)
		sub.findInstalledCSV(oc, itName, dr)
	})

	// It will cover test case: OCP-24027, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-24027-can create and delete catalogsource and sub repeatedly", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			ogD                 = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			subD = subscriptionDescription{
				subName:                "hawtio-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "hawtio-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-test-operator",
				namespace:   "",
				displayName: "Test Catsrc Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "",
				template:    catsrcImageTemplate,
			}
			repeatedCount = 2
			og            = ogD
			sub           = subD
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()

		catsrc.namespace = oc.Namespace()
		sub.catalogSourceName = catsrc.name
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Get address of catalogsource and name")
		imageAddress := getResource(oc, asAdmin, withoutNamespace, "catsrc", "community-operators", "-n", "openshift-marketplace", "-o=jsonpath={.spec.image}")
		o.Expect(imageAddress).NotTo(o.BeEmpty())
		catsrc.address = imageAddress

		for i := 0; i < repeatedCount; i++ {
			g.By("Create Catalogsource")
			catsrc.create(oc, itName, dr)
			newCheck("expect", asUser, withoutNamespace, compare, "READY", ok, []string{"catsrc", catsrc.name, "-n", catsrc.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

			g.By("Create sub with the above catalogsource")
			sub.create(oc, itName, dr)
			newCheck("expect", asUser, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Remove catalog and sub")
			sub.delete(itName, dr)
			sub.deleteCSV(itName, dr)
			catsrc.delete(itName, dr)
			if i < repeatedCount-1 {
				time.Sleep(20 * time.Second)
			}
		}
	})

	// It will cover part of test case: OCP-21404, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-21404-csv will be RequirementsNotMet after sa is delete", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			ogD                 = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			subD = subscriptionDescription{
				subName:                "hawtio-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "hawtio-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			og  = ogD
			sub = subD
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create operator")
		sub.create(oc, itName, dr)
		newCheck("expect", asUser, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Get SA of csv")
		sa := newSa(getResource(oc, asUser, withNamespace, "csv", sub.installedCSV, "-o=jsonpath={.status.requirementStatus[?(@.kind==\"ServiceAccount\")].name}"), sub.namespace)

		g.By("Delete sa of csv")
		sa.getDefinition(oc)
		sa.delete(oc)
		newCheck("expect", asUser, withNamespace, compare, "RequirementsNotMet", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.reason}"}).check(oc)

		g.By("Recovery sa of csv")
		sa.reapply(oc)
		newCheck("expect", asUser, withNamespace, compare, "Succeeded+2+Installing", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)
	})

	// It will cover part of test case: OCP-21404, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-21404-csv will be RequirementsNotMet after role rule is delete", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			ogD                 = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			subD = subscriptionDescription{
				subName:                "hawtio-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "hawtio-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			og  = ogD
			sub = subD
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create operator")
		sub.create(oc, itName, dr)
		newCheck("expect", asUser, withNamespace, compare, "Succeeded"+"InstallSucceeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}{.status.reason}"}).check(oc)

		g.By("Get SA of csv")
		sa := newSa(getResource(oc, asUser, withNamespace, "csv", sub.installedCSV, "-o=jsonpath={.status.requirementStatus[?(@.kind==\"ServiceAccount\")].name}"), sub.namespace)
		sa.checkAuth(oc, "yes", "Hawtio")

		g.By("Get Role of csv")
		role := newRole(getResource(oc, asUser, withNamespace, "role", "-n", sub.namespace, fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-o=jsonpath={.items[0].metadata.name}"), sub.namespace)
		origRules := role.getRules(oc)
		modifiedRules := role.getRulesWithDelete(oc, "hawt.io")

		g.By("Remove rules")
		role.patch(oc, fmt.Sprintf("{\"rules\": %s}", modifiedRules))
		sa.checkAuth(oc, "no", "Hawtio")

		g.By("Recovery rules")
		role.patch(oc, fmt.Sprintf("{\"rules\": %s}", origRules))
		sa.checkAuth(oc, "yes", "Hawtio")
	})

	// It will cover test case: OCP-29723, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-29723-As cluster admin find abnormal status condition via components of operator resource", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-29723-operator",
				namespace:   "",
				displayName: "Test Catsrc 29723 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-api:v1",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "cockroachdb",
				namespace:              "",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "cockroachdb",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "cockroachdb.v2.1.11",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("delete catalog source")
		catsrc.delete(itName, dr)
		g.By("delete sa")
		_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "sa", "cockroachdb-operator", "-n", sub.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check abnormal status")
		output := getResource(oc, asAdmin, withoutNamespace, "operator", sub.operatorPackage+"."+sub.namespace, "-o=json")
		o.Expect(output).NotTo(o.BeEmpty())

		output = getResource(oc, asAdmin, withoutNamespace, "operator", sub.operatorPackage+"."+sub.namespace,
			fmt.Sprintf("-o=jsonpath={.status.components.refs[?(@.name==\"%s\")].conditions[*].type}", sub.subName))
		o.Expect(output).To(o.ContainSubstring("CatalogSourcesUnhealthy"))

		newCheck("expect", asAdmin, withoutNamespace, contain, "RequirementsNotMet+2+InstallWaiting", ok, []string{"operator", sub.operatorPackage + "." + sub.namespace,
			fmt.Sprintf("-o=jsonpath={.status.components.refs[?(@.name==\"%s\")].conditions[*].reason}", sub.installedCSV)}).check(oc)
	})

	// It will cover test case: OCP-30762, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-30762-installs bundles with v1 CRDs", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-30762-operator",
				namespace:   "",
				displayName: "Test Catsrc 30762 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-api:v1",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "cockroachdb",
				namespace:              "",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "cockroachdb",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "cockroachdb.v2.1.11",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
	})

	// It will cover test case: OCP-27683, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-27683-InstallPlans can install from extracted bundles", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-27683-operator",
				namespace:   "",
				displayName: "Test Catsrc 27683 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/single-bundle-index:1.0.0",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "kiali",
				namespace:              "",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "kiali",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "kiali-operator.v1.4.2",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("get bundle package from ip")
		installPlan := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}")
		o.Expect(installPlan).NotTo(o.BeEmpty())
		ipBundle := getResource(oc, asAdmin, withoutNamespace, "ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.bundleLookups[0].path}")
		o.Expect(ipBundle).NotTo(o.BeEmpty())

		g.By("get bundle package from job")
		jobName := getResource(oc, asAdmin, withoutNamespace, "job", "-n", catsrc.namespace, "-o=jsonpath={.items[0].metadata.name}")
		o.Expect(jobName).NotTo(o.BeEmpty())
		jobBundle := getResource(oc, asAdmin, withoutNamespace, "pod", "-l", "job-name="+jobName, "-n", catsrc.namespace, "-o=jsonpath={.items[0].status.initContainerStatuses[*].image}")
		o.Expect(jobName).NotTo(o.BeEmpty())
		o.Expect(jobBundle).To(o.ContainSubstring(ipBundle))
	})

	// It will cover test case: OCP-24513, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-24513-Operator config support env only", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-24513-operator",
				namespace:   "",
				displayName: "Test Catsrc 24513 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-dep:venv",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "teiid",
				namespace:              "",
				channel:                "beta",
				ipApproval:             "Automatic",
				operatorPackage:        "teiid",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "teiid.v0.4.0",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			opename = "teiid-operator"
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("get parameter of deployment")
		newCheck("expect", asAdmin, withoutNamespace, contain, "ARGS1", ok, []string{"deployment", opename, "-n", sub.namespace, "-o=jsonpath={.spec.template.spec.containers[0].command}"}).check(oc)

		g.By("patch env for sub")
		sub.patch(oc, "{\"spec\": {\"config\": {\"env\": [{\"name\": \"EMPTY_ENV\"},{\"name\": \"ARGS1\",\"value\": \"-v=4\"}]}}}")

		g.By("check the empty env")
		newCheck("expect", asAdmin, withoutNamespace, contain, "EMPTY_ENV", ok, []string{"deployment", opename, "-n", sub.namespace, "-o=jsonpath={.spec.template.spec.containers[0].env[*].name}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "-v=4", ok, []string{"deployment", opename, "-n", sub.namespace, "-o=jsonpath={.spec.template.spec.containers[0].env[*].value}"}).check(oc)
	})

	// It will cover test case: OCP-24382, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-24382-Should restrict CRD update if schema changes [Serial]", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			etcdCluster         = filepath.Join(buildPruningBaseDir, "etcd-cluster.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-24382-operator",
				namespace:   "",
				displayName: "Test Catsrc 24382 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-dep:vschema",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "etcd",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "etcd",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "etcdoperator.v0.9.2",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			etcdCr = customResourceDescription{
				name:      "example-24382",
				namespace: "",
				typename:  "EtcdCluster",
				template:  etcdCluster,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace
		etcdCr.namespace = oc.Namespace()

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("creat cr")
		etcdCr.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{etcdCr.typename, etcdCr.name, "-n", etcdCr.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("update operator")
		sub.patch(oc, "{\"spec\": {\"channel\": \"beta\"}}")
		sub.findInstalledCSV(oc, itName, dr)

		g.By("check schema does not work")
		installPlan := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}")
		o.Expect(installPlan).NotTo(o.BeEmpty())
		newCheck("expect", asAdmin, withoutNamespace, contain, "error validating existing CRs", ok, []string{"ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.conditions[*].message}"}).check(oc)
	})

	// It will cover test case: OCP-25760, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-25760-Operator upgrades does not fail after change the channel", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-25760-operator",
				namespace:   "",
				displayName: "Test Catsrc 25760 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-dep:vchannel",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "teiid",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "teiid",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "teiid.v0.3.0",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("switch channel")
		sub.patch(oc, "{\"spec\": {\"channel\": \"beta\"}}")
		sub.findInstalledCSV(oc, itName, dr)

		g.By("check csv of new channel")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
	})

	// It will cover test case: OCP-35895, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-35895-can't install a CSV with duplicate roles", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-35895-operator",
				namespace:   "",
				displayName: "Test Catsrc 35895 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-dep:vargo",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "argocd-operator",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "argocd-operator",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "argocd-operator.v0.0.11",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("check sa")
		newCheck("expect", asAdmin, withoutNamespace, contain, "argocd-redis-ha-haproxy", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={..serviceAccountName}"}).check(oc)
	})

	// It will cover test case: OCP-32863, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-32863-Support resources required for SAP Gardener Control Plane Operator", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			vpaTemplate         = filepath.Join(buildPruningBaseDir, "vpa-crd.yaml")
			crdVpa              = crdDescription{
				name:     "verticalpodautoscalers.autoscaling.k8s.io",
				template: vpaTemplate,
			}
			og = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-32863-operator",
				namespace:   "",
				displayName: "Test Catsrc 32863 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/single-bundle-index:pdb",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "busybox",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "busybox",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "busybox.v2.0.0",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
		)

		// defer crdVpa.delete(oc) //it is not needed in case it already exist

		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create vpa crd")
		crdVpa.create(oc, itName, dr)

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check csv")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("check additional resources")
		newCheck("present", asAdmin, withoutNamespace, present, "", ok, []string{"vpa", "busybox-vpa", "-n", sub.namespace}).check(oc)
		newCheck("present", asAdmin, withoutNamespace, present, "", ok, []string{"PriorityClass", "super-priority", "-n", sub.namespace}).check(oc)
		newCheck("present", asAdmin, withoutNamespace, present, "", ok, []string{"PodDisruptionBudget", "busybox-pdb", "-n", sub.namespace}).check(oc)
	})

	// It will cover test case: OCP-34472, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-34472-OLM label dependency", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "olm-1933-v8-catalog",
				namespace:   "",
				displayName: "OLM 1933 v8 Operator Catalog",
				publisher:   "QE",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-dep:v8",
				template:    catsrcImageTemplate,
			}
			sub = subscriptionDescription{
				subName:                "teiid",
				namespace:              "",
				channel:                "beta",
				ipApproval:             "Automatic",
				operatorPackage:        "teiid",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "teiid.v0.4.0",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			dependentOperator = "microcks-operator.v1.0.0"
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install perator")
		sub.create(oc, itName, dr)

		g.By("check if dependent operator is installed")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", dependentOperator, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
	})

	// It will cover test case: OCP-37263, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-37263-Subscription stays in UpgradePending but InstallPlan not installing [Slow]", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "olm-1860185-catalog",
				namespace:   "",
				displayName: "OLM 1860185 Catalog",
				publisher:   "QE",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-dep:v1860185",
				template:    catsrcImageTemplate,
			}
			subStrimzi = subscriptionDescription{
				subName:                "strimzi",
				namespace:              "",
				channel:                "strimzi-0.19.x",
				ipApproval:             "Automatic",
				operatorPackage:        "strimzi-kafka-operator",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "strimzi-cluster-operator.v0.19.0",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			subCouchbase = subscriptionDescription{
				subName:                "couchbase",
				namespace:              "",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "couchbase-enterprise-certified",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "couchbase-operator.v1.2.2",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			subPortworx = subscriptionDescription{
				subName:                "portworx",
				namespace:              "",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "portworx-certified",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "portworx-operator.v1.4.2",
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
		)
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		subStrimzi.namespace = oc.Namespace()
		subStrimzi.catalogSourceNamespace = catsrc.namespace
		subCouchbase.namespace = oc.Namespace()
		subCouchbase.catalogSourceNamespace = catsrc.namespace
		subPortworx.namespace = oc.Namespace()
		subPortworx.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install Strimzi")
		subStrimzi.create(oc, itName, dr)

		g.By("check if Strimzi is installed")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subStrimzi.installedCSV, "-n", subStrimzi.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("install Portworx")
		subPortworx.create(oc, itName, dr)

		g.By("check if Portworx is installed")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subPortworx.installedCSV, "-n", subPortworx.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Delete Portworx sub")
		subPortworx.delete(itName, dr)

		g.By("check if Portworx sub is Deleted")
		newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"sub", subPortworx.subName, "-n", subPortworx.namespace}).check(oc)

		g.By("Delete Portworx csv")
		csvPortworx := csvDescription{
			name:      subPortworx.installedCSV,
			namespace: subPortworx.namespace,
		}
		csvPortworx.delete(itName, dr)

		g.By("check if Portworx csv is Deleted")
		newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"csv", subPortworx.installedCSV, "-n", subPortworx.namespace}).check(oc)

		g.By("install Couchbase")
		subCouchbase.create(oc, itName, dr)

		g.By("check if Couchbase is installed")
		err := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			csvPhase := getResource(oc, asAdmin, withoutNamespace, "csv", subCouchbase.installedCSV, "-n", subCouchbase.namespace, "-o=jsonpath={.status.phase}")
			if strings.Contains(csvPhase, "Succeeded") {
				e2e.Logf("Couchbase is installed")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// It will cover test case: OCP-33176, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-33176-Enable generated operator component adoption for operators with single ns mode [Slow]", func() {
		var (
			itName                  = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir     = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate        = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate             = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			catsrcImageTemplate     = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			apiserviceImageTemplate = filepath.Join(buildPruningBaseDir, "apiservice.yaml")
			apiserviceVersion       = "v33176"
			apiserviceName          = apiserviceVersion + ".foos.bar.com"
			og                      = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-33176-operator",
				namespace:   "",
				displayName: "Test Catsrc 33176 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-api:v1",
				template:    catsrcImageTemplate,
			}
			subEtcd = subscriptionDescription{
				subName:                "etcd33176",
				namespace:              "",
				channel:                "singlenamespace-alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "etcd",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "etcdoperator.v0.9.4", //get it from package based on currentCSV if ipApproval is Automatic
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        false,
			}
			subCockroachdb = subscriptionDescription{
				subName:                "cockroachdb33176",
				namespace:              "",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "cockroachdb",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "cockroachdb.v2.1.11", //get it from package based on currentCSV if ipApproval is Automatic
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        false,
			}
		)

		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		subEtcd.namespace = oc.Namespace()
		subEtcd.catalogSourceNamespace = catsrc.namespace
		subCockroachdb.namespace = oc.Namespace()
		subCockroachdb.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install Etcd")
		subEtcd.create(oc, itName, dr)
		defer doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subEtcd.operatorPackage+"."+subEtcd.namespace)

		g.By("Check all resources via operators")
		newCheck("expect", asAdmin, withoutNamespace, contain, "ServiceAccount", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "Role", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "RoleBinding", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "Subscription", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "InstallPlan", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ClusterServiceVersion", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "Deployment", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, subEtcd.namespace, ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].namespace}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "InstallSucceeded", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].conditions[*].reason}"}).check(oc)

		g.By("delete operator and Operator still exists because of crd")
		subEtcd.delete(itName, dr)
		_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "csv", subEtcd.installedCSV, "-n", subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		g.By("reinstall etcd and check Operator")
		subEtcd.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "InstallSucceeded", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].conditions[*].reason}"}).check(oc)

		g.By("delete etcd and the Operator")
		_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "sub", subEtcd.subName, "-n", subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "csv", subEtcd.installedCSV, "-n", subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subEtcd.operatorPackage+"."+subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("install etcd manually")
		subEtcd.ipApproval = "Manual"
		subEtcd.startingCSV = "etcdoperator.v0.9.4"
		subEtcd.installedCSV = ""
		subEtcd.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "InstallPlan", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		g.By("approve etcd")
		subEtcd.approve(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ClusterServiceVersion", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, subEtcd.namespace, ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].namespace}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "InstallSucceeded", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].conditions[*].reason}"}).check(oc)

		g.By("unlabel resource and it is relabeled automatically")
		roleName := getResource(oc, asAdmin, withoutNamespace, "operator", subEtcd.operatorPackage+"."+subEtcd.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='Role')].name}")
		o.Expect(roleName).NotTo(o.BeEmpty())
		_, err = doAction(oc, "label", asAdmin, withoutNamespace, "Role", roleName, "operators.coreos.com/"+subEtcd.operatorPackage+"."+subEtcd.namespace+"-", "-n", subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", asAdmin, withoutNamespace, contain, "Role", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		g.By("delete etcd and the Operator again and Operator should recreated because of crd")
		_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "sub", subEtcd.subName, "-n", subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "csv", subEtcd.installedCSV, "-n", subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subEtcd.operatorPackage+"."+subEtcd.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		// here there is issue and take WA
		_, err = doAction(oc, "label", asAdmin, withoutNamespace, "crd", "etcdbackups.etcd.database.coreos.com", "operators.coreos.com/"+subEtcd.operatorPackage+"."+subEtcd.namespace+"-")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = doAction(oc, "label", asAdmin, withoutNamespace, "crd", "etcdbackups.etcd.database.coreos.com", "operators.coreos.com/"+subEtcd.operatorPackage+"."+subEtcd.namespace+"=")
		o.Expect(err).NotTo(o.HaveOccurred())
		//done for WA
		var componentKind string
		err = wait.Poll(15*time.Second, 240*time.Second, func() (bool, error) {
			componentKind = getResource(oc, asAdmin, withoutNamespace, "operator", subEtcd.operatorPackage+"."+subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}")
			if strings.Contains(componentKind, "CustomResourceDefinition") {
				return true, nil
			}
			e2e.Logf("the got kind is %v", componentKind)
			return false, nil
		})
		if err != nil && strings.Compare(componentKind, "") != 0 {
			e2e.Failf("the operator has wrong component")
			// after the official is supported, will change it again.
		}

		g.By("install Cockroachdb")
		subCockroachdb.create(oc, itName, dr)
		defer doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subCockroachdb.operatorPackage+"."+subCockroachdb.namespace)

		g.By("Check all resources of Cockroachdb via operators")
		newCheck("expect", asAdmin, withoutNamespace, contain, "ServiceAccount", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "Role", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "RoleBinding", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "Subscription", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "InstallPlan", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "ClusterServiceVersion", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "Deployment", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, subCockroachdb.namespace, ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].namespace}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "InstallSucceeded", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].conditions[*].reason}"}).check(oc)

		g.By("create ns test-33176 and label it")
		_, err = doAction(oc, "create", asAdmin, withoutNamespace, "ns", "test-33176")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer doAction(oc, "delete", asAdmin, withoutNamespace, "ns", "test-33176")
		_, err = doAction(oc, "label", asAdmin, withoutNamespace, "ns", "test-33176", "operators.coreos.com/"+subCockroachdb.operatorPackage+"."+subCockroachdb.namespace+"=")
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", asAdmin, withoutNamespace, contain, "Namespace", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		g.By("create apiservice and label it")
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", apiserviceImageTemplate, "-p", "NAME="+apiserviceName, "VERSION="+apiserviceVersion)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer doAction(oc, "delete", asAdmin, withoutNamespace, "apiservice", apiserviceName)
		_, err = doAction(oc, "label", asAdmin, withoutNamespace, "apiservice", apiserviceName, "operators.coreos.com/"+subCockroachdb.operatorPackage+"."+subCockroachdb.namespace+"=")
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", asAdmin, withoutNamespace, contain, "APIService", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

	})

	// It will cover test case: OCP-39897, author: kuiwang@redhat.com
	//Set it as serial because it will delete CRD of teiid. It potential impact other cases if it is in parallel.
	g.It("ConnectedOnly-Author:kuiwang-Medium-39897-operator objects should not be recreated after all other associated resources have been deleted [Serial]", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogSingleTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        "catsrc-39897-operator",
				namespace:   "",
				displayName: "Test Catsrc 39897 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-dep:vteiid-1899588",
				template:    catsrcImageTemplate,
			}
			subTeiid = subscriptionDescription{
				subName:                "teiid39897",
				namespace:              "",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "teiid",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: "",
				startingCSV:            "teiid.v0.3.0", //get it from package based on currentCSV if ipApproval is Automatic
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        false,
			}
			crd = crdDescription{
				name: "virtualdatabases.teiid.io",
			}
		)

		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		og.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		subTeiid.namespace = oc.Namespace()
		subTeiid.catalogSourceNamespace = catsrc.namespace

		g.By("create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("install Teiid")
		subTeiid.create(oc, itName, dr)
		defer doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subTeiid.operatorPackage+"."+subTeiid.namespace)

		g.By("Check the resources via operators")
		newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subTeiid.operatorPackage + "." + subTeiid.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		g.By("delete operator and Operator still exists because of crd")
		subTeiid.delete(itName, dr)
		_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "csv", subTeiid.installedCSV, "-n", subTeiid.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subTeiid.operatorPackage + "." + subTeiid.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		g.By("delete crd")
		crd.delete(oc)

		g.By("delete Operator resource to check if it is recreated")
		doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subTeiid.operatorPackage+"."+subTeiid.namespace)
		newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"operator", subTeiid.operatorPackage + "." + subTeiid.namespace}).check(oc)
	})

	// It will cover test case: OCP-24917, author: tbuskey@redhat.com
	g.It("Author:tbuskey-Medium-24917-Operators in SingleNamespace should not be granted namespace list [Disruptive]", func() {
		g.By("1) Install the OperatorGroup in a random project")
		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		oc.SetupProject()
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		og := operatorGroupDescription{
			name:      "og-24917",
			namespace: oc.Namespace(),
			template:  ogSingleTemplate,
		}
		og.createwithCheck(oc, itName, dr)

		g.By("2) Install the etcdoperator v0.9.4 with Automatic approval")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		sub := subscriptionDescription{
			subName:                "sub-24917",
			namespace:              oc.Namespace(),
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "singlenamespace-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.4",
			singleNamespace:        true,
			template:               subTemplate,
		}
		defer sub.delete(itName, dr)
		defer sub.deleteCSV(itName, dr)
		sub.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("3) check if this operator's SA can list all namespaces")
		expectedSA := fmt.Sprintf("system:serviceaccount:%s:etcd-operator", oc.Namespace())
		msg, err := oc.AsAdmin().WithoutNamespace().Run("policy").Args("who-can", "list", "namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(msg, expectedSA)).To(o.BeFalse())

		g.By("4) get the token of this operator's SA")
		token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "etcd-operator", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("5) get the cluster server")
		server, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("--show-server").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("6) get the current context")
		context, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("--show-context").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// make sure switch to the current cluster-admin role after finished
		defer func() {
			g.By("9) Switch to the cluster-admin role")
			_, err := oc.AsAdmin().WithoutNamespace().Run("config").Args("use-context", context).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("7) login the cluster with this token")
		_, err = oc.AsAdmin().WithoutNamespace().Run("login").Args(server, "--token", token).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		whoami, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(whoami, expectedSA)).To(o.BeTrue())

		g.By("8) this SA user should NOT have the permission to list all namespaces")
		ns, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns").Output()
		o.Expect(strings.Contains(ns, "namespaces is forbidden")).To(o.BeTrue())
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-Medium-25782-CatalogSource Status should have information on last observed state", func() {
		var err error
		var (
			catName             = "installed-community-25782-global-operators"
			msg                 = ""
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			// the namespace and catName are hardcoded in the files
			cmTemplate       = filepath.Join(buildPruningBaseDir, "cm-csv-etcd.yaml")
			catsrcCmTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-configmap.yaml")
		)

		oc.SetupProject()
		itName := g.CurrentGinkgoTestDescription().TestText

		var (
			cm = configMapDescription{
				name:      catName,
				namespace: oc.Namespace(),
				template:  cmTemplate,
			}
			catsrc = catalogSourceDescription{
				name:        catName,
				namespace:   oc.Namespace(),
				displayName: "Community bad Operators",
				publisher:   "QE",
				sourceType:  "configmap",
				address:     catName,
				template:    catsrcCmTemplate,
			}
		)

		g.By("Create ConfigMap with bad operator manifest")
		cm.create(oc, itName, dr)

		// Make sure bad configmap was created
		g.By("Check configmap")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("cm", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(msg, catName)).To(o.BeTrue())

		g.By("Create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Wait for pod to fail")
		waitErr := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace()).Output()
			e2e.Logf("\n%v", msg)
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "CrashLoopBackOff") {
				e2e.Logf("STEP pod is in  CrashLoopBackOff as expected")
				return true, nil
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred())

		g.By("Check catsrc state for TRANSIENT_FAILURE in lastObservedState")
		waitErr = wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", catName, "-n", oc.Namespace(), "-o=jsonpath={.status}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "TRANSIENT_FAILURE") && strings.Contains(msg, "lastObservedState") {
				msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", catName, "-n", oc.Namespace(), "-o=jsonpath={.status.connectionState.lastObservedState}").Output()
				e2e.Logf("catalogsource had lastObservedState =  %v as expected ", msg)
				return true, nil
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred())
		e2e.Logf("cleaning up")
	})

	// It will cover test case: OCP-25644, author: tbuskey@redhat.com
	g.It("Author:tbuskey-Medium-25644-OLM collect CSV health per version", func() {
		var err error
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogTemplate          = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			ogAllTemplate       = filepath.Join(buildPruningBaseDir, "og-allns.yaml")
			subFile             = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			csvName             = "etcdoperator.v0.9.4"
			next                = false
			ogName              = "test-25644-group"
		)

		oc.SetupProject()

		og := operatorGroupDescription{
			name:      ogName,
			namespace: oc.Namespace(),
			template:  ogTemplate,
		}
		ogAll := operatorGroupDescription{
			name:      ogName,
			namespace: oc.Namespace(),
			template:  ogAllTemplate,
		}

		sub := subscriptionDescription{
			subName:                "sub-25644",
			namespace:              oc.Namespace(),
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			ipApproval:             "Automatic",
			template:               subFile,
			channel:                "singlenamespace-alpha",
			operatorPackage:        "etcd",
			startingCSV:            "etcdoperator.v0.9.4",
			singleNamespace:        true,
		}

		g.By("Create cluster-scoped OperatorGroup")
		ogAll.create(oc, itName, dr)
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "-n", oc.Namespace()).Output()
		e2e.Logf("og: %v, %v", msg, og.name)

		g.By("Subscribe to etcd operator and wait for the csv to fail")
		// CSV should fail && show fail.  oc describe csv xyz will have error
		defer sub.delete(itName, dr)
		defer sub.deleteCSV(itName, dr)
		sub.createWithoutCheck(oc, itName, dr)
		// find the CSV so that it can be delete after finished
		sub.findInstalledCSV(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Failed", ok, []string{"csv", csvName, "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace(), csvName, "-o=jsonpath={.status.conditions..reason}").Output()
		e2e.Logf("--> get the csv reason: %v ", msg)
		o.Expect(strings.Contains(msg, "UnsupportedOperatorGroup") || strings.Contains(msg, "NoOperatorGroup")).To(o.BeTrue())

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace(), csvName, "-o=jsonpath={.status.conditions..message}").Output()
		e2e.Logf("--> get the csv message: %v\n", msg)
		o.Expect(strings.Contains(msg, "InstallModeType not supported") || strings.Contains(msg, "csv in namespace with no operatorgroup")).To(o.BeTrue())

		g.By("Get prometheus token")
		olmToken, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(olmToken).NotTo(o.BeEmpty())

		g.By("get OLM pod name")
		olmPodname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-operator-lifecycle-manager", "--selector=app=olm-operator", "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(olmPodname).NotTo(o.BeEmpty())

		g.By("check metrics")
		metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(olmPodname, "-n", "openshift-operator-lifecycle-manager", "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://localhost:8081/metrics").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metrics).NotTo(o.BeEmpty())

		var metricsVal, metricsVar string
		for _, s := range strings.Fields(metrics) {
			if next {
				metricsVal = s
				break
			}
			if strings.Contains(s, "csv_abnormal{") && strings.Contains(s, csvName) && strings.Contains(s, oc.Namespace()) {
				metricsVar = s
				next = true
			}
		}
		e2e.Logf("\nMetrics\n    %v == %v\n", metricsVar, metricsVal)
		o.Expect(metricsVal).NotTo(o.BeEmpty())

		g.By("reset og to single namespace")
		og.delete(itName, dr)
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "-n", oc.Namespace()).Output()
		e2e.Logf("og deleted:%v", msg)

		og.create(oc, itName, dr)
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "-n", oc.Namespace(), "--no-headers").Output()
		e2e.Logf("og created:%v", msg)

		g.By("Wait for csv to recreate and ready")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "etcdoperator.v0.9.4", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace(), csvName, "-o=jsonpath={.status.reason}").Output()
		e2e.Logf("--> get the csv reason: %v ", msg)
		o.Expect(strings.Contains(msg, "InstallSucceeded")).To(o.BeTrue())
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace(), csvName, "-o=jsonpath={.status.message}").Output()
		e2e.Logf("--> get the csv message: %v\n", msg)
		o.Expect(strings.Contains(msg, "completed with no errors")).To(o.BeTrue())

		g.By("Make sure pods are fully running")
		waitErr := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace()).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "etcd-operator") && strings.Contains(msg, "Running") && strings.Contains(msg, "3/3") {
				return true, nil
			}
			return false, nil
		})
		e2e.Logf("\nPods\n%v", msg)
		o.Expect(waitErr).NotTo(o.HaveOccurred())

		g.By("check new metrics")
		next = false
		metricsVar = ""
		metricsVal = ""
		metrics, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args(olmPodname, "-n", "openshift-operator-lifecycle-manager", "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://localhost:8081/metrics").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metrics).NotTo(o.BeEmpty())
		for _, s := range strings.Fields(metrics) {
			if next {
				metricsVal = s
				break
			}
			if strings.Contains(s, "csv_succeeded{") && strings.Contains(s, csvName) && strings.Contains(s, oc.Namespace()) {
				metricsVar = s
				next = true
			}
		}
		e2e.Logf("\nMetrics\n%v ==  %v\n", metricsVar, metricsVal)
		o.Expect(metricsVar).NotTo(o.BeEmpty())
		o.Expect(metricsVal).NotTo(o.BeEmpty())

		g.By("SUCCESS")

	})

	// Test case: OCP-24566, author:xzha@redhat.com
	g.It("ConnectedOnly-Author:xzha-Medium-24566-OLM automatically configures operators with global proxy config", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		subTemplateProxy := filepath.Join(buildPruningBaseDir, "olm-proxy-subscription.yaml")
		oc.SetupProject()
		var (
			og = operatorGroupDescription{
				name:      "test-og",
				namespace: oc.Namespace(),
				template:  ogSingleTemplate,
			}
			sub = subscriptionDescription{
				subName:                "planetscale-sub",
				namespace:              oc.Namespace(),
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				channel:                "beta",
				ipApproval:             "Automatic",
				operatorPackage:        "planetscale",
				singleNamespace:        true,
				template:               subTemplate,
			}
			subP = subscriptionDescription{subName: "planetscale-sub",
				namespace:              oc.Namespace(),
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				channel:                "beta",
				ipApproval:             "Automatic",
				operatorPackage:        "planetscale",
				singleNamespace:        true,
				template:               subTemplateProxy}
			subProxyTest = subscriptionDescriptionProxy{
				subscriptionDescription: subP,
				httpProxy:               "test_http_proxy",
				httpsProxy:              "test_https_proxy",
				noProxy:                 "test_no_proxy",
			}
			subProxyFake = subscriptionDescriptionProxy{
				subscriptionDescription: subP,
				httpProxy:               "fake_http_proxy",
				httpsProxy:              "fake_https_proxy",
				noProxy:                 "fake_no_proxy",
			}
			subProxyEmpty = subscriptionDescriptionProxy{
				subscriptionDescription: subP,
				httpProxy:               "",
				httpsProxy:              "",
				noProxy:                 "",
			}
		)
		itName := g.CurrentGinkgoTestDescription().TestText

		//oc get proxy cluster
		g.By(fmt.Sprintf("0) check the cluster is proxied"))
		httpProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpProxy}")
		httpsProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpsProxy}")
		noProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.noProxy}")
		g.By(fmt.Sprintf("1) create the OperatorGroup in project: %s", oc.Namespace()))
		og.createwithCheck(oc, itName, dr)

		if httpProxy == "" {
			g.By("2) install operator and check the proxy is empty")
			sub.create(oc, itName, dr)
			g.By("install operator SUCCESS")
			nodeHTTPProxy := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTP_PROXY\")].value}")
			o.Expect(nodeHTTPProxy).To(o.BeEmpty())
			nodeHTTPSProxy := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTPS_PROXY\")].value}")
			o.Expect(nodeHTTPSProxy).To(o.BeEmpty())
			nodeNoProxy := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"NO_PROXY\")].value}")
			o.Expect(nodeNoProxy).To(o.BeEmpty())
			g.By("CHECK proxy configure SUCCESS")
			sub.delete(itName, dr)
			sub.deleteCSV(itName, dr)
		} else {
			o.Expect(httpProxy).NotTo(o.BeEmpty())
			o.Expect(httpsProxy).NotTo(o.BeEmpty())
			o.Expect(noProxy).NotTo(o.BeEmpty())

			g.By("2) install operator and check the proxy")
			sub.create(oc, itName, dr)
			g.By("install operator SUCCESS")
			nodeHTTPProxy := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTP_PROXY\")].value}")
			o.Expect(nodeHTTPProxy).To(o.Equal(httpProxy))
			nodeHTTPSProxy := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTPS_PROXY\")].value}")
			o.Expect(nodeHTTPSProxy).To(o.Equal(httpsProxy))
			nodeNoProxy := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"NO_PROXY\")].value}")
			o.Expect(nodeNoProxy).To(o.Equal(noProxy))
			g.By("CHECK proxy configure SUCCESS")
			sub.delete(itName, dr)
			sub.deleteCSV(itName, dr)

			g.By("3) create subscription and set variables ( HTTP_PROXY, HTTPS_PROXY and NO_PROXY ) with non-empty values. ")
			subProxyTest.create(oc, itName, dr)
			nodeHTTPProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTP_PROXY\")].value}")
			o.Expect(nodeHTTPProxy).To(o.Equal("test_http_proxy"))
			nodeHTTPSProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTPS_PROXY\")].value}")
			o.Expect(nodeHTTPSProxy).To(o.Equal("test_https_proxy"))
			nodeNoProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"NO_PROXY\")].value}")
			o.Expect(nodeNoProxy).To(o.Equal("test_no_proxy"))
			subProxyTest.delete(itName, dr)
			subProxyTest.getCSV().delete(itName, dr)

			g.By("4) Create a new subscription and set variables ( HTTP_PROXY, HTTPS_PROXY and NO_PROXY ) with a fake value.")
			subProxyFake.create(oc, itName, dr)
			nodeHTTPProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTP_PROXY\")].value}")
			o.Expect(nodeHTTPProxy).To(o.Equal("fake_http_proxy"))
			nodeHTTPSProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTPS_PROXY\")].value}")
			o.Expect(nodeHTTPSProxy).To(o.Equal("fake_https_proxy"))
			nodeNoProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"NO_PROXY\")].value}")
			o.Expect(nodeNoProxy).To(o.Equal("fake_no_proxy"))
			subProxyFake.delete(itName, dr)
			subProxyFake.getCSV().delete(itName, dr)

			g.By("5) Create a new subscription and set variables ( HTTP_PROXY, HTTPS_PROXY and NO_PROXY ) with an empty value.")
			subProxyEmpty.create(oc, itName, dr)
			nodeHTTPProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=marketplace.operatorSource=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={.spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTP_PROXY\")].value}")
			o.Expect(nodeHTTPProxy).To(o.BeEmpty())
			nodeHTTPSProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=marketplace.operatorSource=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={.spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"HTTPS_PROXY\")].value}")
			o.Expect(nodeHTTPSProxy).To(o.BeEmpty())
			nodeNoProxy = getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=marketplace.operatorSource=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={.spec.template.spec.containers[?(.name==\"planetscale-operator\")].env[?(.name==\"NO_PROXY\")].value}")
			o.Expect(nodeNoProxy).To(o.BeEmpty())
			subProxyEmpty.delete(itName, dr)
			subProxyEmpty.getCSV().delete(itName, dr)
		}
	})

	// author: tbuskey@redhat.com, test case OCP-21080
	g.It("Author:tbuskey-High-21080-OLM Check OLM metrics", func() {

		type metrics struct {
			csv_count               int
			csv_upgrade_count       int
			catalog_source_count    int
			install_plan_count      int
			subscription_count      int
			subscription_sync_total int
		}

		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogTemplate          = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subFile             = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			catPodname          string
			data                PrometheusQueryResult
			err                 error
			exists              bool
			i                   int
			metricsBefore       metrics
			metricsAfter        metrics
			msg                 string
			olmPodname          string
			olmToken            string
			subSync             PrometheusQueryResult
		)

		oc.SetupProject()

		var (
			og = operatorGroupDescription{
				name:      "test-21080-group",
				namespace: oc.Namespace(),
				template:  ogTemplate,
			}
			sub = subscriptionDescription{
				subName:                "etcd-21080",
				namespace:              oc.Namespace(),
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				ipApproval:             "Automatic",
				channel:                "singlenamespace-alpha",
				operatorPackage:        "etcd",
				singleNamespace:        true,
				template:               subFile,
			}
		)

		g.By("check for operator")
		e2e.Logf("Check if %v exists in the %v catalog", sub.operatorPackage, sub.catalogSourceName)
		exists, err = clusterPackageExists(oc, sub)
		if !exists {
			e2e.Failf("FAIL:PackageMissing %v does not exist in catalog %v", sub.operatorPackage, sub.catalogSourceName)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(exists).To(o.BeTrue())

		g.By("Get token & pods")
		og.create(oc, itName, dr)
		olmToken, err = oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(olmToken).NotTo(o.BeEmpty())

		olmPodname, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-operator-lifecycle-manager", "--selector=app=olm-operator", "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(olmPodname).NotTo(o.BeEmpty())

		catPodname, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-operator-lifecycle-manager", "--selector=app=catalog-operator", "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(catPodname).NotTo(o.BeEmpty())

		g.By("collect olm metrics before")
		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", olmPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=csv_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsBefore.csv_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", olmPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=csv_upgrade_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsBefore.csv_upgrade_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=catalog_source_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsBefore.catalog_source_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=install_plan_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsBefore.install_plan_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=subscription_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsBefore.subscription_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		metricsBefore.subscription_sync_total = 0

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=subscription_sync_total").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &subSync)
		for i = range subSync.Data.Result {
			if strings.Contains(subSync.Data.Result[i].Metric.SrcName, sub.subName) {
				metricsBefore.subscription_sync_total, err = strconv.Atoi(subSync.Data.Result[i].Value[1].(string))
			}
		}

		e2e.Logf("\nbefore {csv_count, csv_upgrade_count, catalog_source_count, install_plan_count, subscription_count, subscription_sync_total}\n%v", metricsBefore)

		g.By("Subscribe")
		sub.create(oc, itName, dr) // check kept timing out
		newCheck("expect", asAdmin, withoutNamespace, compare, "AtLatestKnown", ok, []string{"sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)

		g.By("Collect olm metrics after")
		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", olmPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=csv_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsAfter.csv_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", olmPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=csv_upgrade_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsAfter.csv_upgrade_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=catalog_source_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsAfter.catalog_source_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=install_plan_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsAfter.install_plan_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=subscription_count").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &data)
		metricsAfter.subscription_count, err = strconv.Atoi(data.Data.Result[0].Value[1].(string))

		metricsAfter.subscription_sync_total = 0
		msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-operator-lifecycle-manager", catPodname, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=subscription_sync_total").Outputs()
		o.Expect(msg).NotTo(o.BeEmpty())
		json.Unmarshal([]byte(msg), &subSync)
		for i = range subSync.Data.Result {
			if strings.Contains(subSync.Data.Result[i].Metric.SrcName, sub.subName) {
				metricsAfter.subscription_sync_total, err = strconv.Atoi(subSync.Data.Result[i].Value[1].(string))
			}
		}

		g.By("Results")
		e2e.Logf("{csv_count csv_upgrade_count catalog_source_count install_plan_count subscription_count subscription_sync_total}")
		e2e.Logf("%v", metricsBefore)
		e2e.Logf("%v", metricsAfter)

		g.By("Check Results")
		// csv_count can increase or decrease
		// o.Expect(metricsBefore.csv_count <= metricsAfter.csv_count).To(o.BeTrue())
		// e2e.Logf("PASS csv_count is equal or greater")

		o.Expect(metricsBefore.csv_upgrade_count <= metricsAfter.csv_upgrade_count).To(o.BeTrue())
		e2e.Logf("PASS csv_upgrade_count is equal or greater")

		o.Expect(metricsBefore.catalog_source_count <= metricsAfter.catalog_source_count).To(o.BeTrue())
		e2e.Logf("PASS catalog_source_count is equal or greater")

		o.Expect(metricsBefore.install_plan_count <= metricsAfter.install_plan_count).To(o.BeTrue())
		e2e.Logf("PASS install_plan_count is equal or greater")

		o.Expect(metricsBefore.subscription_count <= metricsAfter.subscription_count).To(o.BeTrue())
		e2e.Logf("PASS subscription_count is equal or greater")

		o.Expect(metricsBefore.subscription_sync_total <= metricsAfter.subscription_sync_total).To(o.BeTrue())
		e2e.Logf("PASS subscription_sync_total is equal or greater")
		e2e.Logf("All PASS\n")

		g.By("DONE")

	})

	// author: xzha@redhat.com, test case OCP-40529
	g.It("ConnectedOnly-Author:xzha-Medium-40529-OPERATOR_CONDITION_NAME should have correct value", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		oc.SetupProject()
		namespaceName := oc.Namespace()
		var (
			og = operatorGroupDescription{
				name:      "test-og",
				namespace: namespaceName,
				template:  ogSingleTemplate,
			}
			sub = subscriptionDescription{
				subName:                "ditto-40529-operator",
				namespace:              namespaceName,
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				channel:                "alpha",
				ipApproval:             "Manual",
				operatorPackage:        "ditto-operator",
				singleNamespace:        true,
				template:               subTemplate,
				startingCSV:            "ditto-operator.v0.1.1",
			}
		)
		itName := g.CurrentGinkgoTestDescription().TestText
		g.By("STEP 1: create the OperatorGroup ")
		og.createwithCheck(oc, itName, dr)

		g.By("STEP 2: create sub")
		defer sub.delete(itName, dr)
		defer sub.deleteCSV(itName, dr)
		sub.create(oc, itName, dr)
		e2e.Logf("approve the install plan")
		sub.approveSpecificIP(oc, itName, dr, "ditto-operator.v0.1.1", "Complete")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "ditto-operator.v0.1.1", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("STEP 3: check OPERATOR_CONDITION_NAME")
		newCheck("expect", asAdmin, withoutNamespace, compare, "ditto-operator.v0.1.1", ok, []string{"deployment", "ditto-operator", "-n", namespaceName, "-o=jsonpath={.spec.template.spec.containers[*].env[?(@.name==\"OPERATOR_CONDITION_NAME\")].value}"}).check(oc)

		g.By("STEP 4: approve the install plan")
		sub.approveSpecificIP(oc, itName, dr, "ditto-operator.v0.2.0", "Complete")
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", "ditto-operator.v0.2.0", "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("STEP 5: check OPERATOR_CONDITION_NAME")
		newCheck("expect", asAdmin, withoutNamespace, compare, "ditto-operator.v0.2.0", ok, []string{"deployment", "ditto-operator", "-n", namespaceName, "-o=jsonpath={.spec.template.spec.containers[*].env[?(@.name==\"OPERATOR_CONDITION_NAME\")].value}"}).check(oc)
	})

	// author: xzha@redhat.com, test case OCP-40534
	g.It("ConnectedOnly-Author:xzha-Medium-40534-the deployment should not lost the resources section", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		oc.SetupProject()
		namespaceName := oc.Namespace()
		var (
			og = operatorGroupDescription{
				name:      "test-og",
				namespace: namespaceName,
				template:  ogSingleTemplate,
			}
			sub = subscriptionDescription{
				subName:                "tidb-40534-operator",
				namespace:              namespaceName,
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "tidb-operator",
				singleNamespace:        true,
				template:               subTemplate,
			}
		)
		itName := g.CurrentGinkgoTestDescription().TestText
		g.By("STEP 1: create the OperatorGroup ")
		og.createwithCheck(oc, itName, dr)

		g.By("STEP 2: create sub")
		defer sub.delete(itName, dr)
		defer sub.deleteCSV(itName, dr)
		sub.create(oc, itName, dr)

		g.By("STEP 3: check OPERATOR_CONDITION_NAME")
		cpuCSV := getResource(oc, asAdmin, withoutNamespace, "csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={..containers[?(@.name==\"tidb-operator\")].resources.requests.cpu}")
		o.Expect(cpuCSV).NotTo(o.BeEmpty())
		memoryCSV := getResource(oc, asAdmin, withoutNamespace, "csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={..containers[?(@.name==\"tidb-operator\")].resources.requests.memory}")
		o.Expect(memoryCSV).NotTo(o.BeEmpty())
		cpuDeployment := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..containers[?(@.name==\"tidb-operator\")].resources.requests.cpu}")
		o.Expect(cpuDeployment).To(o.Equal(cpuDeployment))
		memoryDeployment := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..containers[?(@.name==\"tidb-operator\")].resources.requests.memory}")
		o.Expect(memoryDeployment).To(o.Equal(memoryCSV))

	})

})

var _ = g.Describe("[sig-operators] OLM for an end user handle to support", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-cm-"+getRandomString(), exutil.KubeConfigPath())
		dr = make(describerResrouce)
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {})

	// It will cover part of test case: OCP-29275, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-29275-label to target namespace of operator group with multi namespace", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogMultiTemplate     = filepath.Join(buildPruningBaseDir, "og-multins.yaml")
			og                  = operatorGroupDescription{
				name:         "og-1651-1",
				namespace:    "",
				multinslabel: "test-og-label-1651",
				template:     ogMultiTemplate,
			}
			p1 = projectDescription{
				name:            "test-ns1651-1",
				targetNamespace: "",
			}
			p2 = projectDescription{
				name:            "test-ns1651-2",
				targetNamespace: "",
			}
		)

		defer p1.delete(oc)
		defer p2.delete(oc)
		//oc.TeardownProject()
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		p1.targetNamespace = oc.Namespace()
		p2.targetNamespace = oc.Namespace()
		og.namespace = oc.Namespace()
		g.By("Create new projects and label them")
		p1.create(oc, itName, dr)
		p1.label(oc, "test-og-label-1651")
		p2.create(oc, itName, dr)
		p2.label(oc, "test-og-label-1651")

		g.By("Create og and check the label")
		og.create(oc, itName, dr)
		ogUID := getResource(oc, asAdmin, withNamespace, "og", og.name, "-o=jsonpath={.metadata.uid}")
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+ogUID, ok,
			[]string{"ns", p1.name, "-o=jsonpath={.metadata.labels}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+ogUID, ok,
			[]string{"ns", p2.name, "-o=jsonpath={.metadata.labels}"}).check(oc)

		g.By("delete og and check there is no label")
		og.delete(itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+ogUID, nok,
			[]string{"ns", p1.name, "-o=jsonpath={.metadata.labels}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+ogUID, nok,
			[]string{"ns", p2.name, "-o=jsonpath={.metadata.labels}"}).check(oc)

		g.By("create another og to check the label")
		og.name = "og-1651-2"
		og.create(oc, itName, dr)
		ogUID = getResource(oc, asAdmin, withNamespace, "og", og.name, "-o=jsonpath={.metadata.uid}")
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+ogUID, ok,
			[]string{"ns", p1.name, "-o=jsonpath={.metadata.labels}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "olm.operatorgroup.uid/"+ogUID, ok,
			[]string{"ns", p2.name, "-o=jsonpath={.metadata.labels}"}).check(oc)
	})

	// It will cover test case: OCP-22200, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-22200-add minimum kube version to CSV [Slow]", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogTemplate          = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			cmNcTemplate        = filepath.Join(buildPruningBaseDir, "cm-namespaceconfig.yaml")
			catsrcCmTemplate    = filepath.Join(buildPruningBaseDir, "catalogsource-configmap.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
			}
			cmNc = configMapDescription{
				name:      "cm-community-namespaceconfig-operators",
				namespace: "", //must be set in iT
				template:  cmNcTemplate,
			}
			catsrcNc = catalogSourceDescription{
				name:        "catsrc-community-namespaceconfig-operators",
				namespace:   "", //must be set in iT
				displayName: "Community namespaceconfig Operators",
				publisher:   "Community",
				sourceType:  "configmap",
				address:     "cm-community-namespaceconfig-operators",
				template:    catsrcCmTemplate,
			}
			subNc = subscriptionDescription{
				subName:                "namespace-configuration-operator",
				namespace:              "", //must be set in iT
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "namespace-configuration-operator",
				catalogSourceName:      "catsrc-community-namespaceconfig-operators",
				catalogSourceNamespace: "", //must be set in iT
				startingCSV:            "",
				currentCSV:             "namespace-configuration-operator.v0.1.0", //it matches to that in cm, so set it.
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			cm     = cmNc
			catsrc = catsrcNc
			sub    = subNc
		)

		//oc.TeardownProject()
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		cm.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace
		og.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create configmap of csv")
		cm.create(oc, itName, dr)

		g.By("Get minKubeVersionRequired and kubeVersionUpdated")
		output := getResource(oc, asUser, withoutNamespace, "cm", cm.name, "-o=json")
		csvDesc := strings.TrimSuffix(strings.TrimSpace(strings.SplitN(strings.SplitN(output, "\"clusterServiceVersions\": ", 2)[1], "\"customResourceDefinitions\":", 2)[0]), ",")
		o.Expect(strings.Contains(csvDesc, "minKubeVersion:")).To(o.BeTrue())
		minKubeVersionRequired := strings.TrimSpace(strings.SplitN(strings.SplitN(csvDesc, "minKubeVersion:", 2)[1], "\\n", 2)[0])
		kubeVersionUpdated := generateUpdatedKubernatesVersion(oc)
		e2e.Logf("the kubeVersionUpdated version is %s, and minKubeVersionRequired is %s", kubeVersionUpdated, minKubeVersionRequired)

		g.By("Create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Update the minKubeVersion greater than the cluster KubeVersion")
		cm.patch(oc, fmt.Sprintf("{\"data\": {\"clusterServiceVersions\": %s}}", strings.ReplaceAll(csvDesc, "minKubeVersion: "+minKubeVersionRequired, "minKubeVersion: "+kubeVersionUpdated)))

		g.By("Create sub with greater KubeVersion")
		sub.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, contain, "CSV version requirement not met", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.requirementStatus[?(@.kind==\"ClusterServiceVersion\")].message}"}).check(oc)

		g.By("Remove sub and csv and update the minKubeVersion to orignl")
		sub.delete(itName, dr)
		sub.deleteCSV(itName, dr)
		cm.patch(oc, fmt.Sprintf("{\"data\": {\"clusterServiceVersions\": %s}}", csvDesc))

		g.By("Create sub with orignal KubeVersion")
		sub.create(oc, itName, dr)
		err := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			csvPhase := getResource(oc, asAdmin, withoutNamespace, "csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}")
			if strings.Contains(csvPhase, "Succeeded") {
				e2e.Logf("sub is installed")
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			msg := getResource(oc, asAdmin, withoutNamespace, "csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.requirementStatus[?(@.kind==\"ClusterServiceVersion\")].message}")
			if strings.Contains(msg, "CSV version requirement not met") && !strings.Contains(msg, kubeVersionUpdated) {
				e2e.Failf("the csv can not be installed with correct kube version")
			}
		}
	})

	// It will cover test case: OCP-23473, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-23473-permit z-stream releases skipping during operator updates", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogTemplate          = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			cmNcTemplate        = filepath.Join(buildPruningBaseDir, "cm-namespaceconfig.yaml")
			catsrcCmTemplate    = filepath.Join(buildPruningBaseDir, "catalogsource-configmap.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
			}
			skippedVersion = "namespace-configuration-operator.v0.0.2"
			cmNc           = configMapDescription{
				name:      "cm-community-namespaceconfig-operators",
				namespace: "", //must be set in iT
				template:  cmNcTemplate,
			}
			catsrcNc = catalogSourceDescription{
				name:        "catsrc-community-namespaceconfig-operators",
				namespace:   "", //must be set in iT
				displayName: "Community namespaceconfig Operators",
				publisher:   "Community",
				sourceType:  "configmap",
				address:     "cm-community-namespaceconfig-operators",
				template:    catsrcCmTemplate,
			}
			subNc = subscriptionDescription{
				subName:                "namespace-configuration-operator",
				namespace:              "", //must be set in iT
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "namespace-configuration-operator",
				catalogSourceName:      "catsrc-community-namespaceconfig-operators",
				catalogSourceNamespace: "", //must be set in iT
				startingCSV:            "",
				currentCSV:             "namespace-configuration-operator.v0.1.0", //it matches to that in cm, so set it.
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			cm     = cmNc
			catsrc = catsrcNc
			sub    = subNc
		)

		//oc.TeardownProject()
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		cm.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace
		og.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create configmap of csv")
		cm.create(oc, itName, dr)

		g.By("Create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create sub")
		sub.ipApproval = "Manual"
		sub.startingCSV = "namespace-configuration-operator.v0.0.1"
		sub.create(oc, itName, dr)

		g.By("manually approve sub")
		sub.approve(oc, itName, dr)

		g.By(fmt.Sprintf("there is skipped csv version %s", skippedVersion))
		o.Expect(strings.Contains(sub.ipCsv, skippedVersion)).To(o.BeFalse())
	})

	// It will cover test case: OCP-24664, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-24664-CRD updates if new schemas are backwards compatible", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogTemplate          = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			cmLearnV1Template   = filepath.Join(buildPruningBaseDir, "cm-learn-v1.yaml")
			cmLearnV2Template   = filepath.Join(buildPruningBaseDir, "cm-learn-v2.yaml")
			catsrcCmTemplate    = filepath.Join(buildPruningBaseDir, "catalogsource-configmap.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			og                  = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
			}
			cmLearn = configMapDescription{
				name:      "cm-learn-operators",
				namespace: "", //must be set in iT
				template:  cmLearnV1Template,
			}
			catsrcLearn = catalogSourceDescription{
				name:        "catsrc-learn-operators",
				namespace:   "", //must be set in iT
				displayName: "Learn Operators",
				publisher:   "Community",
				sourceType:  "configmap",
				address:     "cm-learn-operators",
				template:    catsrcCmTemplate,
			}
			subLearn = subscriptionDescription{
				subName:                "learn-operator",
				namespace:              "", //must be set in iT
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "learn-operator",
				catalogSourceName:      "catsrc-learn-operators",
				catalogSourceNamespace: "", //must be set in iT
				startingCSV:            "",
				currentCSV:             "learn-operator.v0.0.1", //it matches to that in cm, so set it.
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			cm     = cmLearn
			catsrc = catsrcLearn
			sub    = subLearn
			crd    = crdDescription{
				name: "learns.app.learn.com",
			}
		)

		defer crd.delete(oc)

		//oc.TeardownProject()
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		cm.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace
		og.namespace = oc.Namespace()

		g.By("ensure no such crd")
		crd.delete(oc)

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create configmap of csv")
		cm.create(oc, itName, dr)

		g.By("Create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create sub")
		sub.create(oc, itName, dr)
		newCheck("expect", asUser, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
		newCheck("expect", asAdmin, withoutNamespace, contain, "v2", nok, []string{"crd", crd.name, "-A", "-o=jsonpath={.status.storedVersions}"}).check(oc)

		g.By("update the cm to update csv definition")
		cm.template = cmLearnV2Template
		cm.create(oc, itName, dr)

		g.By("update channel of Sub")
		sub.patch(oc, "{\"spec\": {\"channel\": \"beta\"}}")
		sub.expectCSV(oc, itName, dr, "learn-operator.v0.0.2")
		newCheck("expect", asAdmin, withoutNamespace, contain, "v2", ok, []string{"crd", crd.name, "-A", "-o=jsonpath={.status.storedVersions}"}).check(oc)
	})

	// It will cover test case: OCP-21824, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-21824-verify CRD should be ready before installing the operator", func() {
		var (
			itName               = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir  = exutil.FixturePath("testdata", "olm")
			ogTemplate           = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			cmReadyTestTemplate  = filepath.Join(buildPruningBaseDir, "cm-certutil-readytest.yaml")
			catsrcCmTemplate     = filepath.Join(buildPruningBaseDir, "catalogsource-configmap.yaml")
			subTemplate          = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			crdOlmtestTemplate   = filepath.Join(buildPruningBaseDir, "crd-olmtest.yaml")
			cmReadyTestsTemplate = filepath.Join(buildPruningBaseDir, "cm-certutil-readytests.yaml")
			og                   = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
			}
			cmCertUtilReadytest = configMapDescription{
				name:      "cm-certutil-readytest-operators",
				namespace: "", //must be set in iT
				template:  cmReadyTestTemplate,
			}
			catsrcCertUtilReadytest = catalogSourceDescription{
				name:        "catsrc-certutil-readytest-operators",
				namespace:   "", //must be set in iT
				displayName: "certutil readytest Operators",
				publisher:   "Community",
				sourceType:  "configmap",
				address:     "cm-certutil-readytest-operators",
				template:    catsrcCmTemplate,
			}
			subCertUtilReadytest = subscriptionDescription{
				subName:                "cert-utils-operator",
				namespace:              "", //must be set in iT
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "cert-utils-operator",
				catalogSourceName:      "catsrc-certutil-readytest-operators",
				catalogSourceNamespace: "", //must be set in iT
				startingCSV:            "",
				currentCSV:             "cert-utils-operator.v0.0.3", //it matches to that in cm, so set it.
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        true,
			}
			cm           = cmCertUtilReadytest
			catsrc       = catsrcCertUtilReadytest
			sub          = subCertUtilReadytest
			crdReadytest = crdDescription{
				name:     "readytest.stable.example.com",
				template: crdOlmtestTemplate,
			}
			crdReadytests = crdDescription{
				name:     "readytests.stable.example.com",
				template: crdOlmtestTemplate,
			}
		)

		defer crdReadytest.delete(oc)
		defer crdReadytests.delete(oc)

		//oc.TeardownProject()
		oc.SetupProject() //project and its resource are deleted automatically when out of It, so no need derfer or AfterEach
		cm.namespace = oc.Namespace()
		catsrc.namespace = oc.Namespace()
		sub.namespace = oc.Namespace()
		sub.catalogSourceNamespace = catsrc.namespace
		og.namespace = oc.Namespace()

		g.By("Create og")
		og.create(oc, itName, dr)

		g.By("Create cm with wrong crd")
		cm.create(oc, itName, dr)

		g.By("Create catalog source")
		catsrc.create(oc, itName, dr)

		g.By("Create sub and canot succeed")
		sub.createWithoutCheck(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "", ok, []string{"sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)

		g.By("update cm to correct crd")
		cm.template = cmReadyTestsTemplate
		cm.create(oc, itName, dr)

		g.By("sub succeed and csv succeed")
		sub.findInstalledCSV(oc, itName, dr)
		newCheck("expect", asUser, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
	})

})

var _ = g.Describe("[sig-operators] OLM for an end user handle within all namespace", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-all-"+getRandomString(), exutil.KubeConfigPath())
		dr = make(describerResrouce)
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.getIr(itName).cleanup()
		dr.rmIr(itName)
	})

	// It will cover test case: OCP-21484, OCP-21532(acutally it covers OCP-21484), author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-21484-High-21532-watch special or all namespace by operator group", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			sub                 = subscriptionDescription{
				subName:                "composable-operator",
				namespace:              "openshift-operators",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "composable-operator",
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				// startingCSV:            "composable-operator.v0.1.3",
				startingCSV:     "", //get it from package based on currentCSV if ipApproval is Automatic
				currentCSV:      "",
				installedCSV:    "",
				template:        subTemplate,
				singleNamespace: false,
			}

			project = projectDescription{
				name:            "olm-enduser-specific-21484",
				targetNamespace: oc.Namespace(),
			}
			cl = checkList{}
		)

		// OCP-21532
		g.By("Check the global operator global-operators support all namesapces")
		cl.add(newCheck("expect", asAdmin, withoutNamespace, compare, "[]", ok, []string{"og", "global-operators", "-n", "openshift-operators", "-o=jsonpath={.status.namespaces}"}))

		// OCP-21484, OCP-21532
		g.By("Create operator targeted at all namespace")
		sub.create(oc, itName, dr)

		g.By("Create new namespace")
		project.create(oc, itName, dr)

		// OCP-21532
		g.By("New annotations is added to copied CSV in current namespace")
		cl.add(newCheck("expect", asUser, withNamespace, contain, "alm-examples", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.metadata.annotations}"}))

		// OCP-21484, OCP-21532
		g.By("Check the csv within new namespace is copied. note: the step is slow because it wait to copy csv to new namespace")
		cl.add(newCheck("expect", asAdmin, withoutNamespace, compare, "Copied", ok, []string{"csv", sub.installedCSV, "-n", project.name, "-o=jsonpath={.status.reason}"}))

		cl.check(oc)

	})

	// It will cover test case: OCP-24906, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-24906-Operators requesting cluster-scoped permission can trigger kube GC bug", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			sub                 = subscriptionDescription{
				subName:                "amq-streams",
				namespace:              "openshift-operators",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "amq-streams",
				catalogSourceName:      "redhat-operators",
				catalogSourceNamespace: "openshift-marketplace",
				// startingCSV:            "amqstreams.v1.3.0",
				startingCSV:     "", //get it from package based on currentCSV if ipApproval is Automatic
				currentCSV:      "",
				installedCSV:    "",
				template:        subTemplate,
				singleNamespace: false,
			}
			cl = checkList{}
		)

		g.By("Create operator targeted at all namespace")
		sub.create(oc, itName, dr)

		g.By("Check clusterrolebinding has no OwnerReferences")
		cl.add(newCheck("expect", asAdmin, withoutNamespace, compare, "", ok, []string{"clusterrolebinding", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..OwnerReferences}"}))

		g.By("Check clusterrole has no OwnerReferences")
		cl.add(newCheck("expect", asAdmin, withoutNamespace, compare, "", ok, []string{"clusterrole", fmt.Sprintf("--selector=olm.owner=%s", sub.installedCSV), "-n", sub.namespace, "-o=jsonpath={..OwnerReferences}"}))
		//do check parallelly
		cl.check(oc)
	})

	// It will cover test case: OCP-33241, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Author:kuiwang-Medium-33241-Enable generated operator component adoption for operators with all ns mode [Serial]", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			catsrc              = catalogSourceDescription{
				name:        "catsrc-33241-operator",
				namespace:   "openshift-marketplace",
				displayName: "Test Catsrc 33241 Operators",
				publisher:   "Red Hat",
				sourceType:  "grpc",
				address:     "quay.io/olmqe/olm-api:v1",
				template:    catsrcImageTemplate,
			}
			subCockroachdb = subscriptionDescription{
				subName:                "cockroachdb33241",
				namespace:              "openshift-operators",
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "cockroachdb",
				catalogSourceName:      catsrc.name,
				catalogSourceNamespace: catsrc.namespace,
				startingCSV:            "", //get it from package based on currentCSV if ipApproval is Automatic
				currentCSV:             "",
				installedCSV:           "",
				template:               subTemplate,
				singleNamespace:        false,
			}
		)

		g.By("check if cockroachdb is already installed with all ns.")
		csvList := getResource(oc, asAdmin, withoutNamespace, "csv", "-n", subCockroachdb.namespace, "-o=jsonpath={.items[*].metadata.name}")
		if !strings.Contains(csvList, subCockroachdb.operatorPackage) {
			g.By("create catsrc")
			catsrc.create(oc, itName, dr)
			defer catsrc.delete(itName, dr)

			g.By("Create operator targeted at all namespace")
			subCockroachdb.create(oc, itName, dr)
			csvCockroachdb := csvDescription{
				name:      subCockroachdb.installedCSV,
				namespace: subCockroachdb.namespace,
			}
			defer subCockroachdb.delete(itName, dr)
			defer csvCockroachdb.delete(itName, dr)
			crdName := getResource(oc, asAdmin, withoutNamespace, "operator", subCockroachdb.operatorPackage+"."+subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='CustomResourceDefinition')].name}")
			o.Expect(crdName).NotTo(o.BeEmpty())
			defer doAction(oc, "delete", asAdmin, withoutNamespace, "crd", crdName)
			defer doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subCockroachdb.operatorPackage+"."+subCockroachdb.namespace)

			g.By("Check all resources via operators")
			resourceKind := getResource(oc, asAdmin, withoutNamespace, "operator", subCockroachdb.operatorPackage+"."+subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}")
			o.Expect(resourceKind).To(o.ContainSubstring("Deployment"))
			o.Expect(resourceKind).To(o.ContainSubstring("ServiceAccount"))
			o.Expect(resourceKind).To(o.ContainSubstring("Role"))
			o.Expect(resourceKind).To(o.ContainSubstring("RoleBinding"))
			o.Expect(resourceKind).To(o.ContainSubstring("ClusterRole"))
			o.Expect(resourceKind).To(o.ContainSubstring("ClusterRoleBinding"))
			o.Expect(resourceKind).To(o.ContainSubstring("CustomResourceDefinition"))
			o.Expect(resourceKind).To(o.ContainSubstring("Subscription"))
			o.Expect(resourceKind).To(o.ContainSubstring("InstallPlan"))
			o.Expect(resourceKind).To(o.ContainSubstring("ClusterServiceVersion"))
			newCheck("expect", asAdmin, withoutNamespace, contain, subCockroachdb.namespace, ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].namespace}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "InstallSucceeded", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].conditions[*].reason}"}).check(oc)

			g.By("unlabel resource and it is relabeled automatically")
			clusterRoleName := getResource(oc, asAdmin, withoutNamespace, "operator", subCockroachdb.operatorPackage+"."+subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterRole')].name}")
			o.Expect(clusterRoleName).NotTo(o.BeEmpty())
			_, err := doAction(oc, "label", asAdmin, withoutNamespace, "ClusterRole", clusterRoleName, "operators.coreos.com/"+subCockroachdb.operatorPackage+"."+subCockroachdb.namespace+"-")
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "ClusterRole", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

			g.By("delete opertor and the Operator still exists because of crd")
			subCockroachdb.delete(itName, dr)
			csvCockroachdb.delete(itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subCockroachdb.operatorPackage + "." + subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

			g.By("reinstall operator and check resource via Operator")
			subCockroachdb1 := subCockroachdb
			subCockroachdb1.create(oc, itName, dr)
			defer subCockroachdb1.delete(itName, dr)
			defer doAction(oc, "delete", asAdmin, withoutNamespace, "csv", subCockroachdb1.installedCSV, "-n", subCockroachdb1.namespace)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ClusterServiceVersion", ok, []string{"operator", subCockroachdb1.operatorPackage + "." + subCockroachdb1.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, subCockroachdb1.namespace, ok, []string{"operator", subCockroachdb1.operatorPackage + "." + subCockroachdb1.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].namespace}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "InstallSucceeded", ok, []string{"operator", subCockroachdb1.operatorPackage + "." + subCockroachdb1.namespace, "-o=jsonpath={.status.components.refs[?(.kind=='ClusterServiceVersion')].conditions[*].reason}"}).check(oc)

			g.By("delete operator and delete Operator and it will be recreated because of crd")
			subCockroachdb1.delete(itName, dr)
			_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "csv", subCockroachdb1.installedCSV, "-n", subCockroachdb1.namespace)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subCockroachdb1.operatorPackage+"."+subCockroachdb1.namespace)
			o.Expect(err).NotTo(o.HaveOccurred())
			// here there is issue and take WA
			_, err = doAction(oc, "label", asAdmin, withoutNamespace, "crd", crdName, "operators.coreos.com/"+subCockroachdb1.operatorPackage+"."+subCockroachdb1.namespace+"-")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = doAction(oc, "label", asAdmin, withoutNamespace, "crd", crdName, "operators.coreos.com/"+subCockroachdb1.operatorPackage+"."+subCockroachdb1.namespace+"=")
			o.Expect(err).NotTo(o.HaveOccurred())
			//done for WA
			newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subCockroachdb1.operatorPackage + "." + subCockroachdb1.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		} else {
			g.By("it already exists")
		}
	})
})

var _ = g.Describe("[sig-operators] OLM on VM for an end user handle within a namespace", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-vm-"+getRandomString(), exutil.KubeConfigPath())
		dr = make(describerResrouce)
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {})

	// Test case: OCP-27672, author:xzha@redhat.com
	g.It("VMonly-ConnectedOnly-Author:xzha-Medium-27672-Allow Operator Registry Update Polling with automatic ipApproval [Slow]", func() {
		containerCLI := container.NewPodmanCLI()
		containerTool := "podman"
		quayCLI := container.NewQuayCLI()
		sqlit := db.NewSqlit()
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		defer DeleteDir(buildPruningBaseDir, "fixture-testdata")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		catsrcImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-without-secret.yaml")
		bundleImageTag1 := "quay.io/olmqe/ditto-operator:0.1.0"
		bundleImageTag2 := "quay.io/olmqe/ditto-operator:0.1.1"
		indexTag := "quay.io/olmqe/ditto-index:" + getRandomString()
		defer containerCLI.RemoveImage(indexTag)
		catsrcName := "catsrc-27672-operator" + getRandomString()
		oc.SetupProject()
		namespaceName := oc.Namespace()
		var (
			og = operatorGroupDescription{
				name:      "test-og",
				namespace: namespaceName,
				template:  ogSingleTemplate,
			}

			catsrc = catalogSourceDescription{
				name:        catsrcName,
				namespace:   namespaceName,
				displayName: "Test-Catsrc-17672-Operators",
				publisher:   "Red-Hat",
				sourceType:  "grpc",
				address:     indexTag,
				interval:    "2m0s",
				template:    catsrcImageTemplate,
			}

			sub = subscriptionDescription{
				subName:                "ditto-27672-operator",
				namespace:              namespaceName,
				catalogSourceName:      catsrcName,
				catalogSourceNamespace: namespaceName,
				channel:                "alpha",
				ipApproval:             "Automatic",
				operatorPackage:        "ditto-operator",
				singleNamespace:        true,
				template:               subTemplate,
			}
		)

		itName := g.CurrentGinkgoTestDescription().TestText
		g.By("STEP: create the OperatorGroup ")
		og.createwithCheck(oc, itName, dr)

		g.By("STEP 1: prepare CatalogSource index image")
		_, err := containerCLI.Run("pull").Args(bundleImageTag1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = containerCLI.Run("pull").Args(bundleImageTag2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("catsrc image is %s", indexTag)
		output, err := opm.NewOpmCLI().Run("index").Args("add", "-b", bundleImageTag1, "-t", indexTag, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("push image")
		output, err = containerCLI.Run("push").Args(indexTag).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		defer quayCLI.DeleteTag(strings.Replace(indexTag, "quay.io/", "", 1))
		e2e.Logf("check image")
		indexdbPath := filepath.Join(buildPruningBaseDir, getRandomString())
		err = os.Mkdir(indexdbPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexTag, "--path", "/database/index.db:"+indexdbPath).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbPath, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err := sqlit.CheckOperatorBundlePathExist(path.Join(indexdbPath, "index.db"), bundleImageTag2)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeFalse())

		g.By("STEP 2: Create catalog source")
		catsrc.create(oc, itName, dr)
		g.By("STEP 3: install operator ")
		sub.create(oc, itName, dr)
		o.Expect(sub.getCSV().name).To(o.Equal("ditto-operator.v0.1.0"))

		g.By("STEP 4: update CatalogSource index image")
		output, err = opm.NewOpmCLI().Run("index").Args("add", "-b", bundleImageTag2, "-f", indexTag, "-t", indexTag, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("push image")
		output, err = containerCLI.Run("push").Args(indexTag).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("check index image")
		indexdbPath = filepath.Join(buildPruningBaseDir, getRandomString())
		err = os.Mkdir(indexdbPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexTag, "--path", "/database/index.db:"+indexdbPath).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbPath, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.CheckOperatorBundlePathExist(path.Join(indexdbPath, "index.db"), bundleImageTag2)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("STEP 5: check the operator has been updated")
		err = wait.Poll(3*time.Second, 300*time.Second, func() (bool, error) {
			sub.findInstalledCSV(oc, itName, dr)
			if strings.Compare(sub.installedCSV, "ditto-operator.v0.1.1") == 0 {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("STEP 6: delete the catsrc sub csv")
		catsrc.delete(itName, dr)
		sub.delete(itName, dr)
		sub.getCSV().delete(itName, dr)
	})

	g.It("VMonly-ConnectedOnly-Author:xzha-Medium-27672-Allow Operator Registry Update Polling with manual ipApproval [Slow]", func() {
		containerCLI := container.NewPodmanCLI()
		containerTool := "podman"
		quayCLI := container.NewQuayCLI()
		sqlit := db.NewSqlit()
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		defer DeleteDir(buildPruningBaseDir, "fixture-testdata")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		catsrcImageTemplate := filepath.Join(buildPruningBaseDir, "catalogsource-without-secret.yaml")
		bundleImageTag1 := "quay.io/olmqe/ditto-operator:0.1.0"
		bundleImageTag2 := "quay.io/olmqe/ditto-operator:0.1.1"
		indexTag := "quay.io/olmqe/ditto-index:" + getRandomString()
		defer containerCLI.RemoveImage(indexTag)
		catsrcName := "catsrc-27672-operator" + getRandomString()
		oc.SetupProject()
		namespaceName := oc.Namespace()
		var (
			og = operatorGroupDescription{
				name:      "test-og",
				namespace: namespaceName,
				template:  ogSingleTemplate,
			}

			catsrc = catalogSourceDescription{
				name:        catsrcName,
				namespace:   namespaceName,
				displayName: "Test-Catsrc-17672-Operators",
				publisher:   "Red-Hat",
				sourceType:  "grpc",
				address:     indexTag,
				interval:    "2m0s",
				template:    catsrcImageTemplate,
			}
			sub_manual = subscriptionDescription{
				subName:                "ditto-27672-operator",
				namespace:              namespaceName,
				catalogSourceName:      catsrcName,
				catalogSourceNamespace: namespaceName,
				channel:                "alpha",
				ipApproval:             "Manual",
				operatorPackage:        "ditto-operator",
				singleNamespace:        true,
				template:               subTemplate,
			}
		)

		itName := g.CurrentGinkgoTestDescription().TestText
		g.By("STEP: create the OperatorGroup ")
		og.createwithCheck(oc, itName, dr)

		e2e.Logf("catsrc image is %s", indexTag)
		g.By("STEP 1: prepare CatalogSource index image")
		_, err := containerCLI.Run("pull").Args(bundleImageTag1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = containerCLI.Run("pull").Args(bundleImageTag2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := opm.NewOpmCLI().Run("index").Args("add", "-b", bundleImageTag1, "-t", indexTag, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexTag).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		defer quayCLI.DeleteTag(strings.Replace(indexTag, "quay.io/", "", 1))
		e2e.Logf("check image")
		indexdbPath := filepath.Join(buildPruningBaseDir, getRandomString())
		err = os.Mkdir(indexdbPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexTag, "--path", "/database/index.db:"+indexdbPath).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbPath, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err := sqlit.CheckOperatorBundlePathExist(path.Join(indexdbPath, "index.db"), bundleImageTag2)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeFalse())

		g.By("STEP 2: Create catalog source")
		catsrc.create(oc, itName, dr)
		g.By("STEP 3: install operator ")
		sub_manual.create(oc, itName, dr)
		e2e.Logf("approve the install plan")
		sub_manual.approve(oc, itName, dr)
		sub_manual.expectCSV(oc, itName, dr, "ditto-operator.v0.1.0")

		g.By("STEP 4: update CatalogSource index image")
		output, err = opm.NewOpmCLI().Run("index").Args("add", "-b", bundleImageTag2, "-f", indexTag, "-t", indexTag, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexTag).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("check index image")
		indexdbPath = filepath.Join(buildPruningBaseDir, getRandomString())
		err = os.Mkdir(indexdbPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("get index.db")
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexTag, "--path", "/database/index.db:"+indexdbPath).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbPath, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.CheckOperatorBundlePathExist(path.Join(indexdbPath, "index.db"), bundleImageTag2)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("STEP 5: approve the install plan")
		err = wait.Poll(3*time.Second, 300*time.Second, func() (bool, error) {
			ipCsv := getResource(oc, asAdmin, withoutNamespace, "sub", sub_manual.subName, "-n", sub_manual.namespace, "-o=jsonpath={.status.installplan.name}{\" \"}{.status.currentCSV}")
			if strings.Contains(ipCsv, "ditto-operator.v0.1.1") {
				return true, nil
			} else {
				return false, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		sub_manual.approveSpecificIP(oc, itName, dr, "ditto-operator.v0.1.1", "Complete")
		g.By("STEP 6: check the csv")
		sub_manual.expectCSV(oc, itName, dr, "ditto-operator.v0.1.1")
		e2e.Logf("delete the catsrc sub csv")
		catsrc.delete(itName, dr)
		sub_manual.delete(itName, dr)
		sub_manual.getCSV().delete(itName, dr)
	})
})
