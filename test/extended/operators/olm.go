package operators

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/google/go-github/github"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"path/filepath"
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: jiazha@redhat.com
	g.It("Medium-25922-Support spec.config.volumes and volumemount in Subscription", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		oc.SetupProject()
		og := operatorGroupDescription{
			name:      "test-og",
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
			channel:                "alpha",
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
		sub.patch(oc, "{\"spec\": {\"channel\":\"alpha\",\"config\":{\"volumeMounts\":[{\"mountPath\":\"/test\",\"name\":\"config-volume\"}],\"volumes\":[{\"configMap\":{\"name\":\"special-config\"},\"name\":\"config-volume\"}]},\"name\":\"etcd\",\"source\":\"community-operators\",\"sourceNamespace\":\"openshift-marketplace\"}}")
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
		sub.patch(oc, "{\"spec\":{\"channel\":\"alpha\",\"config\":{\"volumeMounts\":[{\"mountPath\":\"/test\",\"name\":\"volume1\"}],\"volumes\":[{\"persistentVolumeClaim\":{\"claimName\":\"claim1\"},\"name\":\"volume1\"}]},\"name\":\"etcd\",\"source\":\"community-operators\",\"sourceNamespace\":\"openshift-marketplace\"}}")
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
	g.It("Medium-35631-Remove OperatorSource API", func() {
		g.By("1) Check the operatorsource resource")
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("operatorsource").Output()
		e2e.Logf("Get the expected error: %s", msg)
		o.Expect(msg).To(o.ContainSubstring("the server doesn't have a resource type"))

		g.By("2) Check the default 4 CatalogSource CRs")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "-n", "openshift-marketplace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Get the 4 CatalogSource CRs:\n %s", msg)
		o.Expect(msg).To(o.ContainSubstring("certified-operators"))
		o.Expect(msg).To(o.ContainSubstring("community-operators"))
		o.Expect(msg).To(o.ContainSubstring("redhat-marketplace"))
		o.Expect(msg).To(o.ContainSubstring("redhat-operators"))
	})
	// author: bandrade@redhat.com
	g.It("Medium-31693-Check CSV information on the PackageManifest", func() {
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
	// author: jiazha@redhat.com
	g.It("Medium-33902-Catalog Weighting", func() {
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
			name:      "test-og",
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
		sub.getCSV().delete(itName, dr)
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
	g.It("Medium-32560-Unpacking bundle in InstallPlan fails", func() {
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
		sub.getCSV().delete(itName, dr)
		cs.delete(itName, dr)
	})

	// author: bandrade@redhat.com
	g.It("Medium-30765-Operator-version based dependencies metadata", func() {
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
	g.It("Medium-27680-OLM Bundle support for Prometheus Types", func() {
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

	g.It("Medium-24916-Operators in AllNamespaces should be granted namespace list", func() {
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
		defer sub.getCSV().delete(itName, dr)
		newCheck("expect", asAdmin, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)
		msg, err := oc.AsAdmin().WithoutNamespace().Run("policy").Args("who-can", "list", "namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("system:serviceaccount:openshift-operators:jaeger-operator"))
	})
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
			address:     "quay.io/olmqe/catalogsource:etcd-auto",
			template:    csTemplate,
		}

		dr := make(describerResrouce)
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
		cs.create(oc, itName, dr)
		defer cs.delete(itName, dr)

		newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", cs.name, "-n", cs.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)

		g.By("Start to subscribe this etcd-atuo operator")
		sub := subscriptionDescription{
			subName:                "sub-22070",
			namespace:              "openshift-operators",
			catalogSourceName:      "cs-22070",
			catalogSourceNamespace: "openshift-marketplace",
			channel:                "clusterwide-alpha",
			ipApproval:             "Automatic",
			operatorPackage:        "etcd-auto",
			singleNamespace:        false,
			template:               subTemplate,
		}
		sub.create(oc, itName, dr)
		defer sub.delete(itName, dr)
		defer sub.getCSV().delete(itName, dr)
		newCheck("expect", asAdmin, withNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("Assert that etcd dependency is resolved")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("0.9.4-clusterwide"))
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

	// author: bandrade@redhat.com
	g.It("Low-24055-Check for defaultChannel mandatory field when having multiple channels", func() {
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
	g.It("Medium-20981-contain the source commit id [Serial]", func() {
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

	// author: yhui@redhat.com
	g.It("ConnectedOnly-High-30206-Medium-30242-can include secrets and configmaps in the bundle", func() {
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

		g.By("check packagemanifests")
		err = wait.Poll(10*time.Second, 150*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests").Output()
			if err != nil {
				e2e.Logf("Failed to get packagemanifests, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "OLMCOCKROACHDB-30206") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

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
		err = wait.Poll(30*time.Second, 120*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "test-operators-30206", "secrets", "mysecret").Execute()
			if err != nil {
				e2e.Logf("Failed to create secrets, error:%v", err)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check configmaps")
		err = wait.Poll(30*time.Second, 120*time.Second, func() (bool, error) {
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
		err = wait.Poll(20*time.Second, 100*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "test-operators-30206", "secrets", "mysecret").Execute()
			if err != nil {
				e2e.Logf("The secrets has been deleted")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check configmaps has been deleted")
		err = wait.Poll(20*time.Second, 100*time.Second, func() (bool, error) {
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
	g.It("ConnectedOnly-Medium-30312-can allow admission webhook definitions in CSV", func() {
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

		err = wait.Poll(20*time.Second, 100*time.Second, func() (bool, error) {
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
		err = wait.Poll(20*time.Second, 100*time.Second, func() (bool, error) {
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
	g.It("ConnectedOnly-Medium-30317-can allow mutating admission webhook definitions in CSV", func() {
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

		err = wait.Poll(20*time.Second, 100*time.Second, func() (bool, error) {
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
		err = wait.Poll(20*time.Second, 100*time.Second, func() (bool, error) {
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
	g.It("ConnectedOnly-Medium-30319-Admission Webhook Configuration names should be unique", func() {
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

			err = wait.Poll(20*time.Second, 100*time.Second, func() (bool, error) {
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
	g.It("ConnectedOnly-High-34181-can add conversion webhooks for singleton operators", func() {
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

		g.By("check packagemanifests")
		err = wait.Poll(10*time.Second, 150*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests").Output()
			if err != nil {
				e2e.Logf("Failed to get packagemanifests, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "WebhookOperatorCatalog-34181") {
				return true, nil
			}
			return false, nil
		})
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

		err = wait.Poll(10*time.Second, 100*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("api-resources").Args("-o", "name").Output()
			if err != nil {
				e2e.Logf("There is no WebhookTest, err:%v", err)
				return false, err
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
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).To(o.HaveOccurred())

		g.By("check valid CR")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", crwebhook, "-p", "NAME=webhooktest-34181",
			"NAMESPACE=openshift-operators", "VALID=true").OutputToFile("config-34181.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("WebhookTest", "webhooktest-34181", "-n", "openshift-operators").Execute()
		err = wait.Poll(15*time.Second, 150*time.Second, func() (bool, error) {
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
	g.It("ConnectedOnly-High-29809-can complete automatical updates based on replaces", func() {
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
		g.By("check packagemanifests")
		err = wait.Poll(10*time.Second, 150*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests").Output()
			if err != nil {
				e2e.Logf("Failed to get packagemanifests, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "OLMCOCKROACHDB-REPLACE") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

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

		err = wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
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
	g.It("ConnectedOnly-Medium-29775-Medium-29786-as oc user on linux to mirror catalog image", func() {
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
	g.It("ConnectedOnly-Medium-21825-Certs for packageserver can be rotated successfully", func() {
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
	g.It("ConnectedOnly-Medium-23170-API labels should be hash", func() {
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
	g.It("ConnectedOnly-Medium-20979-only one IP is generated", func() {
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
	g.It("ConnectedOnly-Medium-25757-High-22656-manual approval strategy apply to subsequent releases", func() {
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
		installPlan := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}")
		o.Expect(installPlan).NotTo(o.BeEmpty())
		newCheck("expect", asAdmin, withoutNamespace, compare, "RequiresApproval", ok, []string{"ip", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

		g.By("manually approve sub")
		sub.approve(oc, itName, dr)

		g.By("the target CSV is created with upgrade")
		o.Expect(strings.Compare(sub.installedCSV, sub.startingCSV) != 0).To(o.BeTrue())
	})

	// It will cover test case: OCP-24438, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Medium-24438-check subscription CatalogSource Status", func() {
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
	g.It("ConnectedOnly-Medium-24027-can create and delete catalogsource and sub repeatedly", func() {
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
	g.It("ConnectedOnly-Medium-21404-csv will be RequirementsNotMet after sa is delete", func() {
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
	g.It("ConnectedOnly-Medium-21404-csv will be RequirementsNotMet after role rule is delete", func() {
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
	g.It("ConnectedOnly-Medium-29723-As cluster admin find abnormal status condition via components of operator resource", func() {
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
			fmt.Sprintf("-o=jsonpath={.status.components.refs[?(@.name==\"%s\")].conditions[*].reason}", sub.subName))
		o.Expect(output).To(o.ContainSubstring("UnhealthyCatalogSourceFound"))

		newCheck("expect", asAdmin, withoutNamespace, contain, "RequirementsNotMet", ok, []string{"operator", sub.operatorPackage + "." + sub.namespace,
			fmt.Sprintf("-o=jsonpath={.status.components.refs[?(@.name==\"%s\")].conditions[*].reason}", sub.installedCSV)}).check(oc)
	})

	// It will cover test case: OCP-30762, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Medium-30762-installs bundles with v1 CRDs", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
	g.It("ConnectedOnly-Medium-27683-InstallPlans can install from extracted bundles", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
	g.It("ConnectedOnly-Medium-24513-Operator config support env only", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
	g.It("ConnectedOnly-Medium-24382-Should restrict CRD update if schema changes [Serial]", func() {
		var (
			etcdCluster = filepath.Join(buildPruningBaseDir, "etcd-cluster.yaml")
			itName      = g.CurrentGinkgoTestDescription().TestText
			og          = operatorGroupDescription{
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
	g.It("ConnectedOnly-Medium-25760-Operator upgrades does not fail after change the channel", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
	g.It("ConnectedOnly-Medium-35895-can't install a CSV with duplicate roles", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
	g.It("ConnectedOnly-Medium-32863-Support resources required for SAP Gardener Control Plane Operator", func() {
		var (
			itName      = g.CurrentGinkgoTestDescription().TestText
			vpaTemplate = filepath.Join(buildPruningBaseDir, "vpa-crd.yaml")
			crdVpa      = crdDescription{
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
	g.It("ConnectedOnly-Medium-34472-OLM label dependency", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
	g.It("ConnectedOnly-Medium-37263-Subscription stays in UpgradePending but InstallPlan not installing [Slow]", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			og     = operatorGroupDescription{
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
		newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subCouchbase.installedCSV, "-n", subCouchbase.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

	})

	// It will cover test case: OCP-33176, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Medium-33176-Enable generated operator component adoption for operators with single ns mode [Slow]", func() {
		var (
			itName                  = g.CurrentGinkgoTestDescription().TestText
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
		resourceKind := getResource(oc, asAdmin, withoutNamespace, "operator", subEtcd.operatorPackage+"."+subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}")
		o.Expect(resourceKind).To(o.ContainSubstring("Deployment"))
		o.Expect(resourceKind).To(o.ContainSubstring("ServiceAccount"))
		o.Expect(resourceKind).To(o.ContainSubstring("Role"))
		o.Expect(resourceKind).To(o.ContainSubstring("RoleBinding"))
		o.Expect(resourceKind).To(o.ContainSubstring("CustomResourceDefinition"))
		o.Expect(resourceKind).To(o.ContainSubstring("Subscription"))
		o.Expect(resourceKind).To(o.ContainSubstring("InstallPlan"))
		o.Expect(resourceKind).To(o.ContainSubstring("ClusterServiceVersion"))
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
		newCheck("expect", asAdmin, withoutNamespace, contain, "CustomResourceDefinition", ok, []string{"operator", subEtcd.operatorPackage + "." + subEtcd.namespace, "-o=jsonpath={.status.components.refs[*].kind}"}).check(oc)

		g.By("install Cockroachdb")
		subCockroachdb.create(oc, itName, dr)
		defer doAction(oc, "delete", asAdmin, withoutNamespace, "operator", subCockroachdb.operatorPackage+"."+subCockroachdb.namespace)

		g.By("Check all resources of Cockroachdb via operators")
		resourceKind = getResource(oc, asAdmin, withoutNamespace, "operator", subCockroachdb.operatorPackage+"."+subCockroachdb.namespace, "-o=jsonpath={.status.components.refs[*].kind}")
		o.Expect(resourceKind).To(o.ContainSubstring("Deployment"))
		o.Expect(resourceKind).To(o.ContainSubstring("ServiceAccount"))
		o.Expect(resourceKind).To(o.ContainSubstring("Role"))
		o.Expect(resourceKind).To(o.ContainSubstring("RoleBinding"))
		o.Expect(resourceKind).To(o.ContainSubstring("CustomResourceDefinition"))
		o.Expect(resourceKind).To(o.ContainSubstring("Subscription"))
		o.Expect(resourceKind).To(o.ContainSubstring("InstallPlan"))
		o.Expect(resourceKind).To(o.ContainSubstring("ClusterServiceVersion"))
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

	// It will cover test case: OCP-24917, author: tbuskey@redhat.com
	g.It("Medium-24917-Operators in SingleNamespace should not be granted namespace list [Disruptive]", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
			ogSingleTemplate    = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subTemplate         = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
		)

		oc.SetupProject()

		var (
			next       = false
			podName    = ""
			s          = ""
			secretName = ""
			token      = ""
			kToken     = ""
			og         = operatorGroupDescription{
				name:      oc.Namespace(),
				namespace: oc.Namespace(),
				template:  ogSingleTemplate,
			}
			sub = subscriptionDescription{
				subName:                "amq-streams",
				namespace:              oc.Namespace(),
				catalogSourceName:      "redhat-operators",
				catalogSourceNamespace: "openshift-marketplace",
				startingCSV:            "amqstreams.v1.5.3",
				currentCSV:             "amqstreams.v1.5.3",
				installedCSV:           "amqstreams.v1.5.3",
				singleNamespace:        true,
				channel:                "stable",
				ipApproval:             "Automatic",
				operatorPackage:        "amq-streams",
				template:               subTemplate,
			}
		)

		itName := g.CurrentGinkgoTestDescription().TestText
		nameSpace := oc.Namespace()

		g.By("Create og")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", nameSpace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).NotTo(o.BeEmpty())

		og.createwithCheck(oc, itName, dr)

		g.By("Create sub")
		sub.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "AtLatestKnown", ok, []string{"sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)

		g.By("Wait for pod")
		waitErr := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", sub.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).NotTo(o.BeEmpty())
			if strings.Contains(msg, "amq-streams-cluster-operator") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred())

		g.By("Get pod name")
		podName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "--selector=name=amq-streams-cluster-operator", "-n", sub.namespace, "-o=jsonpath={...metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podName).NotTo(o.BeEmpty())

		newCheck("expect", asAdmin, withoutNamespace, contain, "Running,true", ok, []string{"pod", "-n", nameSpace, podName, "-o=jsonpath={.status.phase}{\",\"}{.status..ready}"}).check(oc)

		g.By("Pod is up")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", sub.namespace, podName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(msg, "amq")).To(o.BeTrue())

		g.By("check that policy does not give strimzi-cluster-operator access ")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("policy").Args("who-can", "list", "namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(msg, "strimzi-cluster-operator")).To(o.BeFalse())

		g.By("Find secret name in SA")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("sa", "strimzi-cluster-operator", "-n", nameSpace, "-o=jsonpath={.secrets..name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(msg, "strimzi-cluster-operator-token")).To(o.BeTrue())

		for _, s = range strings.Fields(msg) {
			if strings.Contains(s, "strimzi-cluster-operator-token") {
				secretName = s
			}
		}
		o.Expect(strings.Contains(secretName, "strimzi-cluster-operator-token")).To(o.BeTrue())

		g.By("Get the tokens")
		kToken, err = oc.AsAdmin().WithoutNamespace().Run("whoami").Args("--show-token", "-n", "default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kToken).NotTo(o.BeEmpty())

		msg, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("secret", secretName, "-n", nameSpace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).NotTo(o.BeEmpty())

		for _, s = range strings.Fields(msg) {
			if next {
				token = s
				break
			}
			if s == "token:" {
				next = true
			}
		}
		o.Expect(token).NotTo(o.BeEmpty())

		g.By("login as strimzi-cluster-operator with token")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("login").Args(fmt.Sprintf("--token=%v", token)).Output()
		/*
			Logged into "https://...:6443" as "system:serviceaccount:test-operators:strimzi-cluster-operator" using the token provided.

			You don't have any projects. Contact your system administrator to request a project.
		*/
		o.Expect(err).NotTo(o.HaveOccurred())

		// make sure to relogin as admin after strimzi login
		defer func() {
			g.By("login as kubeadmin")
			msg, err = oc.AsAdmin().WithoutNamespace().Run("login").Args(fmt.Sprintf("--token=%v", kToken)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).NotTo(o.BeEmpty())
			e2e.Logf("kubeadmin message: %v", msg)
			o.Expect(strings.Contains(msg, "You can list all projects")).To(o.BeTrue())
			e2e.Logf("SUCCESS - logged in as kubeadmin")

		}()

		o.Expect(msg).NotTo(o.BeEmpty())
		e2e.Logf("login message: %v", msg)
		o.Expect(strings.Contains(msg, "You don't have any projects")).To(o.BeTrue())
		e2e.Logf("pass - logged in as strimzi-cluster-operator")

	})

	// author: tbuskey@redhat.com
	g.It("Medium-25782-CatalogSource Status should have information on last observed state", func() {
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
	)

	g.BeforeEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {})

	// It will cover part of test case: OCP-29275, author: kuiwang@redhat.com
	g.It("ConnectedOnly-Medium-29275-label to target namespace of operator group with multi namespace", func() {
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
	g.It("ConnectedOnly-Medium-22200-add minimum kube version to CSV", func() {
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
	g.It("ConnectedOnly-Medium-23473-permit z-stream releases skipping during operator updates", func() {
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
	g.It("ConnectedOnly-Medium-24664-CRD updates if new schemas are backwards compatible", func() {
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
	g.It("ConnectedOnly-Medium-21824-verify CRD should be ready before installing the operator", func() {
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
	g.It("ConnectedOnly-Medium-21484-High-21532-watch special or all namespace by operator group", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			sub    = subscriptionDescription{
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
	g.It("ConnectedOnly-Medium-24906-Operators requesting cluster-scoped permission can trigger kube GC bug", func() {
		var (
			itName = g.CurrentGinkgoTestDescription().TestText
			sub    = subscriptionDescription{
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
	g.It("ConnectedOnly-Medium-33241-Enable generated operator component adoption for operators with all ns mode [Serial]", func() {
		var (
			itName              = g.CurrentGinkgoTestDescription().TestText
			catsrcImageTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
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
