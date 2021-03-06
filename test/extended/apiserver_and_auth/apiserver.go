package apiserver_and_auth

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"os/exec"
	"os"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery] API_Server", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: kewang@redhat.com
	g.It("Author:kewang-Medium-32383-bug 1793694 init container setup should have the proper securityContext", func() {
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
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-Author:xxia-Medium-25806-Force encryption key rotation for etcd datastore [Slow][Disruptive]", func() {
		// only run this case in Etcd Encryption On cluster
		g.By("Check if cluster is Etcd Encryption On")
		output, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if "aescbc" == output {
			g.By("Get encryption prefix")
			var err error
			var oasEncValPrefix1, kasEncValPrefix1 string

			oasEncValPrefix1, err = GetEncryptionPrefix(oc, "/openshift.io/routes")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key routes ")
			e2e.Logf("openshift-apiserver resource encrypted value prefix before test is %s", oasEncValPrefix1)

			kasEncValPrefix1, err = GetEncryptionPrefix(oc, "/kubernetes.io/secrets")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key secrets ")
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
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("new encryption key secrets %s, %s not found", newOASEncSecretName, newKASEncSecretName))

			g.By("Waiting for the force encryption completion")
			// Only need to check kubeapiserver because kubeapiserver takes more time.
			var completed bool
			completed, err = WaitEncryptionKeyMigration(oc, newKASEncSecretName)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("saw all migrated-resources for %s", newKASEncSecretName))
			o.Expect(completed).Should(o.Equal(true))

			var oasEncValPrefix2, kasEncValPrefix2 string
			g.By("Get encryption prefix after force encryption completed")
			oasEncValPrefix2, err = GetEncryptionPrefix(oc, "/openshift.io/routes")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key routes ")
			e2e.Logf("openshift-apiserver resource encrypted value prefix after test is %s", oasEncValPrefix2)

			kasEncValPrefix2, err = GetEncryptionPrefix(oc, "/kubernetes.io/secrets")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key secrets ")
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
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-NonPreRelease-Author:xxia-Medium-25811-Etcd encrypted cluster could self-recover when related encryption configuration is deleted [Slow][Disruptive]", func() {
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
			exutil.AssertWaitPollNoErr(err, "do not see recreated secrets encryption-config in openshift-config-managed")

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
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("new encryption key secrets %s, %s not found", newOASEncSecretName, newKASEncSecretName))

			var completed bool
			completed, err = WaitEncryptionKeyMigration(oc, newOASEncSecretName)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("saw all migrated-resources for %s", newOASEncSecretName))
			o.Expect(completed).Should(o.Equal(true))
			completed, err = WaitEncryptionKeyMigration(oc, newKASEncSecretName)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("saw all migrated-resources for %s", newKASEncSecretName))
			o.Expect(completed).Should(o.Equal(true))

		} else {
			g.By("cluster is Etcd Encryption Off, this case intentionally runs nothing")
		}
	})

	// author: xxia@redhat.com
	// It is destructive case, will make openshift-kube-apiserver and openshift-apiserver namespaces deleted, so adding [Disruptive].
	// In test the recovery costs about 22mins in max, so adding [Slow]
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-NonPreRelease-Author:xxia-Medium-36801-Etcd encrypted cluster could self-recover when related encryption namespaces are deleted [Slow][Disruptive]", func() {
		// only run this case in Etcd Encryption On cluster
		g.By("Check if cluster is Etcd Encryption On")
		encryptionType, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if "aescbc" == encryptionType {
			jsonPath := `{.items[?(@.metadata.finalizers[0]=="encryption.apiserver.operator.openshift.io/deletion-protection")].metadata.name}`

			secretNames, err := oc.WithoutNamespace().Run("get").Args("secret", "-n", "openshift-apiserver", "-o=jsonpath="+jsonPath).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			// These secrets have deletion-protection finalizers by design. Remove finalizers, otherwise deleting the namespaces will be stuck
			e2e.Logf("Remove finalizers from secret %s in openshift-apiserver", secretNames)
			for _, item := range strings.Split(secretNames, " ") {
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-apiserver", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			e2e.Logf("Remove finalizers from secret %s in openshift-kube-apiserver", secretNames)
			secretNames, err = oc.WithoutNamespace().Run("get").Args("secret", "-n", "openshift-kube-apiserver", "-o=jsonpath="+jsonPath).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, item := range strings.Split(secretNames, " ") {
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-kube-apiserver", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			var uidsOld string
			uidsOld, err = oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver", "openshift-apiserver", `-o=jsonpath={.items[*].metadata.uid}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			uidOldKasNs, uidOldOasNs := strings.Split(uidsOld, " ")[0], strings.Split(uidsOld, " ")[1]

			e2e.Logf("Check openshift-kube-apiserver pods' revisions before deleting namespace")
			oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-L=revision").Execute()
			g.By("Delete namespaces: openshift-kube-apiserver, openshift-apiserver in the background")
			oc.WithoutNamespace().Run("delete").Args("ns", "openshift-kube-apiserver", "openshift-apiserver").Background()
			// Deleting openshift-kube-apiserver may usually need to hang 1+ minutes and then exit.
			// But sometimes (not always, though) if race happens, it will hang forever. We need to handle this as below code
			isKasNsNew, isOasNsNew := false, false
			// In test, observed the max wait time can be 4m, so the parameter is larger
			err = wait.Poll(5*time.Second, 6*time.Minute, func() (bool, error) {
				if !isKasNsNew {
					uidNewKasNs, err := oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver", `-o=jsonpath={.metadata.uid}`).Output()
					if err == nil {
						if uidNewKasNs != uidOldKasNs {
							isKasNsNew = true
							oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver").Execute()
							e2e.Logf("New ns/openshift-kube-apiserver is seen")

						} else {
							stuckTerminating, _ := oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver", `-o=jsonpath={.status.conditions[?(@.type=="NamespaceFinalizersRemaining")].status}`).Output()
							if stuckTerminating == "True" {
								// We need to handle the race (not always happening) by removing new secrets' finazliers to make namepace not stuck in Terminating
								e2e.Logf("Hit race: when ns/openshift-kube-apiserver is Terminating, new encryption-config secrets are seen")
								secretNames, _, _ := oc.WithoutNamespace().Run("get").Args("secret", "-n", "openshift-kube-apiserver", "-o=jsonpath="+jsonPath).Outputs()
								for _, item := range strings.Split(secretNames, " ") {
									oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-kube-apiserver", `-p={"metadata":{"finalizers":null}}`).Execute()
								}
							}
						}
					}
				}
				if !isOasNsNew {
					uidNewOasNs, err := oc.WithoutNamespace().Run("get").Args("ns", "openshift-apiserver", `-o=jsonpath={.metadata.uid}`).Output()
					if err == nil {
						if uidNewOasNs != uidOldOasNs {
							isOasNsNew = true
							oc.WithoutNamespace().Run("get").Args("ns", "openshift-apiserver").Execute()
							e2e.Logf("New ns/openshift-apiserver is seen")
						}
					}
				}
				if isKasNsNew && isOasNsNew {
					e2e.Logf("Now new openshift-apiserver and openshift-kube-apiserver namespaces are both seen")
					return true, nil
				}

				return false, nil
			})

			exutil.AssertWaitPollNoErr(err, "new openshift-apiserver and openshift-kube-apiserver namespaces are not both seen")

			// After new namespaces are seen, it goes to self recovery
			err = wait.Poll(2*time.Second, 2*time.Minute, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
				if err == nil {
					matched, _ := regexp.MatchString("True.*True.*(True|False)", output)
					if matched {
						e2e.Logf("Detected self recovery is in progress\n%s", output)
						return true, nil
					}
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "Detected self recovery is not in progress")
			e2e.Logf("Check openshift-kube-apiserver pods' revisions when self recovery is in progress")
			oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-L=revision").Execute()

			// In test the recovery costs about 22mins in max, so the parameter is larger
			err = wait.Poll(10*time.Second, 25*time.Minute, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
				if err == nil {
					matched, _ := regexp.MatchString("True.*False.*False", output)
					if matched {
						time.Sleep(100 * time.Second)
						output, err := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
						if err == nil {
							if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
								e2e.Logf("co/kubeapiserver True False False already lasts 100s. Means status is stable enough. Recovery completed\n%s", output)
								e2e.Logf("Check openshift-kube-apiserver pods' revisions when recovery completed")
								oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-L=revision").Execute()
								return true, nil
							}
						}
					}
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "openshift-kube-apiserver pods revisions recovery not completed")

			var output string
			output, err = oc.WithoutNamespace().Run("get").Args("co/openshift-apiserver").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			matched, _ := regexp.MatchString("True.*False.*False", output)
			o.Expect(matched).Should(o.Equal(true))

		} else {
			g.By("cluster is Etcd Encryption Off, this case intentionally runs nothing")
		}
	})

	// author: rgangwar@redhat.com
	g.It("NonPreRelease-Longduration-Author:rgangwar-Low-25926-Wire cipher config from apiservers/cluster into apiserver and authentication operators [Disruptive] [Slow]", func() {
		// Check authentication operator cliconfig, openshiftapiservers.operator.openshift.io and kubeapiservers.operator.openshift.io
		var (
			cipher_to_recover           = `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":}]`
			cipherOps                   = []string{"openshift-authentication", "openshiftapiservers.operator", "kubeapiservers.operator"}
			cipher_to_match             = `["TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"] VersionTLS12`
		)

		cipherItems := []struct {
			cipher_type string
			cipher_to_check string
			patch string
		}{
			{
				cipher_type      : "custom",
				cipher_to_check  : `["TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"] VersionTLS11`,
				patch            : `[{"op": "add", "path": "/spec/tlsSecurityProfile", "value":{"custom":{"ciphers":["ECDHE-ECDSA-CHACHA20-POLY1305","ECDHE-RSA-CHACHA20-POLY1305","ECDHE-RSA-AES128-GCM-SHA256","ECDHE-ECDSA-AES128-GCM-SHA256"],"minTLSVersion":"VersionTLS11"},"type":"Custom"}}]`,
			},
			{
				cipher_type      : "Intermediate",
				cipher_to_check  : cipher_to_match, // cipherSuites of "Intermediate" seems to equal to the default values when .spec.tlsSecurityProfile not set.
				patch            : `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":{"intermediate":{},"type":"Intermediate"}}]`,
			},
			{
				cipher_type      : "Old",
				cipher_to_check  : `["TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256","TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA","TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA","TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA","TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA","TLS_RSA_WITH_AES_128_GCM_SHA256","TLS_RSA_WITH_AES_256_GCM_SHA384","TLS_RSA_WITH_AES_128_CBC_SHA256","TLS_RSA_WITH_AES_128_CBC_SHA","TLS_RSA_WITH_AES_256_CBC_SHA","TLS_RSA_WITH_3DES_EDE_CBC_SHA"] VersionTLS10`,
				patch            : `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":{"old":{},"type":"Old"}}]`,
			},
		}

		// Check ciphers for authentication operator cliconfig, openshiftapiservers.operator.openshift.io and kubeapiservers.operator.openshift.io:
		for _, s := range cipherOps {
			err := verify_ciphers(oc, cipher_to_match, s)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Ciphers are not matched : %s", s))
		}

		//Recovering apiserver/cluster's ciphers:
		defer func() {
			g.By("Restoring apiserver/cluster's ciphers")
			output,err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", cipher_to_recover).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(output, "patched (no change)") {
				e2e.Logf("Apiserver/cluster's ciphers are not changed from the default values")
			} else {
				for _, s := range cipherOps {
					err := verify_ciphers(oc, cipher_to_match, s)
					exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Ciphers are not restored : %s", s))
				}
				g.By("Checking KAS, OAS, Auththentication operators should be in Progressing and Available after rollout and recovery")
				e2e.Logf("Checking kube-apiserver operator should be in Progressing in 100 seconds")
				expectedStatus := map[string]string{"Progressing": "True"}
				err = waitCoBecomes(oc, "kube-apiserver", 100, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not start progressing in 100 seconds")
				e2e.Logf("Checking kube-apiserver operator should be Available in 1500 seconds")
				expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
				err = waitCoBecomes(oc, "kube-apiserver", 1500, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not becomes available in 1500 seconds")

				// Using 60s because KAS takes long time, when KAS finished rotation, OAS and Auth should have already finished.
				e2e.Logf("Checking openshift-apiserver operator should be Available in 60 seconds")
				err = waitCoBecomes(oc, "openshift-apiserver", 60, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "openshift-apiserver operator is not becomes available in 60 seconds")

				e2e.Logf("Checking authentication operator should be Available in 60 seconds")
				err = waitCoBecomes(oc, "authentication", 60, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "authentication operator is not becomes available in 60 seconds")
				e2e.Logf("KAS, OAS and Auth operator are available after rollout and cipher's recovery")
			}
		}()

		// Check custom, intermediate, old ciphers for authentication operator cliconfig, openshiftapiservers.operator.openshift.io and kubeapiservers.operator.openshift.io:
		for _, cipherItem := range cipherItems {
			g.By("Patching the apiserver cluster with ciphers : " + cipherItem.cipher_type)
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", cipherItem.patch).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Calling verify_cipher function to check ciphers and minTLSVersion
			for _, s := range cipherOps {
				err := verify_ciphers(oc, cipherItem.cipher_to_check, s)
				exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Ciphers are not matched: %s : %s", s, cipherItem.cipher_type))
			}
			g.By("Checking KAS, OAS, Auththentication operators should be in Progressing and Available after rollout")
			// Calling waitCoBecomes function to wait for define waitTime so that KAS, OAS, Authentication operator becomes progressing and available.
			// WaitTime 100s for KAS becomes Progressing=True and 1500s to become Available=True and Progressing=False and Degraded=False.
			e2e.Logf("Checking kube-apiserver operator should be in Progressing in 100 seconds")
			expectedStatus := map[string]string{"Progressing": "True"}
			err = waitCoBecomes(oc, "kube-apiserver", 100, expectedStatus) // Wait it to become Progressing=True
			exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not start progressing in 100 seconds")
			e2e.Logf("Checking kube-apiserver operator should be Available in 1500 seconds")
			expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			err = waitCoBecomes(oc, "kube-apiserver", 1500, expectedStatus) // Wait it to become Available=True and Progressing=False and Degraded=False
			exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not becomes available in 1500 seconds")

			// Using 60s because KAS takes long time, when KAS finished rotation, OAS and Auth should have already finished.
			e2e.Logf("Checking openshift-apiserver operator should be Available in 60 seconds")
			err = waitCoBecomes(oc, "openshift-apiserver", 60, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "openshift-apiserver operator is not becomes available in 60 seconds")

			e2e.Logf("Checking authentication operator should be Available in 60 seconds")
			err = waitCoBecomes(oc, "authentication", 60, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "authentication operator is not becomes available in 60 seconds")
		}
	})

	// author: rgangwar@redhat.com
	g.It("NonPreRelease-Author:rgangwar-High-41899-Replacing the admin kubeconfig generated at install time [Disruptive] [Slow]", func() {
		var (
			dirname          = "/tmp/-OCP-41899-ca/"
			name             = dirname + "custom"
			validity         = 3650
			ca_subj          = dirname + "/OU=openshift/CN=admin-kubeconfig-signer-custom"
			user             = "system:admin"
			user_cert        = dirname + "system-admin"
			group            = "system:masters"
			user_subj        = dirname + "/O="+group+"/CN="+user
			new_kubeconfig   = dirname + "kubeconfig." + user
			patch            = `[{"op": "add", "path": "/spec/clientCA", "value":{"name":"client-ca-custom"}}]`
			patch_to_recover = `[{"op": "replace", "path": "/spec/clientCA", "value":}]`
			configmap_bkp    = dirname + "OCP-41899-bkp.yaml"
		)

		defer os.RemoveAll(dirname)
		defer func() {
			g.By("Restoring cluster")
			output, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
			if strings.Contains(string(output), "Unauthorized") {
				err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("--kubeconfig", new_kubeconfig, "-f", configmap_bkp).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = wait.Poll(5*time.Second, 100*time.Second, func() (bool, error) {
					output, _ := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
					if output == "system:admin" {
						e2e.Logf("Old kubeconfig is restored : %s", output)
						// Adding wait time to ensure old kubeconfig restored properly
						time.Sleep(60 * time.Second)
						return true, nil
					} else if output == "error: You must be logged in to the server (Unauthorized)" {
						return false, nil
					}
					return false, nil
				})
				exutil.AssertWaitPollNoErr(err, "Old kubeconfig is not restored")
				restore_cluster_ocp_41899(oc)
				e2e.Logf("Cluster recovered")
			} else if err == nil {
				output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patch_to_recover).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Contains(output, "patched (no change)") {
					e2e.Logf("Apiserver/cluster is not changed from the default values")
					restore_cluster_ocp_41899(oc)
				} else {
					output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patch_to_recover).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					restore_cluster_ocp_41899(oc)
				}
			}
		}()

		//Taking backup of configmap "admin-kubeconfig-client-ca" to restore old kubeconfig
		err := os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Get the default CA backup")
		configmap_bkp, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", "admin-kubeconfig-client-ca", "-n", "openshift-config", "-o", "yaml").OutputToFile("OCP-41899-ca/OCP-41899-bkp.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		sed_cmd := fmt.Sprintf(`sed -i '/creationTimestamp:\|resourceVersion:\|uid:/d' %s`, configmap_bkp)
		_, err = exec.Command("bash", "-c", sed_cmd ).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Generation of a new self-signed CA, in case a corporate or another CA is already existing can be used.
		g.By("Generation of a new self-signed CA")
		e2e.Logf("Generate the CA private key")
		openssl_cmd := fmt.Sprintf(`openssl genrsa -out %s-ca.key 4096`, name)
		_, err = exec.Command("bash", "-c", openssl_cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create the CA certificate")
		openssl_cmd = fmt.Sprintf(`openssl req -x509 -new -nodes -key %s-ca.key -sha256 -days %d -out %s-ca.crt -subj %s`, name, validity, name, ca_subj)
		_ ,err = exec.Command("bash", "-c", openssl_cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Generation of a new system:admin certificate. The client certificate must have the user into the x.509 subject CN field and the group into the O field.
		g.By("Generation of a new system:admin certificate")
		e2e.Logf("Create the user CSR")
		openssl_cmd = fmt.Sprintf(`openssl req -nodes -newkey rsa:2048 -keyout %s.key -subj %s -out %s.csr`, user_cert, user_subj, user_cert)
		_, err = exec.Command("bash", "-c", openssl_cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// sign the user CSR and generate the certificate, the certificate must have the `clientAuth` extension
		e2e.Logf("Sign the user CSR and generate the certificate")
		openssl_cmd = fmt.Sprintf(`openssl x509 -extfile <(printf "extendedKeyUsage = clientAuth") -req -in %s.csr -CA %s-ca.crt -CAkey %s-ca.key -CAcreateserial -out %s.crt -days %d -sha256`, user_cert, name, name, user_cert, validity)
		_, err = exec.Command("bash", "-c", openssl_cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// In order to have a safe replacement, before removing the default CA the new certificate is added as an additional clientCA.
		g.By("Create the client-ca ConfigMap")
		ca_file := fmt.Sprintf(`--from-file=ca-bundle.crt=%s-ca.crt`, name)
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", "client-ca-custom", "-n", "openshift-config", ca_file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Patching apiserver")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Checking openshift-controller-manager operator should be in Progressing in 100 seconds")
		expected_status := map[string]string{"Progressing": "True"}
		err = waitCoBecomes(oc, "openshift-controller-manager", 100, expected_status) // Wait it to become Progressing=True
		exutil.AssertWaitPollNoErr(err, "openshift-controller-manager operator is not start progressing in 100 seconds")
		e2e.Logf("Checking openshift-controller-manager operator should be Available in 300 seconds")
		expected_status = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err = waitCoBecomes(oc, "openshift-controller-manager", 300, expected_status) // Wait it to become Available=True and Progressing=False and Degraded=False
		exutil.AssertWaitPollNoErr(err, "openshift-controller-manager operator is not becomes available in 300 seconds")

		g.By("Create the new kubeconfig")
		e2e.Logf("Add system:admin credentials, context to the kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-credentials", user, "--client-certificate="+user_cert+".crt", "--client-key="+user_cert+".key", "--embed-certs", "--kubeconfig="+new_kubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create context for the user")
		cluster_name, _ := oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "-o", `jsonpath={.clusters[0].name}`).Output()
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-context", user, "--cluster="+cluster_name, "--namespace=default", "--user="+user, "--kubeconfig="+new_kubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Extract certificate authority")
		podnames, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication", "-o", "name").Output()
		podname := strings.Fields(podnames)
		ingress_crt, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-authentication", podname[0], "cat", "/run/secrets/kubernetes.io/serviceaccount/ca.crt").OutputToFile("OCP-41899-ca/OCP-41899-ingress-ca.crt")
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Set certificate authority data")
		server_name, _ := oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "-o", `jsonpath={.clusters[0].cluster.server}`).Output()
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-cluster", cluster_name, "--server="+server_name, "--certificate-authority="+ingress_crt, "--kubeconfig="+new_kubeconfig, "--embed-certs").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Set current context")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("use-context", user, "--kubeconfig="+new_kubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test the new kubeconfig, be aware that the following command may requires some seconds for let the operator reconcile the newly added CA.
		g.By("Testing the new kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("login").Args("--kubeconfig", new_kubeconfig, "-u", user).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("--kubeconfig", new_kubeconfig, "node").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// If the previous commands are successful is possible to replace the default CA.
		e2e.Logf("Replace the default CA")
		configmap_yaml, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("--kubeconfig", new_kubeconfig, "configmap", "admin-kubeconfig-client-ca", "-n", "openshift-config", ca_file, "--dry-run=client", "-o", "yaml").OutputToFile("OCP-41899-ca/OCP-41899.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("--kubeconfig", new_kubeconfig, "-f", configmap_yaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Is now possible to remove the additional CA which we set earlier.
		e2e.Logf("Removing the additional CA")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("--kubeconfig", new_kubeconfig, "apiserver/cluster", "--type=json", "-p", patch_to_recover).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Now the old kubeconfig should be invalid, the following command is expected to fail (make sure to set the proper kubeconfig path).
		e2e.Logf("Testing old kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("use-context", "admin").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(5*time.Second, 100*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
			if strings.Contains(string(output), "Unauthorized") {
				e2e.Logf("Test pass: Old kubeconfig not working!")
				// Adding wait time to ensure new kubeconfig work properly
				time.Sleep(60 * time.Second)
				return true, nil
			} else if err == nil {
				e2e.Logf("Still Old kubeconfig is working!")
				return false, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Test failed: Old kubeconfig is working!")
	})
	// author: rgangwar@redhat.com
	g.It("Author:rgangwar-Medium-43889-Examine non critical kube-apiserver errors", func() {
		var (
			keywords          =    "(error|fail|tcp dial timeout|connect: connection refused|Unable to connect to the server: dial tcp|remote error: tls: bad certificate)"
			exceptions        =    "panic|fatal|SHOULD NOT HAPPEN"
			format            =    "[0-9TZ.:]{2,30}"
			words             =    `(\w+?[^0-9a-zA-Z]+?){,5}`
			afterwords        =    `(\w+?[^0-9a-zA-Z]+?){,12}`
			co                =    "openshift-kube-apiserver-operator"
			dirname           =    "/tmp/-OCP-43889/"
			regex_to_grep_1   =    "("+words+keywords+words+")"+"+"
			regex_to_grep_2   =    "("+words+keywords+afterwords+")"+"+"
		)

		defer os.RemoveAll(dirname)
		err := os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check the log files of KAS operator")
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", co, "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podlog, errlog := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", co, podname).OutputToFile("OCP-43889/kas-o-grep.log")
		o.Expect(errlog).NotTo(o.HaveOccurred())
		cmd := fmt.Sprintf(`cat %v |grep -ohiE '%s' |grep -iEv '%s' | sed -E 's/%s/../g' | sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, podlog, regex_to_grep_1, exceptions, format)
		kas_o_log, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s", kas_o_log)

		g.By("Check the log files of KAS")
		master_node, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
                o.Expect(err).NotTo(o.HaveOccurred())
                master_name := strings.Fields(master_node)
                cmd = fmt.Sprintf(`grep -rohiE '%s' |grep -iEv '%s' /var/log/pods/openshift-kube-apiserver_kube-apiserver*/*/* | sed -E 's/%s/../g'`, regex_to_grep_2, exceptions, format)
		for i := 0; i < len(master_name); i++ {
			_, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", "default", "node/"+master_name[i], "--", "chroot", "/host", "bash", "-c", cmd).OutputToFile("OCP-43889/kas_pod.log."+master_name[i])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v| sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, dirname+"kas_pod.log.*")
		kas_podlogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
                e2e.Logf("%s", kas_podlogs)

		g.By("Check the audit log files of KAS")
		cmd = fmt.Sprintf(`grep -rohiE '%s' /var/log/kube-apiserver/audit*.log |grep -iEv '%s' | sed -E 's/%s/../g'`, regex_to_grep_2, exceptions, format)
		for i := 0; i < len(master_name); i++ {
			_, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", "default", "node/"+master_name[i], "--", "chroot", "/host", "bash", "-c", cmd).OutputToFile("OCP-43889/kas_audit.log."+master_name[i])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v| sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, dirname+"kas_audit.log.*")
		kas_auditlogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s", kas_auditlogs)

		g.By("Checking pod and audit logs")
		if len(kas_o_log) > 0 || len(kas_podlogs) > 0 || len(kas_auditlogs) > 0 {
			e2e.Failf("Found some non-critical-errors....Check non critical errors, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("Test pass: No errors found from KAS operator, KAS logs/audit logs")
		}
	})
})
