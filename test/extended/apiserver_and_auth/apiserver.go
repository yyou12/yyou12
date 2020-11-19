package apiserver_and_auth

import (
	"time"
	"strconv"
	"strings"
	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery] Apiserver_and_Auth", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: kewang@redhat.com
	g.It("Medium-32383-bug 1793694 init container setup should have the proper securityContext", func() {
		checkItems := []struct {
			namespace string
			container string
		}{
			{
				namespace: "openshift-kube-apiserver",
				container: "kube-apiserver",
			},
			{
				namespace: "openshift-apiserver",
				container: "openshift-apiserver",
			},
		}

		for _, checkItem := range checkItems {
			g.By("Get one pod name of " + checkItem.namespace)
			e2e.Logf("namespace is :%s", checkItem.namespace)
			podName, err := oc.AsAdmin().Run("get").Args("-n", checkItem.namespace, "pods", "-l apiserver", "-o=jsonpath={.items[0].metadata.name}").Output()
			if err != nil {
				e2e.Failf("Failed to get kube-apiserver pod name and returned error: %v", err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Get the kube-apiserver pod name: %s", podName)

			g.By("Get privileged value of container " + checkItem.container + " of pod " + podName)
			jsonpath := "-o=jsonpath={range .spec.containers[?(@.name==\"" + checkItem.container + "\")]}{.securityContext.privileged}"
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, jsonpath, "-n", checkItem.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("true"))
			e2e.Logf("#### privileged value: %s ####", msg)

			g.By("Get privileged value of initcontainer of pod " + podName)
			jsonpath = "-o=jsonpath={.spec.initContainers[].securityContext.privileged}"
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, jsonpath, "-n", checkItem.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("true"))
			e2e.Logf("#### privileged value: %s ####", msg)
		}
	})

	// author: xxia@redhat.com
	// It is destructive case, will make kube-apiserver roll out, so adding [Disruptive]. One rollout costs about 25mins, so adding [Slow]
	g.It("Medium-25806-Force encryption key rotation for etcd datastore [Slow][Disruptive]", func() {
		// only run this case in Etcd Encryption On cluster
		g.By("Check if cluster is Etcd Encryption On")
		output, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if "aescbc" == output {
			g.By("Get encryption prefix")
			var err error
			var oasEncValPrefix1, kasEncValPrefix1 string

			oasEncValPrefix1, err = GetEncryptionPrefix(oc, "/openshift.io/routes")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("openshift-apiserver resource encrypted value prefix before test is %s", oasEncValPrefix1)

			kasEncValPrefix1, err = GetEncryptionPrefix(oc, "/kubernetes.io/secrets")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("kube-apiserver resource encrypted value prefix before test is %s", kasEncValPrefix1)

			var oasEncNumber, kasEncNumber int
			oasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-apiserver-[^ ]*`)
			kasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-kube-apiserver-[^ ]*`)

			t := time.Now().Format(time.RFC3339)
			patchYamlToRestore := `[{"op":"replace","path":"/spec/unsupportedConfigOverrides","value":null}]`
			// Below cannot use the patch format "op":"replace" due to it is uncertain
			// whether it is `unsupportedConfigOverrides: null`
			// or the unsupportedConfigOverrides is not existent
			patchYaml := `
spec:
  unsupportedConfigOverrides:
    encryption:
      reason: force OAS rotation ` + t
			for _, kind := range []string{"openshiftapiserver", "kubeapiserver"} {
				defer func() {
					e2e.Logf("Restoring %s/cluster's spec", kind)
					err := oc.WithoutNamespace().Run("patch").Args(kind, "cluster", "--type=json", "-p", patchYamlToRestore).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}()
				g.By("Forcing " + kind + " encryption")
				err := oc.WithoutNamespace().Run("patch").Args(kind, "cluster", "--type=merge", "-p", patchYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			newOASEncSecretName := "encryption-key-openshift-apiserver-" + strconv.Itoa(oasEncNumber+1)
			newKASEncSecretName := "encryption-key-openshift-kube-apiserver-" + strconv.Itoa(kasEncNumber+1)

			g.By("Check the new encryption key secrets appear")
			err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("secrets", newOASEncSecretName, newKASEncSecretName, "-n", "openshift-config-managed").Output()
				if err != nil {
					e2e.Logf("Fail to get new encryption key secrets, error: %s. Trying again", err)
					return false, nil
				}
				e2e.Logf("Got new encryption key secrets:\n%s", output)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for the force encryption completion")
			// Only need to check kubeapiserver because kubeapiserver takes more time.
			var completed bool
			completed, err = WaitEncryptionKeyMigration(oc, newKASEncSecretName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(completed).Should(o.Equal(true))

			var oasEncValPrefix2, kasEncValPrefix2 string
			g.By("Get encryption prefix after force encryption completed")
			oasEncValPrefix2, err = GetEncryptionPrefix(oc, "/openshift.io/routes")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("openshift-apiserver resource encrypted value prefix after test is %s", oasEncValPrefix2)

			kasEncValPrefix2, err = GetEncryptionPrefix(oc, "/kubernetes.io/secrets")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("kube-apiserver resource encrypted value prefix after test is %s", kasEncValPrefix2)

			o.Expect(oasEncValPrefix2).Should(o.ContainSubstring("k8s:enc:aescbc:v1"))
			o.Expect(kasEncValPrefix2).Should(o.ContainSubstring("k8s:enc:aescbc:v1"))
			o.Expect(oasEncValPrefix2).NotTo(o.Equal(oasEncValPrefix1))
			o.Expect(kasEncValPrefix2).NotTo(o.Equal(kasEncValPrefix1))
		} else {
			g.By("cluster is Etcd Encryption Off, this case intentionally runs nothing")
		}
	})

	// author: xxia@redhat.com
	// It is destructive case, will make kube-apiserver roll out, so adding [Disruptive]. One rollout costs about 25mins, so adding [Slow]
	g.It("Medium-25811-Etcd encrypted cluster could self-recover when related encryption configuration is deleted [Slow][Disruptive]", func() {
		// only run this case in Etcd Encryption On cluster
		g.By("Check if cluster is Etcd Encryption On")
		output, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if "aescbc" == output {
			uidsOld, err := oc.WithoutNamespace().Run("get").Args("secret", "encryption-config-openshift-apiserver", "encryption-config-openshift-kube-apiserver", "-n", "openshift-config-managed", `-o=jsonpath={.items[*].metadata.uid}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Delete secrets encryption-config-* in openshift-config-managed")
			for _, item := range []string{"encryption-config-openshift-apiserver", "encryption-config-openshift-kube-apiserver"} {
				e2e.Logf("Remove finalizers from secret %s in openshift-config-managed", item)
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-config-managed", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				e2e.Logf("Delete secret %s in openshift-config-managed", item)
				err = oc.WithoutNamespace().Run("delete").Args("secret", item, "-n", "openshift-config-managed").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			uidsOldSlice := strings.Split(uidsOld, " ")
			e2e.Logf("uidsOldSlice = %s", uidsOldSlice)
			err = wait.Poll(2*time.Second, 60*time.Second, func() (bool, error) {
				uidsNew, err := oc.WithoutNamespace().Run("get").Args("secret", "encryption-config-openshift-apiserver", "encryption-config-openshift-kube-apiserver", "-n", "openshift-config-managed", `-o=jsonpath={.items[*].metadata.uid}`).Output()
				if err != nil {
					e2e.Logf("Fail to get new encryption-config-* secrets, error: %s. Trying again", err)
					return false, nil
				}
				uidsNewSlice := strings.Split(uidsNew, " ")
				e2e.Logf("uidsNewSlice = %s", uidsNewSlice)
				if uidsNewSlice[0] != uidsOldSlice[0] && uidsNewSlice[1] != uidsOldSlice[1] {
					e2e.Logf("Saw recreated secrets encryption-config-* in openshift-config-managed")
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			var oasEncNumber, kasEncNumber int
			oasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred())
			kasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-kube-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred())

			oldOASEncSecretName := "encryption-key-openshift-apiserver-" + strconv.Itoa(oasEncNumber)
			oldKASEncSecretName := "encryption-key-openshift-kube-apiserver-" + strconv.Itoa(kasEncNumber)
			g.By("Delete secrets encryption-key-* in openshift-config-managed")
			for _, item := range []string{oldOASEncSecretName, oldKASEncSecretName} {
				e2e.Logf("Remove finalizers from key %s in openshift-config-managed", item)
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-config-managed", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				e2e.Logf("Delete secret %s in openshift-config-managed", item)
				err = oc.WithoutNamespace().Run("delete").Args("secret", item, "-n", "openshift-config-managed").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			newOASEncSecretName := "encryption-key-openshift-apiserver-" + strconv.Itoa(oasEncNumber+1)
			newKASEncSecretName := "encryption-key-openshift-kube-apiserver-" + strconv.Itoa(kasEncNumber+1)
			g.By("Check the new encryption key secrets appear")
			err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("secrets", newOASEncSecretName, newKASEncSecretName, "-n", "openshift-config-managed").Output()
				if err != nil {
					e2e.Logf("Fail to get new encryption-key-* secrets, error: %s. Trying again", err)
					return false, nil
				}
				e2e.Logf("Got new encryption-key-* secrets:\n%s", output)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			var completed bool
			completed, err = WaitEncryptionKeyMigration(oc, newOASEncSecretName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(completed).Should(o.Equal(true))
			completed, err = WaitEncryptionKeyMigration(oc, newKASEncSecretName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(completed).Should(o.Equal(true))

		} else {
			g.By("cluster is Etcd Encryption Off, this case intentionally runs nothing")
		}
	})

})
