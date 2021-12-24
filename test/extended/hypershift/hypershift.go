package hypershift

import (
	"encoding/base64"
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"os"
	"strings"
)

var _ = g.Describe("[sig-hypershift] Hypershift", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("hypershift", exutil.KubeConfigPath())
	var guestClusterName, guestClusterNamespace string

	g.BeforeEach(func() {
		operator := doOcpReq(oc, OcpGet, false, []string{"pods", "-n", "hypershift", "-ojsonpath={.items[*].metadata.name}"})
		if len(operator) <= 0 {
			g.Skip("hypershift operator not found, skip test run")
		}

		clusterNames := doOcpReq(oc, OcpGet, false,
			[]string{"-n", "clusters", "hostedcluster", "-o=jsonpath={.items[*].metadata.name}"})
		if len(clusterNames) <= 0 {
			g.Skip("hypershift guest cluster not found, skip test run")
		}

		//get first guest cluster to run test
		guestClusterName = strings.Split(clusterNames, " ")[0]
		guestClusterNamespace = "clusters-" + guestClusterName

		res := doOcpReq(oc, OcpGet, true,
			[]string{"-n", "hypershift", "pod", "-o=jsonpath={.items[0].status.phase}"})
		checkSubstring(res, []string{"Running"})
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-42855-Check Status Conditions for HostedControlPlane", func() {
		g.By("hypershift OCP-42855 check hostedcontrolplane condition status")

		res := doOcpReq(oc, OcpGet, true,
			[]string{"-n", guestClusterNamespace, "hostedcontrolplane", guestClusterName,
				"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}"})
		checkSubstring(res,
			[]string{"ValidHostedControlPlaneConfiguration True",
				"EtcdAvailable True", "KubeAPIServerAvailable True", "InfrastructureReady True"})
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-43555-Allow direct ingress on guest clusters on AWS", func() {
		g.By("hypershift OCP-43555 allow direct ingress on guest cluster")
		guestClusterKubeconfigFile := "guestcluster-kubeconfig-43555"

		var bashClient = NewCmdClient()
		defer func() {
			os.Remove(guestClusterKubeconfigFile)
		}()
		_, err := bashClient.Run(fmt.Sprintf("hypershift create kubeconfig --name %s > %s", guestClusterName, guestClusterKubeconfigFile)).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		res := doOcpReq(oc, OcpGet, true,
			[]string{"clusteroperator", "kube-apiserver", fmt.Sprintf("--kubeconfig=%s", guestClusterKubeconfigFile),
				"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}"})
		checkSubstring(res, []string{"Degraded False"})

		ingressDomain := doOcpReq(oc, OcpGet, true,
			[]string{"-n", "openshift-ingress-operator", "ingresscontrollers", "-ojsonpath={.items[0].spec.domain}",
				fmt.Sprintf("--kubeconfig=%s", guestClusterKubeconfigFile)})
		e2e.Logf("The guest cluster ingress domain is : %s\n", ingressDomain)

		console := doOcpReq(oc, OcpWhoami, true,
			[]string{fmt.Sprintf("--kubeconfig=%s", guestClusterKubeconfigFile), "--show-console"})

		pwdbase64 := doOcpReq(oc, OcpGet, true,
			[]string{"-n", guestClusterNamespace, "secret", "kubeadmin-password", "-ojsonpath={.data.password}"})
		pwd, err := base64.StdEncoding.DecodeString(pwdbase64)
		o.Expect(err).ShouldNot(o.HaveOccurred())

		parms := fmt.Sprintf("curl -u admin:%s %s  -k  -LIs -o /dev/null -w %s ", string(pwd), console, "%{http_code}")
		res, err = bashClient.Run(parms).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		checkSubstring(res, []string{"200"})
	})
})
