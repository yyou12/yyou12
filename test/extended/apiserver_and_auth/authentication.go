package apiserver_and_auth

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-auth] Authentication", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: xxia@redhat.com
	// It is destructive case, will make co/authentical Available=False for a while, so adding [Disruptive]
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-Author:xxia-Medium-29917-Deleted authentication resources can come back immediately [Disruptive]", func() {
		g.By("Delete namespace openshift-authentication")
		err := oc.WithoutNamespace().Run("delete").Args("ns", "openshift-authentication").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Waiting for the namespace back, it should be back immediate enough. If it is not back immediately, it is bug")
		err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("ns", "openshift-authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get namespace openshift-authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("openshift-authentication.*Active", output); matched {
				e2e.Logf("Namespace is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "openshift-authentication is not back")

		g.By("Waiting for oauth-openshift pods back")
		// It needs some time to wait for pods recreated and Running, so the Poll parameters are a little larger
		err = wait.Poll(15*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get pods under openshift-authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("oauth-openshift.*Running", output); matched {
				e2e.Logf("Pods are back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod of openshift-authentication is not back")

		g.By("Waiting for the clusteroperator back to normal")
		// It needs more time to wait for clusteroperator back to normal. In test, the max time observed is up to 4 mins, so the Poll parameters are larger
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("co", "authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator authentication is back to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator authentication is not back to normal")

		g.By("Delete authentication.operator cluster")
		err = oc.WithoutNamespace().Run("delete").Args("authentication.operator", "cluster").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Waiting for authentication.operator back")
		// It needs more time to wait for authentication.operator back. In test, the max time observed is up to 4 mins, so the Poll parameters are larger
		err = wait.Poll(30*time.Second, 360*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("authentication.operator", "--no-headers").Output()
			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("^cluster ", output); matched {
				e2e.Logf("authentication.operator cluster is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "authentication.operator cluster is not back")

		g.By("Delete project openshift-authentication-operator")
		err = oc.WithoutNamespace().Run("delete").Args("project", "openshift-authentication-operator").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Waiting for project openshift-authentication-operator back")
		// It needs more time to wait for project openshift-authentication-operator back. In test, the max time observed is up to 6 mins, so the Poll parameters are larger
		err = wait.Poll(30*time.Second, 480*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("project", "openshift-authentication-operator").Output()
			if err != nil {
				e2e.Logf("Fail to get project openshift-authentication-operator, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("openshift-authentication-operator.*Active", output); matched {
				e2e.Logf("project openshift-authentication-operator is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "project openshift-authentication-operator  is not back")

		g.By("Waiting for the authentication-operator pod back")
		// It needs some time to wait for pods recreated and Running, so the Poll parameters are a little larger
		err = wait.Poll(15*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication-operator").Output()
			if err != nil {
				e2e.Logf("Fail to get pod under openshift-authentication-operator, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("authentication-operator.*Running", output); matched {
				e2e.Logf("Pod is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod of  openshift-authentication-operator is not back")
	})

	// author: pmali@redhat.com
	// It is destructive case, will make co/authentical Available=False for a while, so adding [Disruptive]

	g.It("Author:pmali-High-33390-Network Stability check every level of a managed route [Disruptive] [Flaky]", func() {
		g.By("Check pods under openshift-authentication namespace is available")
		err := wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get pods under openshift-authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("oauth-openshift.*Running", output); matched {
				e2e.Logf("Pods are in Running state:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod of openshift-authentication is not Running")

		// Check authentication operator, If its UP and running that means route and service is also working properly. No need to check seperately Service and route endpoints.
		g.By("Check authentication operator is available")
		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "authentication", "-o=jsonpath={.status.conditions[0].status}").Output()
			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("False", output); matched {
				e2e.Logf("authentication.operator cluster is UP:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "authentication.operator cluster is not UP")

		//Check service endpoint is showing correct error

		buildPruningBaseDir := exutil.FixturePath("testdata", "apiserver_and_auth")

		g.By("Check service endpoint is showing correct error")
		networkPolicyAllow := filepath.Join(buildPruningBaseDir, "allow-same-namespace.yaml")

		g.By("Create AllowNetworkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift-authentication", "-f"+networkPolicyAllow).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyAllow).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check authentication operator after allow network policy change
		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "authentication", "-o=jsonpath={.status.conditions[0].message}").Output()

			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if strings.Contains(output, "OAuthServiceEndpointsCheckEndpointAccessibleControllerDegraded") {
				e2e.Logf("Allow network policy applied successfully:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Allow network policy applied failure")

		g.By("Delete allow-same-namespace Networkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyAllow).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		//Deny all trafic for route
		g.By("Check route is showing correct error")

		networkPolicyDeny := filepath.Join(buildPruningBaseDir, "deny-network-policy.yaml")

		g.By("Create Deny-all Networkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift-authentication", "-f="+networkPolicyDeny).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyDeny).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check authentication operator after network policy change
		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "authentication", "-o=jsonpath={.status.conditions[0].message}").Output()

			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if strings.Contains(output, "OAuthRouteCheckEndpointAccessibleControllerDegraded") {
				e2e.Logf("Deny network policy applied:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Deny network policy not applied")

		g.By("Delete Networkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyDeny).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: ytripath@redhat.com
	g.It("NonPreRelease-Longduration-Author:ytripath-Medium-20804-Support ConfigMap injection controller [Disruptive] [Slow]", func() {
		oc.SetupProject()

		// Check the pod service-ca is running in namespace openshift-service-ca
		podDetails, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		matched, _ := regexp.MatchString("service-ca-.*Running", podDetails)
		o.Expect(matched).Should(o.Equal(true))

		// Create a configmap --from-literal and annotating it with service.beta.openshift.io/inject-cabundle=true
		err = oc.Run("create").Args("configmap", "my-config", "--from-literal=key1=config1", "--from-literal=key2=config2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("annotate").Args("configmap", "my-config", "service.beta.openshift.io/inject-cabundle=true").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for service-ca.crt to be created in configmap
		err = wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			output, err := oc.Run("get").Args("configmap", "my-config", `-o=json`).Output()
			if err != nil {
				e2e.Logf("Failed to get configmap, error: %s. Trying again", err)
				return false, nil
			}
			if strings.Contains(output, "service-ca.crt") {
				e2e.Logf("service-ca injected into configmap successfully\n")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "service-ca.crt not found in configmap")

		oldCert, err := oc.Run("get").Args("configmap", "my-config", `-o=jsonpath={.data.service-ca\.crt}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Delete secret signing-key in openshift-service-ca project
		podOldUID, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca", "-o=jsonpath={.items[0].metadata.uid}").Output()
		err = oc.AsAdmin().Run("delete").WithoutNamespace().Args("-n", "openshift-service-ca", "secret", "signing-key").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			// sleep for 200 seconds to make sure the pod is restarted
			time.Sleep(200 * time.Second)
			var podStatus string
			err := wait.Poll(15*time.Second, 15*time.Minute, func() (bool, error) {
				e2e.Logf("Check if all pods are in Completed or Running state across all namespaces")
				podStatus, err = oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-A", `--field-selector=metadata.namespace!=openshift-kube-apiserver,status.phase==Pending`).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf(podStatus)
				if podStatus == "No resources found" {
					// Sleep for 100 seconds then double-check if all pods are up and running
					time.Sleep(100 * time.Second)
					podStatus, err = oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-A", `--field-selector=metadata.namespace!=openshift-kube-apiserver,status.phase==Pending`).Output()
					if err == nil {
						if podStatus == "No resources found" {
							e2e.Logf("No pending pods found")
							return true, nil
						}
						return false, err
					}
				}
				return false, err
			})
			exutil.AssertWaitPollNoErr(err, "These pods are still not back up after waiting for 15 minutes\n"+podStatus)
		}()

		// Waiting for the pod to be Ready, after several minutes(10 min ?) check the cert data in the configmap
		g.By("Waiting for service-ca to be ready, then check if cert data is updated")
		err = wait.Poll(15*time.Second, 5*time.Minute, func() (bool, error) {
			podStatus, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca", `-o=jsonpath={.items[0].status.containerStatuses[0].ready}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			podUID, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca", "-o=jsonpath={.items[0].metadata.uid}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if podStatus == `true` && podOldUID != podUID {
				// We need use AsAdmin() otherwise it will frequently hit "error: You must be logged in to the server (Unauthorized)"
				// before the affected components finish pod restart after the secret deletion, like kube-apiserver, oauth-apiserver etc.
				// Still researching if this is a bug
				newCert, _ := oc.AsAdmin().Run("get").Args("configmap", "my-config", `-o=jsonpath={.data.service-ca\.crt}`).Output()
				matched, _ := regexp.MatchString(oldCert, newCert)
				if !matched {
					g.By("Cert data has been updated")
					return true, nil
				}
			}
			return false, err
		})
		exutil.AssertWaitPollNoErr(err, "Cert data not updated after waiting for 5 mins")
	})
})
