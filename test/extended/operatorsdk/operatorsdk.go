package operatorsdk

import (
    "fmt"
    "os/exec"
    "strings"

    g "github.com/onsi/ginkgo"
    o "github.com/onsi/gomega"

    exutil "github.com/openshift/openshift-tests-private/test/extended/util"
    e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] Operator_SDK should", func() {
    defer g.GinkgoRecover()

    var operatorsdkCLI = NewOperatorSDKCLI()
    var oc = exutil.NewCLIWithoutNamespace("default")

    // author: jfan@redhat.com
    g.It("Medium-35458-SDK run bundle create registry image pod", func() {

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
    g.It("High-37465-SDK olm improve olm related sub commands", func() {

        operatorsdkCLI.showInfo = true
        g.By("check the olm status")
        output, _ := operatorsdkCLI.Run("olm").Args("status", "--olm-namespace", "openshift-operator-lifecycle-manager").Output()
        o.Expect(output).To(o.ContainSubstring("Successfully got OLM status for version"))
    })

    // author: jfan@redhat.com
    g.It("High-37312-SDK olm improve manage operator bundles in new manifests metadata format", func() {

        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir /tmp/memcached-operator-37312 && cd /tmp/memcached-operator-37312 && operator-sdk init --project-version 3-alpha --plugins ansible.sdk.operatorframework.io/v1 --domain example.com --group cache --version v1alpha1 --kind Memcached --generate-playbook").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/memcached-operator-37312").Output()
        result, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-37312 && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1 | grep \"Bundle manifests generated successfully in bundle\"").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(result).To(o.ContainSubstring("Bundle manifests generated successfully in bundle"))
    })

    // author: jfan@redhat.com
    g.It("High-37141-SDK Helm support simple structural schema generation for Helm CRDs", func() {

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
    g.It("High-37311-SDK ansible valid structural schemas for ansible based operators", func() {
        operatorsdkCLI.showInfo = true
        exec.Command("bash", "-c", "mkdir /tmp/ansible-operator-37311 && cd /tmp/ansible-operator-37311 && operator-sdk init --project-name nginx-operator --plugins ansible.sdk.operatorframework.io/v1").Output()
        defer exec.Command("bash", "-c", "rm -rf /tmp/ansible-operator-37311").Output()
        _, err := exec.Command("bash", "-c", "cd /tmp/ansible-operator-37311 && operator-sdk create api --group apps --version v1beta1 --kind Nginx").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        output, err := exec.Command("bash", "-c", "cat /tmp/ansible-operator-37311/config/crd/bases/apps.my.domain_nginxes.yaml | grep -E \"x-kubernetes-preserve-unknown-fields: true\"").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        o.Expect(output).To(o.ContainSubstring("x-kubernetes-preserve-unknown-fields: true"))
    })

    // author: chuo@redhat.com
    g.It("Medium-27718-scorecard remove version flag", func() {
        operatorsdkCLI.showInfo = true
        output, _ := operatorsdkCLI.Run("scorecard").Args("--version").Output()
        o.Expect(output).To(o.ContainSubstring("unknown flag: --version"))               
    })
    
    // author: chuo@redhat.com
    g.It("Critical-37655-run bundle upgrade connect to the Operator SDK CLI", func() {
        operatorsdkCLI.showInfo = true
        output, _ := operatorsdkCLI.Run("run").Args("bundle-upgrade", "-h").Output()
        o.Expect(output).To(o.ContainSubstring("Upgrade an Operator previously installed in the bundle format with OLM"))		
    })
})
