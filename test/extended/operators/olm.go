package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/google/go-github/github"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"io/ioutil"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: jiazha@redhat.com
	g.It("High-32559-catalog operator crashed", func() {
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

	g.It("Critical-22070-support grpc sourcetype [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		catalogsourceYAML := filepath.Join(buildPruningBaseDir, "image-catalogsource.yaml")
		subscriptionYAML := filepath.Join(buildPruningBaseDir, "image-sub.yaml")

		// Check packagemanifest: Test Operators
		_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", catalogsourceYAML).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "packagemanifest", "etcd-auto").Output()
			if err != nil {
				e2e.Logf("Fail to get packagemanifest:%v, and try again", err)
				return false, nil
			}
			e2e.Logf("Get packagemanifest etcd-auto successfully")
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check the Subscription, InstallPlan, CSV
		_, err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", subscriptionYAML).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		getCmds := []struct {
			kind   string
			expect string
		}{
			{"subscription", "test-operator"},
			{"clusterserviceversion", "etcdoperator.v0.9.4-clusterwide"},
			{"pods", "etcd-operator"},
		}

		for _, cmd := range getCmds {
			err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(cmd.kind, "-n", "openshift-operators").Output()
				if err != nil {
					e2e.Logf("Fail to get %v, error:%v", cmd.kind, err)
					return false, nil
				}
				if strings.Contains(output, cmd.expect) {
					e2e.Logf("Get %v successfully", cmd.kind)
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		// Clean up
		removeCmds := []struct {
			kind      string
			namespace string
			name      string
		}{
			{"catalogsource", "openshift-marketplace", "test-operator"},
			{"subscription", "openshift-operators", "--all"},
			{"clusterserviceversion", "openshift-operators", "etcdoperator.v0.9.4-clusterwide"},
		}
		for _, v := range removeCmds {
			e2e.Logf("Start to remove: %v", v)
			_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	// author: bandrade@redhat.com
	g.It("Medium-21130-Fetching non-existent `PackageManifest` should return 404", func() {
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
	g.It("Low-24057-Have terminationMessagePolicy defined as FallbackToLogsOnError", func() {
		msg, err := oc.SetNamespace("openshift-operator-lifecycle-manager").AsAdmin().Run("get").Args("pods", "-o=jsonpath={range .items[*].spec}{.containers[*].name}{\"\t\"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		amountOfContainers := len(strings.Split(msg, "\t"))

		msg, err = oc.SetNamespace("openshift-operator-lifecycle-manager").AsAdmin().Run("get").Args("pods", "-o=jsonpath={range .items[*].spec}{.containers[*].terminationMessagePolicy}{\"t\"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		regexp := regexp.MustCompile("FallbackToLogsOnError")
		amountOfContainersWithFallbackToLogsOnError := len(regexp.FindAllStringIndex(msg, -1))
		o.Expect(amountOfContainers).To(o.Equal(amountOfContainersWithFallbackToLogsOnError))
		if amountOfContainers != amountOfContainersWithFallbackToLogsOnError {
			e2e.Failf("OLM does not have all containers definied with FallbackToLogsOnError terminationMessagePolicy")
		}
	})

	// author: bandrade@redhat.com
	g.It("High-32613-Operators won't install if the CSV dependency is already installed", func() {

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
	// author: jiazha@redhat.com
	g.It("Medium-20981-contain the source commit id [Serial]", func() {
		sameCommit := ""
		subPods := []string{"catalog-operator", "olm-operator", "packageserver"}

		for _, v := range subPods {
			podName, err := oc.AsAdmin().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "pods", "-l", fmt.Sprintf("app=%s", v), "-o=jsonpath={.items[0].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("get pod name:%s", podName)

			g.By(fmt.Sprintf("get olm version from the %s pod", v))
			oc.SetNamespace("openshift-operator-lifecycle-manager")
			commands := []string{"exec", podName, "--", "olm", "--version"}
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
				client := github.NewClient(nil)
				_, _, err := client.Git.GetCommit(context.Background(), "operator-framework", "operator-lifecycle-manager", gitCommitID)
				if err != nil {
					e2e.Failf("Git.GetCommit returned error: %v", err)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

			} else if gitCommitID != sameCommit {
				e2e.Failf("These commitIDs inconformity!!!")
			}
		}
	})

})

var _ = g.Describe("[sig-operators] OLM for an end user use", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-23440", exutil.KubeConfigPath())
	)

	// author: tbuskey@redhat.com
	g.It("Low-24058-components should have resource limits defined", func() {
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
			if strings.Contains(line, "packageserver") {
				continue
			} else {
				name := strings.Split(line, ",")
				if len(name) > 1 {
					if len(name) > 1 && len(name[1]) < 1 {
						olmUnlimited++
						olmNames = append(olmNames, name[0])
					}
				}
			}
		}
		if olmUnlimited > 0 {
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
	g.It("Medium-22259-marketplace operator CR status on a running cluster [Serial]", func() {

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
	g.It("Medium-24076-check the version of olm operator is appropriate in ClusterOperator", func() {
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
	g.It("Medium-29775-Medium-29786-as oc user on linux to mirror catalog image", func() {
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

	// It will cover test case: OCP-21825, author: kuiwang@redhat.com
	g.It("Medium-21825-Certs for packageserver can be rotated successfully", func() {
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

		buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		dr                  = make(describerResrouce)

		ogD = operatorGroupDescription{
			name:      "og-singlenamespace",
			namespace: "",
			template:  ogSingleTemplate,
		}
		subD = subscriptionDescription{
			name:                   "hawtio-operator",
			namespace:              "",
			channel:                "alpha",
			ipApproval:             "Automatic",
			operator:               "hawtio-operator",
			catalogSourceName:      "community-operators",
			catalogSourceNamespace: "openshift-marketplace",
			startingCSV:            "",
			currentCSV:             "",
			installedCSV:           "",
			template:               subTemplate,
			singleNamespace:        true,
		}
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {})

	// It will cover test case: OCP-29231 and OCP-29277, author: kuiwang@redhat.com
	g.It("Medium-29231-Medium-29277-label to target namespace of group", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og1    = operatorGroupDescription{
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
	g.It("Medium-23170-API labels should be hash", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = ogD
			sub    = subD
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
	g.It("Medium-20979-only one IP is generated", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = ogD
			sub    = subD
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
	g.It("Medium-25757-High-22656-manual approval strategy apply to subsequent releases", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = ogD
			sub    = subD
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
		installPlan := getResource(oc, asAdmin, withoutNamespace, "sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}")
		o.Expect(installPlan).NotTo(o.BeEmpty())
		newCheck("expect", asAdmin, withoutNamespace, compare, "RequiresApproval", ok, []string{"ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("manually approve sub")
		sub.approve(oc, itName, dr)

		g.By("the target CSV is created with upgrade")
		o.Expect(strings.Compare(sub.installedCSV, sub.currentCSV) == 0).To(o.BeTrue())
	})

	// It will cover test case: OCP-24438, author: kuiwang@redhat.com
	g.It("Medium-24438-check subscription CatalogSource Status", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
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
		newCheck("expect", asUser, withoutNamespace, contain, "UnhealthyCatalogSourceFound", ok, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.conditions[*].reason}"}).check(oc)

		g.By("create catalogsource")
		imageAddress := getResource(oc, asAdmin, withoutNamespace, "catsrc", "community-operators", "-n", "openshift-marketplace", "-o=jsonpath={.spec.image}")
		o.Expect(imageAddress).NotTo(o.BeEmpty())
		catsrc.address = imageAddress
		catsrc.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", catsrc.name, "-n", catsrc.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("check its condition is AllCatalogSourcesHealthy and csv is created")
		newCheck("expect", asUser, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.conditions[*].reason}"}).check(oc)
		sub.findInstalledCSV(oc, itName, dr)
	})

	// It will cover test case: OCP-24027, author: kuiwang@redhat.com
	g.It("Medium-24027-can create and delete catalogsource and sub repeatedly", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
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
			sub.getCSV().delete(itName, dr)
			catsrc.delete(itName, dr)
			if i < repeatedCount-1 {
				time.Sleep(20 * time.Second)
			}
		}
	})

	// It will cover part of test case: OCP-21404, author: kuiwang@redhat.com
	g.It("Medium-21404-csv will be RequirementsNotMet after sa is delete", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = ogD
			sub    = subD
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
	g.It("Medium-21404-csv will be RequirementsNotMet after role rule is delete", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = ogD
			sub    = subD
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
	g.It("Medium-29723-As cluster admin find abnormal status condition via components of operator resource", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
				name:                   "cockroachdb",
				namespace:              "",
				channel:                "stable",
				ipApproval:             "Automatic",
				operator:               "cockroachdb",
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
		output := getResource(oc, asAdmin, withoutNamespace, "operator", sub.operator+"."+sub.namespace, "-o=json")
		o.Expect(output).NotTo(o.BeEmpty())

		output = getResource(oc, asAdmin, withoutNamespace, "operator", sub.operator+"."+sub.namespace,
			fmt.Sprintf("-o=jsonpath={.status.components.refs[?(@.name==\"%s\")].conditions[*].reason}", sub.name))
		o.Expect(output).To(o.ContainSubstring("UnhealthyCatalogSourceFound"))

		output = getResource(oc, asAdmin, withoutNamespace, "operator", sub.operator+"."+sub.namespace,
			fmt.Sprintf("-o=jsonpath={.status.components.refs[?(@.name==\"%s\")].conditions[*].message}", sub.installedCSV))
		o.Expect(output).To(o.ContainSubstring("requirements not met"))
	})
})

var _ = g.Describe("[sig-operators] OLM for an end user handle to support", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-cm-"+getRandomString(), exutil.KubeConfigPath())

		buildPruningBaseDir  = exutil.FixturePath("testdata", "olm")
		cmNcTemplate         = filepath.Join(buildPruningBaseDir, "cm-namespaceconfig.yaml")
		cmReadyTestTemplate  = filepath.Join(buildPruningBaseDir, "cm-certutil-readytest.yaml")
		cmReadyTestsTemplate = filepath.Join(buildPruningBaseDir, "cm-certutil-readytests.yaml")
		cmLearnV1Template    = filepath.Join(buildPruningBaseDir, "cm-learn-v1.yaml")
		cmLearnV2Template    = filepath.Join(buildPruningBaseDir, "cm-learn-v2.yaml")
		catsrcCmTemplate     = filepath.Join(buildPruningBaseDir, "catalogsource-configmap.yaml")
		ogTemplate           = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		ogMultiTemplate      = filepath.Join(buildPruningBaseDir, "og-multins.yaml")
		subTemplate          = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		crdOlmtestTemplate   = filepath.Join(buildPruningBaseDir, "crd-olmtest.yaml")
		dr                   = make(describerResrouce)

		cmNc = configMapDescription{
			name:      "cm-community-namespaceconfig-operators",
			namespace: "", //must be set in iT
			template:  cmNcTemplate,
		}
		cmLearn = configMapDescription{
			name:      "cm-learn-operators",
			namespace: "", //must be set in iT
			template:  cmLearnV1Template,
		}
		cmCertUtilReadytest = configMapDescription{
			name:      "cm-certutil-readytest-operators",
			namespace: "", //must be set in iT
			template:  cmReadyTestTemplate,
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
		catsrcLearn = catalogSourceDescription{
			name:        "catsrc-learn-operators",
			namespace:   "", //must be set in iT
			displayName: "Learn Operators",
			publisher:   "Community",
			sourceType:  "configmap",
			address:     "cm-learn-operators",
			template:    catsrcCmTemplate,
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
		subNc = subscriptionDescription{
			name:                   "namespace-configuration-operator",
			namespace:              "", //must be set in iT
			channel:                "alpha",
			ipApproval:             "Automatic",
			operator:               "namespace-configuration-operator",
			catalogSourceName:      "catsrc-community-namespaceconfig-operators",
			catalogSourceNamespace: "", //must be set in iT
			startingCSV:            "",
			currentCSV:             "namespace-configuration-operator.v0.1.0", //it matches to that in cm, so set it.
			installedCSV:           "",
			template:               subTemplate,
			singleNamespace:        true,
		}
		subLearn = subscriptionDescription{
			name:                   "learn-operator",
			namespace:              "", //must be set in iT
			channel:                "alpha",
			ipApproval:             "Automatic",
			operator:               "learn-operator",
			catalogSourceName:      "catsrc-learn-operators",
			catalogSourceNamespace: "", //must be set in iT
			startingCSV:            "",
			currentCSV:             "learn-operator.v0.0.1", //it matches to that in cm, so set it.
			installedCSV:           "",
			template:               subTemplate,
			singleNamespace:        true,
		}
		subCertUtilReadytest = subscriptionDescription{
			name:                   "cert-utils-operator",
			namespace:              "", //must be set in iT
			channel:                "alpha",
			ipApproval:             "Automatic",
			operator:               "cert-utils-operator",
			catalogSourceName:      "catsrc-certutil-readytest-operators",
			catalogSourceNamespace: "", //must be set in iT
			startingCSV:            "",
			currentCSV:             "cert-utils-operator.v0.0.3", //it matches to that in cm, so set it.
			installedCSV:           "",
			template:               subTemplate,
			singleNamespace:        true,
		}
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {})

	// It will cover part of test case: OCP-29275, author: kuiwang@redhat.com
	g.It("Medium-29275-label to target namespace of operator group with multi namespace", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
	g.It("Medium-22200-add minimum kube version to CSV", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
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
		sub.getCSV().delete(itName, dr)
		cm.patch(oc, fmt.Sprintf("{\"data\": {\"clusterServiceVersions\": %s}}", csvDesc))

		g.By("Create sub with orignal KubeVersion")
		sub.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
	})

	// It will cover test case: OCP-23473, author: kuiwang@redhat.com
	g.It("Medium-23473-permit z-stream releases skipping during operator updates", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
			}
			skippedVersion = "namespace-configuration-operator.v0.0.2"
			cm             = cmNc
			catsrc         = catsrcNc
			sub            = subNc
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
	g.It("Medium-24664-CRD updates if new schemas are backwards compatible", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
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
	g.It("Medium-21824-verify CRD should be ready before installing the operator", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
				name:      "og-singlenamespace",
				namespace: "",
				template:  ogTemplate,
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
		newCheck("expect", asAdmin, withoutNamespace, compare, "", ok, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)

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

		buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		dr                  = make(describerResrouce)
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
	g.It("Medium-21484-High-21532-watch special or all namespace by operator group", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			sub    = subscriptionDescription{
				name:                   "composable-operator",
				namespace:              "openshift-operators",
				channel:                "alpha",
				ipApproval:             "Automatic",
				operator:               "composable-operator",
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
	g.It("Medium-24906-Operators requesting cluster-scoped permission can trigger kube GC bug", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			sub    = subscriptionDescription{
				name:                   "amq-streams",
				namespace:              "openshift-operators",
				channel:                "stable",
				ipApproval:             "Automatic",
				operator:               "amq-streams",
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

})

const (
	asAdmin          = true
	asUser           = false
	withoutNamespace = true
	withNamespace    = false
	compare          = true
	contain          = false
	requireNS        = true
	notRequireNS     = false
	present          = true
	notPresent       = false
	ok               = true
	nok              = false
)

type csvDescription struct {
	name      string
	namespace string
}

func (csv csvDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(csv.name, "csv", csv.namespace)
}

type subscriptionDescription struct {
	name                   string
	namespace              string
	channel                string
	ipApproval             string
	operator               string
	catalogSourceName      string
	catalogSourceNamespace string
	startingCSV            string
	currentCSV             string
	installedCSV           string
	template               string
	singleNamespace        bool
	ipCsv                  string
}

func (sub *subscriptionDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	sub.createWithoutCheck(oc, itName, dr)
	if strings.Compare(sub.ipApproval, "Automatic") == 0 {
		sub.findInstalledCSV(oc, itName, dr)
	} else {
		newCheck("expect", asAdmin, withoutNamespace, compare, "UpgradePending", ok, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)
	}
}

func (sub *subscriptionDescription) createWithoutCheck(oc *exutil.CLI, itName string, dr describerResrouce) {
	isAutomatic := strings.Compare(sub.ipApproval, "Automatic") == 0
	if strings.Compare(sub.currentCSV, "") == 0 {
		sub.currentCSV = getResource(oc, asAdmin, withoutNamespace, "packagemanifest", sub.name, fmt.Sprintf("-o=jsonpath={.status.channels[?(@.name==\"%s\")].currentCSV}", sub.channel))
		o.Expect(sub.currentCSV).NotTo(o.BeEmpty())
	}
	if isAutomatic {
		sub.startingCSV = sub.currentCSV
	} else {
		o.Expect(sub.startingCSV).NotTo(o.BeEmpty())
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "SUBNAME="+sub.name, "SUBNAMESPACE="+sub.namespace, "CHANNEL="+sub.channel,
		"APPROVAL="+sub.ipApproval, "OPERATORNAME="+sub.operator, "SOURCENAME="+sub.catalogSourceName, "SOURCENAMESPACE="+sub.catalogSourceNamespace, "STARTINGCSV="+sub.startingCSV)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "sub", sub.name, requireNS, sub.namespace))
}

func (sub *subscriptionDescription) findInstalledCSV(oc *exutil.CLI, itName string, dr describerResrouce) {
	newCheck("expect", asAdmin, withoutNamespace, compare, "AtLatestKnown", ok, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)
	installedCSV := getResource(oc, asAdmin, withoutNamespace, "sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}")
	o.Expect(installedCSV).NotTo(o.BeEmpty())
	if strings.Compare(sub.installedCSV, installedCSV) != 0 {
		sub.installedCSV = installedCSV
		dr.getIr(itName).add(newResource(oc, "csv", sub.installedCSV, requireNS, sub.namespace))
	}
	e2e.Logf("the installed CSV name is %s", sub.installedCSV)
}

func (sub *subscriptionDescription) expectCSV(oc *exutil.CLI, itName string, dr describerResrouce, cv string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		sub.findInstalledCSV(oc, itName, dr)
		if strings.Compare(sub.installedCSV, cv) == 0 {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (sub *subscriptionDescription) approve(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
		for strings.Compare(sub.installedCSV, "") == 0 {
			state := getResource(oc, asAdmin, withoutNamespace, "sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}")
			if strings.Compare(state, "AtLatestKnown") == 0 {
				sub.installedCSV = getResource(oc, asAdmin, withoutNamespace, "sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}")
				dr.getIr(itName).add(newResource(oc, "csv", sub.installedCSV, requireNS, sub.namespace))
				e2e.Logf("it is already done, and the installed CSV name is %s", sub.installedCSV)
				continue
			}

			ipCsv := getResource(oc, asAdmin, withoutNamespace, "sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}{\" \"}{.status.currentCSV}")
			sub.ipCsv = ipCsv + "##" + sub.ipCsv
			installPlan := strings.Fields(ipCsv)[0]
			o.Expect(installPlan).NotTo(o.BeEmpty())
			e2e.Logf("try to approve installPlan %s", installPlan)
			patchResource(oc, asAdmin, withoutNamespace, "ip", installPlan, "-n", sub.namespace, "--type", "merge", "-p", "{\"spec\": {\"approved\": true}}")
			err := wait.Poll(3*time.Second, 10*time.Second, func() (bool, error) {
				err := newCheck("expect", asAdmin, withoutNamespace, compare, "Complete", ok, []string{"ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).checkWithoutAssert(oc)
				if err != nil {
					e2e.Logf("the get error is %v, and try next", err)
					return false, nil
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (sub *subscriptionDescription) getCSV() csvDescription {
	return csvDescription{sub.installedCSV, sub.namespace}
}

func (sub *subscriptionDescription) getInstanceVersion(oc *exutil.CLI) string {
	version := ""
	output := strings.Split(getResource(oc, asUser, withoutNamespace, "csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.metadata.annotations.alm-examples}"), "\n")
	for _, line := range output {
		if strings.Contains(line, "\"version\"") {
			version = strings.Trim(strings.Fields(strings.TrimSpace(line))[1], "\"")
			break
		}
	}
	o.Expect(version).NotTo(o.BeEmpty())
	return version
}

func (sub *subscriptionDescription) createInstance(oc *exutil.CLI, instance string) {
	path := filepath.Join(e2e.TestContext.OutputDir, sub.namespace+"-"+"instance.json")
	err := ioutil.WriteFile(path, []byte(instance), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-n", sub.namespace, "-f", path).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (sub *subscriptionDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(sub.name, "sub", sub.namespace)
}

func (sub *subscriptionDescription) patch(oc *exutil.CLI, patch string) {
	patchResource(oc, asAdmin, withoutNamespace, "sub", sub.name, "-n", sub.namespace, "--type", "merge", "-p", patch)
}

type crdDescription struct {
	name     string
	template string
}

func (crd *crdDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", crd.template, "-p", "NAME="+crd.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "crd", crd.name, notRequireNS, ""))
}

func (crd *crdDescription) delete(oc *exutil.CLI) {
	removeResource(oc, asAdmin, withoutNamespace, "crd", crd.name)
}

type configMapDescription struct {
	name      string
	namespace string
	template  string
}

func (cm *configMapDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cm.template, "-p", "NAME="+cm.name, "NAMESPACE="+cm.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "cm", cm.name, requireNS, cm.namespace))
}
func (cm *configMapDescription) patch(oc *exutil.CLI, patch string) {
	patchResource(oc, asAdmin, withoutNamespace, "cm", cm.name, "-n", cm.namespace, "--type", "merge", "-p", patch)
}

type catalogSourceDescription struct {
	name        string
	namespace   string
	displayName string
	publisher   string
	sourceType  string
	address     string
	template    string
}

func (catsrc *catalogSourceDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", catsrc.template,
		"-p", "NAME="+catsrc.name, "NAMESPACE="+catsrc.namespace, "ADDRESS="+catsrc.address,
		"DISPLAYNAME="+"\""+catsrc.displayName+"\"", "PUBLISHER="+"\""+catsrc.publisher+"\"", "SOURCETYPE="+catsrc.sourceType)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "catsrc", catsrc.name, requireNS, catsrc.namespace))
}
func (catsrc *catalogSourceDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(catsrc.name, "catsrc", catsrc.namespace)
}

type operatorGroupDescription struct {
	name         string
	namespace    string
	multinslabel string
	template     string
}

func (og *operatorGroupDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	var err error
	if strings.Compare(og.multinslabel, "") == 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
	} else {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "MULTINSLABEL="+og.multinslabel)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "og", og.name, requireNS, og.namespace))
}
func (og *operatorGroupDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(og.name, "og", og.namespace)
}

type operatorSourceDescription struct {
	name              string
	namespace         string
	namelabel         string
	registrynamespace string
	displayname       string
	publisher         string
	template          string
	deploymentName    string
}

func (osrc *operatorSourceDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", osrc.template, "-p", "NAME="+osrc.name, "NAMESPACE="+osrc.namespace,
		"NAMELABEL="+osrc.namelabel, "REGISTRYNAMESPACE="+osrc.registrynamespace, "DISPLAYNAME="+osrc.displayname, "PUBLISHER="+osrc.publisher)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "opsrc", osrc.name, requireNS, osrc.namespace))
}

func (osrc *operatorSourceDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(osrc.name, "opsrc", osrc.namespace)
}

func (osrc *operatorSourceDescription) getRunningNodes(oc *exutil.CLI) string {
	nodesNames := getResource(oc, asAdmin, withoutNamespace, "pod", fmt.Sprintf("--selector=marketplace.operatorSource=%s", osrc.name), "-n", osrc.namespace, "-o=jsonpath={.items[*]..nodeName}")
	o.Expect(nodesNames).NotTo(o.BeEmpty())
	return nodesNames
}
func (osrc *operatorSourceDescription) getDeployment(oc *exutil.CLI) {
	output := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=opsrc-owner-name=%s", osrc.name), "-n", osrc.namespace, "-o=jsonpath={.items[0].metadata.name}")
	o.Expect(output).NotTo(o.BeEmpty())
	osrc.deploymentName = output
}
func (osrc *operatorSourceDescription) patchDeployment(oc *exutil.CLI, content string) {
	if strings.Compare(osrc.deploymentName, "") == 0 {
		osrc.deploymentName = osrc.name
	}
	patchResource(oc, asAdmin, withoutNamespace, "deployment", osrc.deploymentName, "-n", osrc.namespace, "--type", "merge", "-p", content)
}
func (osrc *operatorSourceDescription) getTolerations(oc *exutil.CLI) string {
	if strings.Compare(osrc.deploymentName, "") == 0 {
		osrc.deploymentName = osrc.name
	}
	output := getResource(oc, asAdmin, withoutNamespace, "deployment", osrc.deploymentName, "-n", osrc.namespace, "-o=jsonpath={.spec.template.spec.tolerations}")
	if strings.Compare(output, "") == 0 {
		e2e.Logf("no tolerations %v", output)
		return "\"tolerations\": null"
	}
	tolerations := "\"tolerations\": " + convertLMtoJSON(output)
	e2e.Logf("the tolerations:===%v===", tolerations)
	return tolerations
}

type catalogSourceConfigDescription struct {
	name            string
	namespace       string
	packages        string
	targetnamespace string
	source          string
	template        string
}

func (csc *catalogSourceConfigDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", csc.template, "-p", "NAME="+csc.name, "NAMESPACE="+csc.namespace,
		"PACKAGES="+csc.packages, "TARGETNAMESPACE="+csc.targetnamespace, "SOURCE="+csc.source)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "csc", csc.name, requireNS, csc.namespace))
}

func (csc *catalogSourceConfigDescription) delete(itName string, dr describerResrouce) {
	dr.getIr(itName).remove(csc.name, "csc", csc.namespace)
}

type projectDescription struct {
	name            string
	targetNamespace string
}

func (p *projectDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	removeResource(oc, asAdmin, withoutNamespace, "project", p.name)
	_, err := doAction(oc, "new-project", asAdmin, withoutNamespace, p.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "project", p.name, notRequireNS, ""))
	_, err = doAction(oc, "project", asAdmin, withoutNamespace, p.targetNamespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (p *projectDescription) label(oc *exutil.CLI, label string) {
	_, err := doAction(oc, "label", asAdmin, withoutNamespace, "ns", p.name, "env="+label)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (p *projectDescription) delete(oc *exutil.CLI) {
	_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "project", p.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

type serviceAccountDescription struct {
	name           string
	namespace      string
	definitionfile string
}

func newSa(name, namespace string) *serviceAccountDescription {
	return &serviceAccountDescription{
		name:           name,
		namespace:      namespace,
		definitionfile: "",
	}
}
func (sa *serviceAccountDescription) getDefinition(oc *exutil.CLI) {
	parameters := []string{"sa", sa.name, "-n", sa.namespace, "-o=json"}
	definitionfile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(parameters...).OutputToFile("sa-config.json")
	o.Expect(err).NotTo(o.HaveOccurred())
	sa.definitionfile = definitionfile
}
func (sa *serviceAccountDescription) delete(oc *exutil.CLI) {
	_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "sa", sa.name, "-n", sa.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}
func (sa *serviceAccountDescription) reapply(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", sa.definitionfile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}
func (sa *serviceAccountDescription) checkAuth(oc *exutil.CLI, expected string, cr string) {
	err := wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		output, _ := doAction(oc, "auth", asAdmin, withNamespace, "--as", fmt.Sprintf("system:serviceaccount:%s:%s", sa.namespace, sa.name), "can-i", "create", cr)
		e2e.Logf("the result of checkAuth:%v", output)
		if strings.Contains(output, expected) {
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

type roleDescription struct {
	name      string
	namespace string
}

func newRole(name string, namespace string) *roleDescription {
	return &roleDescription{
		name:      name,
		namespace: namespace,
	}
}
func (role *roleDescription) patch(oc *exutil.CLI, patch string) {
	patchResource(oc, asAdmin, withoutNamespace, "role", role.name, "-n", role.namespace, "--type", "merge", "-p", patch)
}
func (role *roleDescription) getRules(oc *exutil.CLI) string {
	return role.getRulesWithDelete(oc, "nodelete")
}
func (role *roleDescription) getRulesWithDelete(oc *exutil.CLI, delete string) string {
	var roleboday map[string]interface{}
	output := getResource(oc, asAdmin, withoutNamespace, "role", role.name, "-n", role.namespace, "-o=json")
	err := json.Unmarshal([]byte(output), &roleboday)
	o.Expect(err).NotTo(o.HaveOccurred())
	rules := roleboday["rules"].([]interface{})

	handleRuleAttribute := func(rc *strings.Builder, rt string, r map[string]interface{}) {
		rc.WriteString("\"" + rt + "\":[")
		items := r[rt].([]interface{})
		e2e.Logf("%s:%v, and the len:%v", rt, items, len(items))
		for i, v := range items {
			vc := v.(string)
			rc.WriteString("\"" + vc + "\"")
			if i != len(items)-1 {
				rc.WriteString(",")
			}
		}
		rc.WriteString("]")
		if strings.Compare(rt, "verbs") != 0 {
			rc.WriteString(",")
		}
	}

	var rc strings.Builder
	rc.WriteString("[")
	for _, rv := range rules {
		rule := rv.(map[string]interface{})
		if strings.Compare(delete, "nodelete") != 0 && strings.Compare(rule["apiGroups"].([]interface{})[0].(string), delete) == 0 {
			continue
		}

		rc.WriteString("{")
		handleRuleAttribute(&rc, "apiGroups", rule)
		handleRuleAttribute(&rc, "resources", rule)
		handleRuleAttribute(&rc, "verbs", rule)
		rc.WriteString("},")
	}
	result := strings.TrimSuffix(rc.String(), ",") + "]"
	e2e.Logf("rc:%v", result)
	return result
}

type checkDescription struct {
	method          string
	executor        bool
	inlineNamespace bool
	expectAction    bool
	expectContent   string
	expect          bool
	resource        []string
}

func newCheck(method string, executor bool, inlineNamespace bool, expectAction bool,
	expectContent string, expect bool, resource []string) checkDescription {
	return checkDescription{
		method:          method,
		executor:        executor,
		inlineNamespace: inlineNamespace,
		expectAction:    expectAction,
		expectContent:   expectContent,
		expect:          expect,
		resource:        resource,
	}
}
func (ck checkDescription) check(oc *exutil.CLI) {
	switch ck.method {
	case "present":
		ok := isPresentResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.resource...)
		o.Expect(ok).To(o.BeTrue())
	case "expect":
		err := expectedResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.expectContent, ck.expect, ck.resource...)
		o.Expect(err).NotTo(o.HaveOccurred())
	default:
		err := fmt.Errorf("unknown method")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}
func (ck checkDescription) checkWithoutAssert(oc *exutil.CLI) error {
	switch ck.method {
	case "present":
		ok := isPresentResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.resource...)
		if ok {
			return nil
		}
		return fmt.Errorf("it is not epxected")
	case "expect":
		return expectedResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.expectContent, ck.expect, ck.resource...)
	default:
		return fmt.Errorf("unknown method")
	}
}

type checkList []checkDescription

func (cl checkList) add(ck checkDescription) {
	cl = append(cl, ck)
}
func (cl checkList) empty() {
	cl = cl[0:0]
}
func (cl checkList) check(oc *exutil.CLI) {
	var wg sync.WaitGroup
	for _, ck := range cl {
		wg.Add(1)
		go func(ck checkDescription) {
			defer g.GinkgoRecover()
			defer wg.Done()
			ck.check(oc)
		}(ck)
	}
	wg.Wait()
}

type resourceDescription struct {
	oc               *exutil.CLI
	asAdmin          bool
	withoutNamespace bool
	kind             string
	name             string
	requireNS        bool
	namespace        string
}

func newResource(oc *exutil.CLI, kind string, name string, nsflag bool, namespace string) resourceDescription {
	return resourceDescription{
		oc:               oc,
		asAdmin:          asAdmin,
		withoutNamespace: withoutNamespace,
		kind:             kind,
		name:             name,
		requireNS:        nsflag,
		namespace:        namespace,
	}
}
func (r resourceDescription) delete() {
	if r.withoutNamespace && r.requireNS {
		removeResource(r.oc, r.asAdmin, r.withoutNamespace, r.kind, r.name, "-n", r.namespace)
	} else {
		removeResource(r.oc, r.asAdmin, r.withoutNamespace, r.kind, r.name)
	}
}

type itResource map[string]resourceDescription

func (ir itResource) add(r resourceDescription) {
	ir[r.name+r.kind+r.namespace] = r
}
func (ir itResource) get(name string, kind string, namespace string) resourceDescription {
	r, ok := ir[name+kind+namespace]
	o.Expect(ok).To(o.BeTrue())
	return r
}
func (ir itResource) remove(name string, kind string, namespace string) {
	rKey := name + kind + namespace
	if r, ok := ir[rKey]; ok {
		r.delete()
		delete(ir, rKey)
	}
}
func (ir itResource) cleanup() {
	for _, r := range ir {
		e2e.Logf("cleanup resource %s,   %s", r.kind, r.name)
		ir.remove(r.name, r.kind, r.namespace)
	}
}

type describerResrouce map[string]itResource

func (dr describerResrouce) addIr(itName string) {
	dr[itName] = itResource{}
}
func (dr describerResrouce) getIr(itName string) itResource {
	ir, ok := dr[itName]
	o.Expect(ok).To(o.BeTrue())
	return ir
}
func (dr describerResrouce) rmIr(itName string) {
	delete(dr, itName)
}

func convertLMtoJSON(content string) string {
	var jb strings.Builder
	jb.WriteString("[")
	items := strings.Split(strings.TrimSuffix(strings.TrimPrefix(content, "["), "]"), "map")
	for _, item := range items {
		if strings.Compare(item, "") == 0 {
			continue
		}
		kvs := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(item, "["), "]"))
		jb.WriteString("{")
		for ki, kv := range kvs {
			p := strings.Split(kv, ":")
			jb.WriteString("\"" + p[0] + "\":")
			jb.WriteString("\"" + p[1] + "\"")
			if ki < len(kvs)-1 {
				jb.WriteString(", ")
			}
		}
		jb.WriteString("},")
	}
	return strings.TrimSuffix(jb.String(), ",") + "]"
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

func generateUpdatedKubernatesVersion(oc *exutil.CLI) string {
	subKubeVersions := strings.Split(getKubernetesVersion(oc), ".")
	zVersion, _ := strconv.Atoi(subKubeVersions[1])
	subKubeVersions[1] = strconv.Itoa(zVersion + 1)
	return strings.Join(subKubeVersions[0:2], ".") + ".0"
}

func getKubernetesVersion(oc *exutil.CLI) string {
	output, err := doAction(oc, "version", asAdmin, withoutNamespace, "-o=json")
	o.Expect(err).NotTo(o.HaveOccurred())

	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	o.Expect(err).NotTo(o.HaveOccurred())

	gitVersion := result["serverVersion"].(map[string]interface{})["gitVersion"]
	e2e.Logf("gitVersion is %v", gitVersion)
	return strings.TrimPrefix(gitVersion.(string), "v")
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "olm-config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func isPresentResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, present bool, parameters ...string) bool {
	parameters = append(parameters, "--ignore-not-found")
	err := wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		if !present && strings.Compare(output, "") == 0 {
			return true, nil
		}
		if present && strings.Compare(output, "") != 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false
	}
	return true
}

func patchResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) {
	_, err := doAction(oc, "patch", asAdmin, withoutNamespace, parameters...)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func execResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	err := wait.Poll(3*time.Second, 6*time.Second, func() (bool, error) {
		output, err := doAction(oc, "exec", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the exec error is %v, and try next", err)
			return false, nil
		}
		result = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of exec resource:%v", result)
	return result
}

func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	err := wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		result = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of queried resource:%v", result)
	return result
}

func expectedResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, isCompare bool, content string, expect bool, parameters ...string) error {
	cc := func(a, b string, ic bool) bool {
		bs := strings.Split(b, "+2+")
		ret := false
		for _, s := range bs {
			if (ic && strings.Compare(a, s) == 0) || (!ic && strings.Contains(a, s)) {
				ret = true
			}
		}
		return ret
	}
	return wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		e2e.Logf("the queried resource:%s", output)
		if isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s matches one of the content %s, expected", output, content)
			return true, nil
		}
		if isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not matche the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s contains one of the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not contain the content %s, expected", output, content)
			return true, nil
		}
		return false, nil
	})
}

func removeResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) {
	output, err := doAction(oc, "delete", asAdmin, withoutNamespace, parameters...)
	if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
		e2e.Logf("the resource is deleted already")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
			e2e.Logf("the resource is delete successfully")
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}
