package operatorsdk

import (
    "fmt"
    "os/exec"
    "strings"
    "time"

    g "github.com/onsi/ginkgo"
    o "github.com/onsi/gomega"

    exutil "github.com/openshift/openshift-tests-private/test/extended/util"
    e2e "k8s.io/kubernetes/test/e2e/framework"
    "k8s.io/apimachinery/pkg/util/wait"
    "path/filepath"
)

var _ = g.Describe("[sig-operators] Operator_SDK should", func() {
    defer g.GinkgoRecover()

    var operatorsdkCLI = NewOperatorSDKCLI()
    var oc = exutil.NewCLIWithoutNamespace("default")

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-35458-SDK run bundle create registry image pod", func() {

        bundleImages := []struct {
            image  string
            expect string
        }{
            {"quay.io/olmqe/etcd-bundle:0.9.2-share", "Successfully created registry pod"},
        }
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        for _, b := range bundleImages {
            g.By(fmt.Sprintf("create registry image pod %s", b.image))
            output, err := operatorsdkCLI.Run("run").Args("bundle", b.image, "-n", oc.Namespace(), "--timeout=5m").Output()
            if strings.Contains(output, b.expect) {
                e2e.Logf(fmt.Sprintf("That's expected! %s", b.image))
            } else {
                e2e.Failf(fmt.Sprintf("Failed to validating the %s, error: %v", b.image, err))
            }
        }
    })
    
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
        output, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/etcd-bundle:0.9.2-share", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("OLM has successfully installed"))
        output, err = operatorsdkCLI.Run("run").Args("bundle-upgrade", "quay.io/olmqe/etcd-bundle:0.9.4-share", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("Successfully upgraded to"))
        output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("quay-io-olmqe-etcd-bundle-0-9-4-share"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "etcdoperator.v0.9.4", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("Succeeded"))
        output, err = operatorsdkCLI.Run("cleanup").Args("etcd", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-38054-SDK run bundle create pods and csv", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        output, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/etcd-bundle:0.9.2-share", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace()).Output()
        o.Expect(output).To(o.ContainSubstring("quay-io-olmqe-etcd-bundle-0-9-2-share"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "etcdoperator.v0.9.2", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("Succeeded"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "etcd-catalog", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("grpc"))
        output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("installplan", "-n", oc.Namespace()).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("etcdoperator.v0.9.2"))
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
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var memcached = filepath.Join(buildPruningBaseDir, "cache_v1_memcached.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/memcached-bundle:v4.8", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createMemcached, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", memcached, "-p", "NAME=memcached-sample").OutputToFile("config-27977.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createMemcached, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "memcached-sample") {
                e2e.Logf("found pod memcached-sample")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-34292-SDK ansible operator flags maxConcurrentReconciles by arg max concurrent reconciles", func() {
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/memcached-bundle:v4.8", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        g.By("Check the reconciles number in logs")
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/memcached-operator-controller-manager", "-c", "manager", "-n", namespace).Output()
            if strings.Contains(msg, "\"worker count\":4") {
                e2e.Logf("found worker count:4")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())  
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-28157-SDK ansible blacklist supported in watches.yaml", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var memcached = filepath.Join(buildPruningBaseDir, "cache_v1_memcached.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/memcached-bundle:v4.8", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createMemcached, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", memcached, "-p", "NAME=memcached-sample").OutputToFile("config-28157.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createMemcached, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "memcached-sample") {
                e2e.Logf("found pod memcached-sample")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        msg, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/memcached-operator-controller-manager", "-c", "manager", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("Skipping cache lookup"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-28586-SDK ansible Content Collections Support in watches.yaml", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var collectiontest = filepath.Join(buildPruningBaseDir, "cache_v1_collectiontest.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/memcached-bundle:v4.8", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createCollection, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", collectiontest, "-p", "NAME=collectiontest").OutputToFile("config-28586.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCollection, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/memcached-operator-controller-manager", "-c", "manager", "-n", namespace).Output()
            if strings.Contains(msg, "dummy : Create ConfigMap") {
                e2e.Logf("found dummy : Create ConfigMap")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-29374-SDK ansible Migrate kubernetes Ansible modules to a collect", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var memcached = filepath.Join(buildPruningBaseDir, "cache_v1_memcached.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/memcached-bundle:v4.8", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createMemcached, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", memcached, "-p", "NAME=memcached-sample").OutputToFile("config-29374.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createMemcached, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", namespace).Output()
            if strings.Contains(msg, "test-secret") {
                e2e.Logf("found secret test-secret")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
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
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/nginx-bundle:v4.8", "-n", namespace).Output()
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
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("nginx", "nginx-sample", "-n", namespace).Execute()
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
        msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/raffaelespazzoli-proactive-node-scaling-operator-bundle:latest-", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("Successfully created registry pod: raffaelespazzoli-proactive-node-scaling-operator-bundle-latest"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-35443-SDK run bundle InstallMode for own namespace [Slow]", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
        var operatorGroup = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
        operatorsdkCLI.showInfo = true
        //oc.SetupProject()
        //namespace := oc.Namespace()
        err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-sdk-35443").Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-sdk-35443").Execute()
        // install the operator without og with installmode 
        msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--install-mode", "OwnNamespace", "-n", "test-sdk-35443", "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", "test-sdk-35443", "-o=jsonpath={.spec.targetNamespaces}").Output()
        o.Expect(msg).To(o.ContainSubstring("test-sdk-35443"))
        output, err := operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", "test-sdk-35443").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "-n", "test-sdk-35443", "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        // install the operator with og and installmode
        configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-own", "NAMESPACE=test-sdk-35443",).OutputToFile("config-35443.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--install-mode", "OwnNamespace", "-n", "test-sdk-35443", "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", "test-sdk-35443").Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // install the operator with og without installmode
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "-n", "test-sdk-35443", "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "-n", "test-sdk-35443", "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", "test-sdk-35443").Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // delete the og
        _, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-own", "-n", "test-sdk-35443").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "-n", "test-sdk-35443", "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        // install the operator without og and installmode, the csv support ownnamespace and singlenamespace
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.2.0", "-n", "test-sdk-35443", "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", "test-sdk-35443", "-o=jsonpath={.spec.targetNamespaces}").Output()
        o.Expect(msg).To(o.ContainSubstring("test-sdk-35443"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", "test-sdk-35443").Output()
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
        msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--install-mode", "SingleNamespace=test-sdk-41064", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", namespace, "-o=jsonpath={.spec.targetNamespaces}").Output()
        o.Expect(msg).To(o.ContainSubstring("test-sdk-41064"))
        output, err := operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        // install the operator with og and installmode
        configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-single", "NAMESPACE=" + namespace, "KAKA=test-sdk-41064",).OutputToFile("config-41064.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--install-mode", "SingleNamespace=test-sdk-41064", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // install the operator with og without installmode
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "-n", namespace, "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // delete the og
        _, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-single", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        // install the operator without og and installmode, the csv only support singlenamespace
        msg, _ = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.3.0", "-n", namespace, "--timeout", "1m").Output()
        o.Expect(msg).To(o.ContainSubstring("AllNamespaces InstallModeType not supported"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator", "-n", namespace).Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-Medium-41065-SDK run bundle InstallMode for all namespace [Slow] [Serial]", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
        var operatorGroup = filepath.Join(buildPruningBaseDir, "og-allns.yaml")
        operatorsdkCLI.showInfo = true
        err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-sdk-41065").Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-sdk-41065").Execute()  
        // install the operator without og with installmode all namespace
        defer  oc.AsAdmin().WithoutNamespace().Run("project").Args(oc.Namespace()).Execute()
        err = oc.AsAdmin().WithoutNamespace().Run("project").Args("test-sdk-41065").Execute()
        msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--install-mode", "AllNamespaces", "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        defer operatorsdkCLI.Run("cleanup").Args("example-operator").Output()
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-o=jsonpath={.spec.targetNamespaces}").Output()
        o.Expect(msg).To(o.ContainSubstring(""))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
        o.Expect(msg).To(o.ContainSubstring("example-operator"))
        output, err := operatorsdkCLI.Run("cleanup").Args("example-operator").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        // install the operator with og and installmode
        configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-allnames", "NAMESPACE=test-sdk-41065").OutputToFile("config-41065.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--install-mode", "AllNamespaces", "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
        o.Expect(msg).To(o.ContainSubstring("example-operator"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator").Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // install the operator with og without installmode
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--timeout", "5m").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
        o.Expect(msg).To(o.ContainSubstring("example-operator"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator").Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
        // delete the og
        _, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-allnames", "-n", "test-sdk-41065").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-installmode-bundle-0-1-0", "-n", "test-sdk-41065", "--no-headers").Output()
            if strings.Contains(msg, "not found") {
                e2e.Logf("not found pod quay-io-olmqe-installmode-bundle-0-1-0")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
        // install the operator without og and installmode, the csv only support allnamespace and ownnamespace
        msg, _ = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/installmode-bundle:0.1.0", "--timeout", "5m").Output()
        o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
        msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
        o.Expect(msg).To(o.ContainSubstring("example-operator"))
        output, _ = operatorsdkCLI.Run("cleanup").Args("example-operator").Output()
        o.Expect(output).To(o.ContainSubstring("uninstalled"))
    })

    // author: jfan@redhat.com
    g.It("ConnectedOnly-Author:jfan-High-41497-SDK ansible operatorsdk util k8s status in the task", func() {
        buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
        var memcached = filepath.Join(buildPruningBaseDir, "cache_v1_memcached.yaml")
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        namespace := oc.Namespace()
        _, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/memcached-bundle:v4.8", "-n", namespace).Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        createMemcached, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", memcached, "-p", "NAME=memcached-sample").OutputToFile("config-41497.json")
        o.Expect(err).NotTo(o.HaveOccurred())
        err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createMemcached, "-n", namespace).Execute()
        o.Expect(err).NotTo(o.HaveOccurred())
        waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("memcached.cache.example.com", "memcached-sample", "-n", namespace, "-o", "yaml").Output()
            if strings.Contains(msg, "hello world") {
                e2e.Logf("k8s_status test hello world")
                return true, nil
            }
            return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())
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
        o.Expect(output).To(o.ContainSubstring("Upgrade an Operator previously installed in the bundle format with OLM"))		
    })

    // author: chuo@redhat.com
    g.It("Author:chuo-Medium-34945-ansible Add flag metricsaddr for ansible operator", func() {
        operatorsdkCLI.showInfo = true
        result, err := exec.Command("bash", "-c", "ansible-operator run --help").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(result).To(o.ContainSubstring("--metrics-addr"))	
    })
    // author: chuo@redhat.com
    g.It("Author:chuo-High-37914-Bump k8s in SDK to v1.19 and controller-runtime to 0.7.0", func() {
        operatorsdkCLI.showInfo = true
        output, _ := operatorsdkCLI.Run("version").Args().Output()
        o.Expect(output).To(o.ContainSubstring("v1.19.4"))
    }) 

    // author: chuo@redhat.com
    g.It("ConnectedOnly-Author:chuo-Medium-34366-change ansible operator flags from maxWorkers using env MAXCONCURRENTRECONCILES ", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34366/memcached-operator && cd /tmp/ocp-34366/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
        defer exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && make undeploy").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34366").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34366-data/config/default/manager_auth_proxy_patch.yaml /tmp/ocp-34366/memcached-operator/config/default/manager_auth_proxy_patch.yaml").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34366-data/config/manager/manager.yaml /tmp/ocp-34366/memcached-operator/config/manager/manager.yaml").Output()
        exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34366-data/roles/memcached/tasks/main.yml /tmp/ocp-34366/memcached-operator/roles/memcached/tasks/main.yml").Output()
        
        // to replace namespace memcached-operator-system with memcached-operator-system-ocp34366
        exec.Command("bash", "-c", "sed -i 's/name: system/name: system-ocp34366/g' `grep -rl \"name: system\" /tmp/ocp-34366/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: system/namespace: system-ocp34366/g'  `grep -rl \"namespace: system\" /tmp/ocp-34366/memcached-operator`").Output()
        exec.Command("bash", "-c", "sed -i 's/namespace: memcached-operator-system/namespace: memcached-operator-system-ocp34366/g'  `grep -rl \"namespace: memcached-operator-system\" /tmp/ocp-34366/memcached-operator`").Output()


        exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && make install").Output()
        exec.Command("bash", "-c", "cd /tmp/ocp-34366/memcached-operator && make deploy IMG=quay.io/olmqe/memcached-operator-max-worker:v4.8").Output()

        waitErr := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "memcached-operator-system-ocp34366").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "Running")  {
				return true, nil
			}
			return false, nil
        })
        o.Expect(waitErr).NotTo(o.HaveOccurred())

        podname, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "memcached-operator-system-ocp34366", "-o=jsonpath={.items[0].metadata.name}").Output()
        output, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podname, "-c", "manager", "-n", "memcached-operator-system-ocp34366").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("\"worker count\":6"))	
    })
})
