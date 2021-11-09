package operatorsdk

import (
    "os/exec"
    "strings"
    "time"
    "fmt"

    g "github.com/onsi/ginkgo"
    o "github.com/onsi/gomega"

    exutil "github.com/openshift/openshift-tests-private/test/extended/util"
    e2e "k8s.io/kubernetes/test/e2e/framework"
    container "github.com/openshift/openshift-tests-private/test/extended/util/container"
    "k8s.io/apimachinery/pkg/util/wait"
    "path/filepath"
)

var _ = g.Describe("[sig-operators] Operator_SDK should", func() {
    defer g.GinkgoRecover()

    var operatorsdkCLI = NewOperatorSDKCLI()
    var oc = exutil.NewCLIWithoutNamespace("default")

    // author: jfan@redhat.com
    g.It("Author:jfan-High-37465-SDK olm improve olm related sub commands", func() {

        operatorsdkCLI.showInfo = true
        g.By("check the olm status")
        output, _ := operatorsdkCLI.Run("olm").Args("status", "--olm-namespace", "openshift-operator-lifecycle-manager").Output()
        o.Expect(output).To(o.ContainSubstring("Successfully got OLM status for version"))
    })

    // author: jfan@redhat.com
    g.It("Author:jfan-High-37312-SDK olm improve manage operator bundles in new manifests metadata format", func() {

        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir /tmp/memcached-operator-37312 && cd /tmp/memcached-operator-37312 && operator-sdk init --plugins ansible.sdk.operatorframework.io/v1 --domain example.com --group cache --version v1alpha1 --kind Memcached --generate-playbook").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/memcached-operator-37312").Output()
        result, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-37312 && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(result).To(o.ContainSubstring("Bundle manifests generated successfully in bundle"))
    })

    // author: jfan@redhat.com
    g.It("Author:jfan-High-37141-SDK Helm support simple structural schema generation for Helm CRDs", func() {

        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir /tmp/nginx-operator-37141 && cd /tmp/nginx-operator-37141 && operator-sdk init --project-name nginx-operator --plugins helm.sdk.operatorframework.io/v1").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/nginx-operator-37141").Output()
        result, err := exec.Command("bash", "-c", "cd /tmp/nginx-operator-37141 && operator-sdk create api --group apps --version v1beta1 --kind Nginx").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(result).To(o.ContainSubstring("Created helm-charts/nginx"))
        result, err = exec.Command("bash", "-c", "cat /tmp/nginx-operator-37141/config/crd/bases/apps.my.domain_nginxes.yaml | grep -E \"x-kubernetes-preserve-unknown-fields: true\"").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(result).To(o.ContainSubstring("x-kubernetes-preserve-unknown-fields: true"))
    })

    // author: jfan@redhat.com
    g.It("Author:jfan-High-37311-SDK ansible valid structural schemas for ansible based operators", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir /tmp/ansible-operator-37311 && cd /tmp/ansible-operator-37311 && operator-sdk init --project-name nginx-operator --plugins ansible.sdk.operatorframework.io/v1").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ansible-operator-37311").Output()
        _, err := exec.Command("bash", "-c", "cd /tmp/ansible-operator-37311 && operator-sdk create api --group apps --version v1beta1 --kind Nginx").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        output, err := exec.Command("bash", "-c", "cat /tmp/ansible-operator-37311/config/crd/bases/apps.my.domain_nginxes.yaml | grep -E \"x-kubernetes-preserve-unknown-fields: true\"").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("x-kubernetes-preserve-unknown-fields: true"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-37627-SDK run bundle upgrade test", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        output, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/rhoas-operator-bundle:0.6.8", "-n", oc.Namespace(), "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("OLM has successfully installed"))
        output, err = operatorsdkCLI.Run("run").Args("bundle-upgrade", "quay.io/olmqe/rhoas-operator-bundle:0.7.1", "-n", oc.Namespace(), "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("Successfully upgraded to"))
        output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("quay-io-olmqe-rhoas-operator-bundle-0-7-1"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "rhoas-operator.0.7.1", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("Succeeded"))
        output, err = operatorsdkCLI.Run("cleanup").Args("rhoas-operator", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled")) 
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-38054-SDK run bundle create pods and csv and registry image pod", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        output, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/podcsvcheck-bundle:v0.0.1", "-n", oc.Namespace(), "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("quay-io-olmqe-podcsvcheck-bundle-v0-0-1"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "podcsvcheck.v0.0.1", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("Succeeded"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "podcsvcheck-catalog", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("grpc"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("installplan", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("podcsvcheck.v0.0.1"))
        output, err = operatorsdkCLI.Run("cleanup").Args("podcsvcheck", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-38060-SDK run bundle detail message about failed", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        output, _ := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/etcd-bundle:0.0.1", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("quay.io/olmqe/etcd-bundle:0.0.1: not found"))  
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-27977-SDK ansible Implement default Ansible content path in watches.yaml", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/contentpath-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment/contentpath-controller-manager", "-n", namespace, "-c", "manager").Output()
            if strings.Contains(msg, "Starting workers") {
                e2e.Logf("found Starting workers")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("contentpath-controller-manager deployment of %s has not Starting workers", namespace))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-34292-SDK ansible operator flags maxConcurrentReconciles by arg max concurrent reconciles", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        defer operatorsdkCLI.Run("cleanup").Args("max-concurrent-reconciles", "-n", namespace).Output()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/max-concurrent-reconciles-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        g.By("Check the reconciles number in logs")
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/max-concurrent-reconciles-controller-manager", "-c", "manager", "-n", namespace).Output()
            if strings.Contains(msg, "\"worker count\":4") {
                e2e.Logf("found worker count:4")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("log of deploy/max-concurrent-reconciles-controller-manager of %s doesn't have worker count:4", namespace))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-28157-SDK ansible blacklist supported in watches.yaml", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var blacklist = filepath.Join(buildPruningBaseDir, "cache1_v1_blacklist.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/blacklist-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createBlacklist, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", blacklist, "-p", "NAME=blacklist-sample").OutputToFile("config-28157.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createBlacklist, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "blacklist-sample") {
                e2e.Logf("found pod blacklist-sample")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("blacklist-sample pod in %s doesn't found", namespace))
        msg, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/blacklist-controller-manager", "-c", "manager", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("Skipping"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-28586-SDK ansible Content Collections Support in watches.yaml", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var collectiontest = filepath.Join(buildPruningBaseDir, "cache5_v1_collectiontest.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/contentcollections-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createCollection, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", collectiontest, "-p", "NAME=collectiontest").OutputToFile("config-28586.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCollection, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/contentcollections-controller-manager", "-c", "manager", "-n", namespace).Output()
            if strings.Contains(msg, "dummy : Create ConfigMap") {
                e2e.Logf("found dummy : Create ConfigMap")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("miss log dummy : Create ConfigMap in %s", namespace))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-29374-SDK ansible Migrate kubernetes Ansible modules to a collect", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var memcached = filepath.Join(buildPruningBaseDir, "cache2_v1_modulescollect.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/modules-to-collect-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createModules, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", memcached, "-p", "NAME=modulescollect-sample").OutputToFile("config-29374.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createModules, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", namespace).Output()
            if strings.Contains(msg, "test-secret") {
                e2e.Logf("found secret test-secret")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("doesn't get secret test-secret %s", namespace))
        //oc get secret test-secret -o yaml
        msg, err := oc.AsAdmin().Run("describe").Args("secret", "test-secret", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("test:  6 bytes"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-37142-SDK helm cr create deletion process", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var nginx = filepath.Join(buildPruningBaseDir, "demo_v1_nginx.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/nginx-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createNginx, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", nginx, "-p", "NAME=nginx-sample").OutputToFile("config-37142.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createNginx, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "nginx-sample") {
                e2e.Logf("found pod nginx-sample")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("miss pod nginx-sample in %s", namespace))
        err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("nginx.helmdemo.example.com", "nginx-sample", "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
    })

    // author: jfan@redhat.com
    g.It("Author:jfan-High-34441-SDK commad operator sdk support init help message", func() {
        output, err := operatorsdkCLI.Run("init").Args("--help").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("--component-config"))
    })

    // author: jfan@redhat.com
    g.It("Author:jfan-Medium-40521-SDK olm improve manage operator bundles in new manifests metadata format", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir /tmp/memcached-operator-40521 && cd /tmp/memcached-operator-40521 && operator-sdk init --plugins ansible.sdk.operatorframework.io/v1 --domain example.com --group cache --version v1alpha1 --kind Memcached --generate-playbook").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/memcached-operator-40521").Output()
        result, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-40521 && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(result).To(o.ContainSubstring("Bundle manifests generated successfully in bundle"))
        exec.Command("bash", "-c", "cd /tmp/memcached-operator-40521 && sed -i '/icon/,+2d' ./bundle/manifests/memcached-operator-40521.clusterserviceversion.yaml").Output()
        msg, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-40521 && operator-sdk bundle validate ./bundle &> ./validateresult && cat validateresult" ).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("All validation tests have completed successfully"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-40520-SDK k8sutil 1123Label creates invalid values", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        msg, _ := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/raffaelespazzoli-proactive-node-scaling-operator-bundle:latest-", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(msg).To(o.ContainSubstring("Successfully created registry pod: raffaelespazzoli-proactive-node-scaling-operator-bundle-latest"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-35443-SDK run bundle InstallMode for own namespace [Slow]", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
        var operatorGroup = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        // install the operator without og with installmode 
        msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsingleallsupport-bundle:v4.9", "--install-mode", "OwnNamespace", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", namespace, "-o=jsonpath={.spec.targetNamespaces}").Output()
        o.Expect(msg).To(o.ContainSubstring(namespace))
        output, err := operatorsdkCLI.Run("cleanup").Args("ownsingleallsupport", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-ownsingleallsupport-bundle-v4-9", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-ownsingleallsupport-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-ownsingleallsupport-bundle-v4-9 can't be deleted in %s", namespace))
        // install the operator with og and installmode
        configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-own", "NAMESPACE=" + namespace).OutputToFile("config-35443.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsingleallsupport-bundle:v4.9", "--install-mode", "OwnNamespace", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("ownsingleallsupport", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // install the operator with og without installmode
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-ownsingleallsupport-bundle-v4-9", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-ownsingleallsupport-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-ownsingleallsupport-bundle-v4-9 can't be deleted in %s", namespace))
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsingleallsupport-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("ownsingleallsupport", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // delete the og
        _, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-own", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-ownsingleallsupport-bundle-v4-9", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("quay-io-olmqe-ownsingleallsupport-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-ownsingleallsupport-bundle-v4-9 can't be deleted in %s", namespace))
        // install the operator without og and installmode, the csv support ownnamespace and singlenamespace
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsinglesupport-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", namespace, "-o=jsonpath={.spec.targetNamespaces}").Output()
        o.Expect(msg).To(o.ContainSubstring(namespace))
        output, _ = operatorsdkCLI.Run("cleanup").Args("ownsinglesupport", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-41064-SDK run bundle InstallMode for single namespace [Slow]", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var operatorGroup = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-sdk-41064").Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-sdk-41064").Execute()  
        msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all1support-bundle:v4.9", "--install-mode", "SingleNamespace=test-sdk-41064", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", namespace, "-o=jsonpath={.spec.targetNamespaces}").Output()
        o.Expect(msg).To(o.ContainSubstring("test-sdk-41064"))
        output, err := operatorsdkCLI.Run("cleanup").Args("all1support", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all1support-bundle-v4-9", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-all1support-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all1support-bundle-v4-9 can't be deleted in %s", namespace))
        // install the operator with og and installmode
        configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-single", "NAMESPACE=" + namespace, "KAKA=test-sdk-41064",).OutputToFile("config-41064.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all1support-bundle:v4.9", "--install-mode", "SingleNamespace=test-sdk-41064", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("all1support", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // install the operator with og without installmode
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all1support-bundle-v4-9", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-all1support-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all1support-bundle-v4-9 can't be deleted in %s", namespace))
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all1support-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("all1support", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // delete the og
        _, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-single", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all1support-bundle-v4-9", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-all1support-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all1support-bundle-v4-9 can't be deleted in %s", namespace))
        // install the operator without og and installmode, the csv only support singlenamespace
        msg, _ = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/singlesupport-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(msg).To(o.ContainSubstring("AllNamespaces InstallModeType not supported"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("singlesupport", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })


    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-41065-SDK run bundle InstallMode for all namespace [Slow]", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
        var operatorGroup = filepath.Join(buildPruningBaseDir, "og-allns.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        // install the operator without og with installmode all namespace
        msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v4.9", "--install-mode", "AllNamespaces", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        defer operatorsdkCLI.Run("cleanup").Args("all2support").Output()
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-o=jsonpath={.spec.targetNamespaces}", "-n", namespace).Output()
        o.Expect(msg).To(o.ContainSubstring(""))
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
            if strings.Contains(msg, "all2support.v0.0.1") {
                e2e.Logf("csv all2support.v0.0.1")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
        output, err := operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all2support-bundle-v4-9", "--no-headers", "-n", namespace).Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-all2support-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all2support-bundle-v4-9 can't be deleted in %s", namespace))
        // install the operator with og and installmode
        configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-allnames", "NAMESPACE=" + namespace).OutputToFile("config-41065.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v4.9", "--install-mode", "AllNamespaces", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
            if strings.Contains(msg, "all2support.v0.0.1") {
                e2e.Logf("csv all2support.v0.0.1")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
        output, _ = operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // install the operator with og without installmode
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all2support-bundle-v4-9", "--no-headers", "-n", namespace).Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-all2support-bundle-v4-8")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all2support-bundle-v4-9 can't be deleted in %s", namespace))
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
            if strings.Contains(msg, "all2support.v0.0.1") {
                e2e.Logf("csv all2support.v0.0.1")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
        output, _ = operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // delete the og
        _, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-allnames", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all2support-bundle-v4-9", "--no-headers", "-n", namespace).Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-all2support-bundle-v4-9")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all2support-bundle-v4-9 can't be deleted in %s", namespace))
        // install the operator without og and installmode, the csv support allnamespace and ownnamespace
        msg, _ = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
            if strings.Contains(msg, "all2support.v0.0.1") {
                e2e.Logf("csv all2support.v0.0.1")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
        output, _ = operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-41497-SDK ansible operatorsdk util k8s status in the task", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var k8sstatus = filepath.Join(buildPruningBaseDir, "cache3_v1_k8sstatus.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/k8sstatus-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createK8sstatus, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", k8sstatus, "-p", "NAME=k8sstatus-sample").OutputToFile("config-41497.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createK8sstatus, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("k8sstatus.cache3.k8sstatus.com", "k8sstatus-sample", "-n", namespace, "-o", "yaml").Output()
            if strings.Contains(msg, "hello world") {
                e2e.Logf("k8s_status test hello world")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get k8sstatus-sample hello world in %s", namespace))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-38757-SDK operator bundle upgrade from traditional operator installation", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var catalogofwso2am = filepath.Join(buildPruningBaseDir, "catalogsource.yaml")
        var ogofwso2am = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
        var subofwso2am = filepath.Join(buildPruningBaseDir, "sub.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        // install wso2am from sub
        createCatalog, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catalogofwso2am, "-p", "NAME=cs-wso2am", "NAMESPACE=" + namespace, "ADDRESS=quay.io/olmqe/wso2am-index:0.1", "DISPLAYNAME=KakaTest").OutputToFile("catalogsource-41497.json")
        err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCatalog, "-n", namespace).Execute()
        createOg, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", ogofwso2am, "-p", "NAME=kakatest-single", "NAMESPACE=" + namespace, "KAKA=" + namespace).OutputToFile("createog-41497.json")
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createOg, "-n", namespace).Execute()
        createSub, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subofwso2am, "-p", "NAME=wso2aminstall", "NAMESPACE=" + namespace, "SOURCENAME=cs-wso2am", "OPERATORNAME=wso2am-operator", "SOURCENAMESPACE=" + namespace, "STARTINGCSV=wso2am-operator.v1.0.0").OutputToFile("createsub-41497.json")
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createSub, "-n", namespace).Execute()
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "wso2am-operator.v1.0.0", "-o=jsonpath={.status.phase}", "-n", namespace).Output()
            if strings.Contains(msg, "Succeeded") {
                e2e.Logf("wso2am installed successfully")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv wso2am-operator.v1.0.0 %s", namespace))
        // upgrade wso2m by operator-sdk
        msg, err := operatorsdkCLI.Run("run").Args("bundle-upgrade", "quay.io/olmqe/wso2am-operator-bundle:1.0.1", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("Successfully upgraded to"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-42928-SDK support the previous base ansible image [Slow]", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var previouscache = filepath.Join(buildPruningBaseDir, "cache_v1_previous.yaml")
        var previouscollection = filepath.Join(buildPruningBaseDir, "previous_v1_collectiontest.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/previousansiblebase-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createPreviouscache, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", previouscache, "-p", "NAME=previous-sample").OutputToFile("config-42928.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createPreviouscache, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        // k8s status
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("previous.cache.previous.com", "previous-sample", "-n", namespace, "-o", "yaml").Output()
            if strings.Contains(msg, "hello world") {
                e2e.Logf("previouscache test hello world")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get previous-sample hello world in %s", namespace))
        
        // migrate test
        msg, err := oc.AsAdmin().Run("describe").Args("secret", "test-secret", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("test:  6 bytes"))

        // blacklist
        msg, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/previousansiblebase-controller-manager", "-c", "manager", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("Skipping"))

        // max concurrent reconciles
        msg, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/previousansiblebase-controller-manager", "-c", "manager", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("\"worker count\":4"))
        
        // content collection
        createPreviousCollection, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", previouscollection, "-p", "NAME=collectiontest-sample").OutputToFile("config1-42928.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createPreviousCollection, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/previousansiblebase-controller-manager", "-c", "manager", "-n", namespace).Output()
            if strings.Contains(msg, "dummy : Create ConfigMap") {
                e2e.Logf("found dummy : Create ConfigMap")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get log dummy create ConfigMap in %s", namespace))

        output, _ := operatorsdkCLI.Run("cleanup").Args("previousansiblebase", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-42929-SDK support the previous base helm image", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var nginx = filepath.Join(buildPruningBaseDir, "helmbase_v1_nginx.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        defer operatorsdkCLI.Run("cleanup").Args("previoushelmbase", "-n", namespace).Output()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/previoushelmbase-bundle:v4.9", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createPreviousNginx, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", nginx, "-p", "NAME=previousnginx-sample").OutputToFile("config-42929.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createPreviousNginx, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "nginx-sample") {
                e2e.Logf("found pod nginx-sample")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get pod nginx-sample in %s", namespace))
        err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("nginx.helmbase.previous.com", "previousnginx-sample", "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        output, _ := operatorsdkCLI.Run("cleanup").Args("previoushelmbase", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-42614-SDK validate the deprecated APIs and maxOpenShiftVersion", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-42614/traefikee-operator").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-42614").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-42614-data/bundle/ /tmp/ocp-42614/traefikee-operator/").Output()
        
        g.By("with deprecated api, with maxOpenShiftVersion")
        msg, err := operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
        o.Expect(msg).To(o.ContainSubstring("This bundle is using APIs which were deprecated and removed in"))
        o.Expect(msg).NotTo(o.ContainSubstring("error"))
        o.Expect(err).NotTo(o.HaveOccurred())

        g.By("with deprecated api, with higher version maxOpenShiftVersion")
        exec.Command("bash", "-c", "sed -i 's/4.8/4.9/g' /tmp/ocp-42614/traefikee-operator/bundle/manifests/traefikee-operator.v2.1.1.clusterserviceversion.yaml").Output()
        msg, _ = operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
        o.Expect(msg).To(o.ContainSubstring("This bundle is using APIs which were deprecated and removed"))
        o.Expect(msg).To(o.ContainSubstring("error"))

        g.By("with deprecated api, with wrong maxOpenShiftVersion")
        exec.Command("bash", "-c", "sed -i 's/4.9/invalid/g' /tmp/ocp-42614/traefikee-operator/bundle/manifests/traefikee-operator.v2.1.1.clusterserviceversion.yaml").Output()
        msg, _ = operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
        o.Expect(msg).To(o.ContainSubstring("csv.Annotations.olm.properties has an invalid value"))
        o.Expect(msg).To(o.ContainSubstring("error"))

        g.By("with deprecated api, without maxOpenShiftVersion")
        exec.Command("bash", "-c", "sed -i '/invalid/d' /tmp/ocp-42614/traefikee-operator/bundle/manifests/traefikee-operator.v2.1.1.clusterserviceversion.yaml").Output()
        msg, _ = operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
        o.Expect(msg).To(o.ContainSubstring("csv.Annotations not specified olm.maxOpenShiftVersion for an OCP version"))
        o.Expect(msg).To(o.ContainSubstring("error"))
    })

    // author: jfan@redhat.com
    g.It("VMonly-ConnectedOnly-Author:jfan-High-34462-SDK playbook ansible operator generate the catalog [Slow]", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var catalogofcatalog = filepath.Join(buildPruningBaseDir, "catalogsource.yaml")
        var ogofcatalog = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
        var subofcatalog = filepath.Join(buildPruningBaseDir, "sub.yaml")
        oc.SetupProject()
        namespace := oc.Namespace()
        containerCLI := container.NewPodmanCLI()
        g.By("Create the playbook ansible operator")
        _, err := exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34462/catalogtest && cd /tmp/ocp-34462/catalogtest && operator-sdk init --plugins=ansible --domain catalogtest.com").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34462").Output()
        _, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && operator-sdk create api --group cache --version v1 --kind Catalogtest --generate-playbook").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34462-data/Dockerfile /tmp/ocp-34462/catalogtest/Dockerfile").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34462-data/config/default/manager_auth_proxy_patch.yaml /tmp/ocp-34462/catalogtest/config/default/manager_auth_proxy_patch.yaml").Output()
        _, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && make docker-build docker-push IMG=quay.io/olmqe/catalogtest-operator:v4.9").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        defer containerCLI.RemoveImage("quay.io/olmqe/catalogtest-operator:v4.9")

        // ocp-40219
        g.By("Generate the bundle image and catalog index image")
        _, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && sed -i 's#controller:latest#quay.io/olmqe/catalogtest-operator:v4.9#g' /tmp/ocp-34462/catalogtest/Makefile").Output()
        _, err = exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34462-data/manifests/bases/ /tmp/ocp-34462/catalogtest/config/manifests/").Output()
        _, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && make bundle").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        _, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && sed -i 's/--container-tool docker //g' Makefile").Output()
        _, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && make bundle-build bundle-push catalog-build catalog-push BUNDLE_IMG=quay.io/olmqe/catalogtest-bundle:v4.9 CATALOG_IMG=quay.io/olmqe/catalogtest-index:v4.9").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        defer containerCLI.RemoveImage("quay.io/olmqe/catalogtest-bundle:v4.9")
        defer containerCLI.RemoveImage("quay.io/olmqe/catalogtest-index:v4.9")

        g.By("Install the operator through olm")
        createCatalog, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catalogofcatalog, "-p", "NAME=cs-catalog", "NAMESPACE=" + namespace, "ADDRESS=quay.io/olmqe/catalogtest-index:v4.9", "DISPLAYNAME=CatalogTest").OutputToFile("catalogsource-34462.json")
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCatalog, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        createOg, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", ogofcatalog, "-p", "NAME=catalogtest-single", "NAMESPACE=" + namespace, "KAKA=" + namespace).OutputToFile("createog-34462.json")
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createOg, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        createSub, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subofcatalog, "-p", "NAME=cataloginstall", "NAMESPACE=" + namespace, "SOURCENAME=cs-catalog", "OPERATORNAME=catalogtest", "SOURCENAMESPACE=" + namespace, "STARTINGCSV=catalogtest.v0.0.1").OutputToFile("createsub-34462.json")
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createSub, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "catalogtest.v0.0.1", "-o=jsonpath={.status.phase}", "-n", namespace).Output()
            if strings.Contains(msg, "Succeeded") {
                e2e.Logf("catalogtest installed successfully")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv catalogtest.v0.0.1 in %s", namespace))
    })
   
    // author: chuo@redhat.com
    g.It("Author:chuo-Medium-27718-scorecard remove version flag", func() {
        operatorsdkCLI.showInfo = true
        output, _ := operatorsdkCLI.Run("scorecard").Args("--version").Output()
        o.Expect(output).To(o.ContainSubstring("unknown flag: --version"))               
    })
    
    // author: chuo@redhat.com
    g.It("Author:chuo-Critical-37655-run bundle upgrade connect to the Operator SDK CLI", func() {
        operatorsdkCLI.showInfo = true
        output, _ := operatorsdkCLI.Run("run").Args("bundle-upgrade", "-h").Output()
        o.Expect(output).To(o.ContainSubstring("help for bundle-upgrade"))		
    })

    // author: chuo@redhat.com
    g.It("Author:chuo-Medium-34945-ansible Add flag metricsaddr for ansible operator", func() {
        operatorsdkCLI.showInfo = true
        result, err := exec.Command("bash", "-c", "ansible-operator run --help").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(result).To(o.ContainSubstring("--metrics-bind-address"))	
    })
    // author: chuo@redhat.com
    g.It("Author:chuo-High-43654-Sync 1.21 to downstream", func() {
        operatorsdkCLI.showInfo = true
        output, _ := operatorsdkCLI.Run("version").Args().Output()
        o.Expect(output).To(o.ContainSubstring("v1.21"))
    })
    // author: chuo@redhat.com
    g.It("ConnectedOnly-Author:chuo-High-34427-Ensure that Ansible Based Operators creation is working", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34427/memcached-operator && cd /tmp/ocp-34427/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
        defer exec.Command("bash", "-c", "cd /tmp/ocp-34427/memcached-operator && make undeploy").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34427").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34427/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
        exec.Command("bash", "-c", "cp test/extended/util/operatorsdk/ocp-34427-data/roles/memcached/tasks/main.yml /tmp/ocp-34427/memcached-operator/roles/memcached/tasks/main.yml").Output()
        exec.Command("bash", "-c", "cp test/extended/util/operatorsdk/ocp-34427-data/roles/memcached/defaults/main.yml /tmp/ocp-34427/memcached-operator/roles/memcached/defaults/main.yml").Output()
        exec.Command("bash", "-c", "sed -i '$d' /tmp/ocp-34427/memcached-operator/config/samples/cache_v1alpha1_memcached.yaml").Output()
        exec.Command("bash", "-c", "sed -i '$a\\  size: 3' /tmp/ocp-34427/memcached-operator/config/samples/cache_v1alpha1_memcached.yaml").Output()
        exec.Command("bash", "-c", "sed -i 's/v4.9/v4.8/g' /tmp/ocp-34427/memcached-operator/config/default/manager_auth_proxy_patch.yaml").Output()

        // to replace namespace memcached-operator-system with memcached-operator-system-ocp34427
        exec.Command("bash", "-c", "sed -i 's/name: system/name: system-ocp34427/g' `grep -rl \"name: system\" /tmp/ocp-34427/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: system/namespace: system-ocp34427/g'  `grep -rl \"namespace: system\" /tmp/ocp-34427/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: memcached-operator-system/namespace: memcached-operator-system-ocp34427/g'  `grep -rl \"namespace: memcached-operator-system\" /tmp/ocp-34427/memcached-operator`").Output()

        exec.Command("bash", "-c", "cd /tmp/ocp-34427/memcached-operator && make install").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34427/memcached-operator && make deploy IMG=quay.io/olmqe/memcached-operator-ansible-base:v4.9.1").Output()

        _, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", "/tmp/ocp-34427/memcached-operator/config/samples/cache_v1alpha1_memcached.yaml","-n","memcached-operator-system-ocp34427").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
            msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "memcached-operator-system-ocp34427").Output()
            o.Expect(err).NotTo(o.HaveOccurred())
            if strings.Contains(msg, "memcached-sample")  {
                e2e.Logf("found pod memcached-sample")
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("No memcached-sample in project memcached-operator-system-ocp34427"))
        msg, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("deployment/memcached-sample-memcached","-n", "memcached-operator-system-ocp34427").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("3 desired | 3 updated | 3 total | 3 available | 0 unavailable"))
    })
    // author: chuo@redhat.com
    g.It("ConnectedOnly-Author:chuo-Medium-34366-change ansible operator flags from maxWorkers using env MAXCONCURRENTRECONCILES ", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34366/memcached-operator && cd /tmp/ocp-34366/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
        defer exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && make undeploy").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34366").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34366-data/roles/memcached/tasks/main.yml /tmp/ocp-34366/memcached-operator/roles/memcached/tasks/main.yml").Output()
        exec.Command("bash", "-c", "sed -i 's/v4.9/v4.8/g' /tmp/ocp-34427/memcached-operator/config/default/manager_auth_proxy_patch.yaml").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34366-data/config/manager/manager.yaml /tmp/ocp-34366/memcached-operator/config/manager/manager.yaml").Output()
        
        // to replace namespace memcached-operator-system with memcached-operator-system-ocp34366
        exec.Command("bash", "-c", "sed -i 's/name: system/name: system-ocp34366/g' `grep -rl \"name: system\" /tmp/ocp-34366/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: system/namespace: system-ocp34366/g'  `grep -rl \"namespace: system\" /tmp/ocp-34366/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: memcached-operator-system/namespace: memcached-operator-system-ocp34366/g'  `grep -rl \"namespace: memcached-operator-system\" /tmp/ocp-34366/memcached-operator`").Output()

 
        exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && make install").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && make deploy IMG=quay.io/olmqe/memcached-operator-max-worker:v4.9").Output()
        _, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args( "policy", "add-cluster-role-to-user", "cluster-admin", "system:serviceaccount:memcached-operator-system-ocp34366:default").Output()
        waitErr := wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "memcached-operator-system-ocp34366").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "Running")  {
				return true, nil
			}
			return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("memcached-operator-controller-manager of project memcached-operator-system-ocp34366 has no Starting workers"))

        podname, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "memcached-operator-system-ocp34366", "-o=jsonpath={.items[0].metadata.name}").Output()
        output, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podname, "-c", "manager", "-n", "memcached-operator-system-ocp34366").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("\"worker count\":6"))

    })

    // author: chuo@redhat.com
    g.It("Author:chuo-Medium-34883-SDK stamp on Operator bundle image", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34883/memcached-operator && cd /tmp/ocp-34883/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34883").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34883/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34883/memcached-operator && mkdir -p /tmp/ocp-34883/memcached-operator/config/manifests/").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34883-data/manifests/bases/ /tmp/ocp-34883/memcached-operator/config/manifests/").Output()
        waitErr := wait.Poll(30*time.Second, 120*time.Second, func() (bool, error) {
			msg, err := exec.Command("bash", "-c", "cd /tmp/ocp-34883/memcached-operator && make bundle").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(msg), "operator-sdk bundle validate ./bundle")  {
				return true, nil
			}
			return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("operator-sdk bundle generate failed"))
        
        output, err := exec.Command("bash", "-c", "cat /tmp/ocp-34883/memcached-operator/bundle/metadata/annotations.yaml  | grep -E \"operators.operatorframework.io.metrics.builder: operator-sdk\"").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("operators.operatorframework.io.metrics.builder: operator-sdk"))	
    })
    
    // author: chuo@redhat.com
    g.It("ConnectedOnly-Author:chuo-Medium-43973-Medium-43976-scorecard basic test migration and migrate olm tests", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()

        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-43973/memcached-operator && cd /tmp/ocp-43973/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-43973").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-43973/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-43973/memcached-operator && mkdir -p /tmp/ocp-43973/memcached-operator/config/manifests/").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-43973-data/manifests/bases/ /tmp/ocp-43973/memcached-operator/config/manifests/").Output()

        waitErr := wait.Poll(30*time.Second, 120*time.Second, func() (bool, error) {
			msg, err := exec.Command("bash", "-c", "cd /tmp/ocp-43973/memcached-operator && make bundle").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(msg), "operator-sdk bundle validate ./bundle")  {
				return true, nil
			}
			return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("operator-sdk bundle generate failed"))

        //ocp-43973
        g.By("scorecard basic test migration")
        output, err := operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-43973/memcached-operator/bundle", "-c", "/tmp/ocp-43973/memcached-operator/bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=basic-check-spec-test", "-n", oc.Namespace()).Output()
        e2e.Logf(" scorecard bundle %v", err)
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("State: pass"))

        //ocp-43976
        g.By("migrate OLM tests-bundle validation")
        output, err = operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-43973/memcached-operator/bundle", "-c", "/tmp/ocp-43973/memcached-operator/bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-bundle-validation-test", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("State: pass"))	

        g.By("migrate OLM tests-crds have validation test")
        output, err = operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-43973/memcached-operator/bundle", "-c", "/tmp/ocp-43973/memcached-operator/bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-crds-have-validation-test", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("State: fail"))	
        o.Expect(output).To(o.ContainSubstring("Suggestions:"))
        o.Expect(output).To(o.ContainSubstring("Add CRD validation for spec field `foo` in Memcached/v1alpha1"))

        g.By("migrate OLM tests-crds have resources test")
        output, err = operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-43973/memcached-operator/bundle", "-c", "/tmp/ocp-43973/memcached-operator/bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-crds-have-resources-test", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("State: fail"))
        o.Expect(output).To(o.ContainSubstring("Owned CRDs do not have resources specified"))

        g.By("migrate OLM tests- spec descriptors test")
        output, err = operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-43973/memcached-operator/bundle", "-c", "/tmp/ocp-43973/memcached-operator/bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-spec-descriptors-test", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("State: fail"))
        o.Expect(output).To(o.ContainSubstring("Suggestions:"))
        o.Expect(output).To(o.ContainSubstring("Add a spec descriptor for foo"))
        o.Expect(output).To(o.ContainSubstring("foo does not have a spec descriptor"))

        g.By("migrate OLM tests- status descriptors test")
        output, err = operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-43973/memcached-operator/bundle", "-c", "/tmp/ocp-43973/memcached-operator/bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-status-descriptors-test", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("State: fail"))
        o.Expect(output).To(o.ContainSubstring("memcacheds.cache.example.com does not have a status descriptor"))
    })
    // author: chuo@redhat.com
    g.It("ConnectedOnly-Author:chuo-High-31219-scorecard bundle is mandatory ", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-31219/memcached-operator && cd /tmp/ocp-31219/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-31219").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-31219/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-31219/memcached-operator && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1").Output()
        output, err := operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-31219/memcached-operator/bundle", "-c", "/tmp/ocp-31219/memcached-operator/config/scorecard/bases/config.yaml", "-w", "60s", "--selector=test=basic-check-spec-test", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("tests selected"))
    })

    // author: chuo@redhat.com
    g.It("ConnectedOnly-Author:chuo-High-34426-Ensure that Helm Based Operators creation is working ", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34426/nginx-operator && cd /tmp/ocp-34426/nginx-operator && operator-sdk init --plugins=helm").Output()
        defer exec.Command("bash", "-c", "cd /tmp/ocp-34426/nginx-operator && make undeploy").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34426").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34426/nginx-operator && operator-sdk create api --group demo --version v1 --kind Nginx").Output()

        // to replace namespace memcached-operator-system with nginx-operator-system-34426
        exec.Command("bash", "-c", "sed -i 's/name: system/name: system-ocp34426/g' `grep -rl \"name: system\" /tmp/ocp-34426/nginx-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: system/namespace: system-ocp34426/g'  `grep -rl \"namespace: system\" /tmp/ocp-34426/nginx-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: nginx-operator-system/namespace: nginx-operator-system-ocp34426/g'  `grep -rl \"namespace: nginx-operator-system\" /tmp/ocp-34426/nginx-operator`").Output()

        exec.Command("bash", "-c", "cd /tmp/ocp-34426/nginx-operator && make install").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34426/nginx-operator && make deploy IMG=quay.io/olmqe/nginx-operator-base:v4.8").Output()

        _, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args( "policy", "add-scc-to-user", "anyuid", "system:serviceaccount:nginx-operator-system-ocp34426:nginx-sample").Output()
        o.Expect(err).NotTo(o.HaveOccurred())

        _, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args( "-f", "/tmp/ocp-34426/nginx-operator/config/samples/demo_v1_nginx.yaml","-n","nginx-operator-system-ocp34426").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        
        waitErr := wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "nginx-operator-system-ocp34426").Output()
            if strings.Contains(msg, "nginx-sample") {
                return true, nil
            }
            return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("No nginx-sample in project nginx-operator-system-ocp34426"))
        
        podstatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "nginx-operator-system-ocp34426","-o=jsonpath={.items[1].status.phase}").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(podstatus).To(o.ContainSubstring("Running"))
    })


    // author: chuo@redhat.com
     g.It("ConnectedOnly-Author:chuo-High-40341-Ansible operator needs a way to pass vars as unsafe ", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-40341/memcached-operator && cd /tmp/ocp-40341/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
        defer exec.Command("bash", "-c", "cd /tmp/ocp-40341/memcached-operator && make undeploy").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-40341").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-40341/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
        exec.Command("bash", "-c", "sed -i '$d' /tmp/ocp-40341/memcached-operator/config/samples/cache_v1alpha1_memcached.yaml").Output()
        exec.Command("bash", "-c", "sed -i '$a\\  size: 3' /tmp/ocp-40341/memcached-operator/config/samples/cache_v1alpha1_memcached.yaml").Output()
        exec.Command("bash", "-c", "sed -i '$a\\  testKey: testVal' /tmp/ocp-40341/memcached-operator/config/samples/cache_v1alpha1_memcached.yaml").Output()
        
        // to replace namespace memcached-operator-system with memcached-operator-system-ocp40341
        exec.Command("bash", "-c", "sed -i 's/name: system/name: system-ocp40341/g' `grep -rl \"name: system\" /tmp/ocp-40341/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: system/namespace: system-ocp40341/g'  `grep -rl \"namespace: system\" /tmp/ocp-40341/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: memcached-operator-system/namespace: memcached-operator-system-ocp40341/g'  `grep -rl \"namespace: memcached-operator-system\" /tmp/ocp-40341/memcached-operator`").Output()
        
        exec.Command("bash", "-c", "cd /tmp/ocp-40341/memcached-operator && make install").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-40341/memcached-operator && make deploy IMG=quay.io/olmqe/memcached-operator-pass-unsafe:v4.8").Output()
        
    
        waitErr := wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "memcached-operator-system-ocp40341").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "Running")  {
				return true, nil
			}
			return false, nil
        })
        exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("No pod in Running status in project memcached-operator-system-ocp40341"))
        
        _, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", "/tmp/ocp-40341/memcached-operator/config/samples/cache_v1alpha1_memcached.yaml","-n","memcached-operator-system-ocp40341").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        
        output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("memcached.cache.example.com/memcached-sample", "-n", "memcached-operator-system-ocp40341","-o","yaml").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("size: 3"))	
        o.Expect(output).To(o.ContainSubstring("testKey: testVal"))
    })
})
