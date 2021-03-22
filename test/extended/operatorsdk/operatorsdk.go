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
            indeximage string
            expect string
        }{
            {"quay.io/openshift-qe-optional-operators/ose-cluster-nfd-operator-bundle:latest", "quay.io/openshift-qe-optional-operators/ocp4-index:latest", "Successfully created registry pod"},
        }
        operatorsdkCLI.showInfo = true
        oc.SetupProject()
        for _, b := range bundleImages {
            g.By(fmt.Sprintf("create registry image pod %s", b.image))
            output, err := operatorsdkCLI.Run("run").Args("bundle", b.image, "--index-image", b.indeximage, "-n", oc.Namespace(), "--timeout=5m").Output()
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
        exec.Command("bash", "-c", "mkdir /tmp/memcached-operator-37312 && cd /tmp/memcached-operator-37312 && operator-sdk init --project-version 3-alpha --plugins ansible.sdk.operatorframework.io/v1 --domain example.com --group cache --version v1alpha1 --kind Memcached --generate-playbook").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/memcached-operator-37312").Output()
        result, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-37312 && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1 | grep \"Bundle manifests generated successfully in bundle\"").Output()
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
        output, err = operatorsdkCLI.Run("run").Args("cleanup", "etcd", "-n", oc.Namespace()).Output()
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
            msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
            if strings.Contains(msg, "memcached-sample") {
                e2e.Logf("found pod memcached-sample")
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
})
