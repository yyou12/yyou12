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

var _ = g.Describe("[sig-auth] Apiserver_and_Auth", func() {
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

	g.It("Author:pmali-High-33390-Network Stability check every level of a managed route [Disruptive]", func() {
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

})
