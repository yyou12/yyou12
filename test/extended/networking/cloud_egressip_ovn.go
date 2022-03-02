package networking

import (
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	ci "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var (
		ipEchoUrl       string
		a               *exutil.Aws_client
		egressNodeLabel = "k8s.ovn.org/egress-assignable"

		oc = exutil.NewCLI("networking-"+getRandomString(), exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		platform := ci.CheckPlatform(oc)
		networkType := checkNetworkType(oc)
		e2e.Logf("\n\nThe platform is %v,  networkType is %v\n", platform, networkType)
		acceptedPlatform := strings.Contains(platform, "aws") || strings.Contains(platform, "gcp")
		if !acceptedPlatform || !strings.Contains(networkType, "ovn") {
			g.Skip("Test cases should be run on AWS or GCP cluster with ovn network plugin, skip for other platforms or other network plugin!!")
		}

		switch platform {
		case "aws":
			e2e.Logf("\n AWS is detected, running the case on AWS\n")
			if ipEchoUrl == "" {
				getAwsCredentialFromCluster(oc)
				a = exutil.InitAwsSession()
				_, err := getAwsIntSvcInstanceID(a, oc)
				if err != nil {
					e2e.Logf("There is no int svc instance in this cluster, %v", err)
					g.Skip("There is no int svc instance in this cluster, skip the cases!!")
				}

				ipEchoUrl, err = installIpEchoServiceOnAWS(a, oc)
				if err != nil {
					e2e.Logf("No ip-echo service installed on the bastion host, %v", err)
					g.Skip("No ip-echo service installed on the bastion host, skip the cases!!")
				}
			}
		case "gcp":
			e2e.Logf("\n GCP is detected, running the case on GCP\n")
			if ipEchoUrl == "" {
				// If an int-svc instance with external IP found, IpEcho service will be installed on the int-svc instance
				// otherwise, just give error message and skip the test
				infraId, err := exutil.GetInfraId(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				host, err := getIntSvcExternalIpFromGcp(oc, infraId)
				if err != nil {
					e2e.Logf("There is no int svc instance in this cluster, %v", err)
					g.Skip("There is no int svc instance in this cluster, skip the cases!!")
				}
				ipEchoUrl, err = installIpEchoServiceOnGCP(oc, infraId, host)
				if err != nil {
					e2e.Logf("No ip-echo service installed on the bastion host, %v", err)
					g.Skip("No ip-echo service installed on the bastion host, skip the cases!!")
				}
			}
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}

	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-High-47163-Deleting EgressIP object and recreating it works. [Serial]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1.yaml")

		g.By("create new namespace")
		oc.SetupProject()

		g.By("Label EgressIP node")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Apply EgressLabel Key for this test on one node.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel)

		g.By("Apply label to namespace")
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name=test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name-").Output()

		g.By("Create an egressip object")
		sub1 := getIfaddrFromNode(nodeList.Items[0].Name, oc)
		freeIps := findUnUsedIPsOnNode(oc, nodeList.Items[0].Name, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-47163",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("Create a pod ")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: oc.Namespace(),
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)
		waitPodReady(oc, pod1.namespace, pod1.name)
		defer pod1.deletePingPod(oc)

		g.By("Check source IP is EgressIP")
		e2e.Logf("\n ipEchoUrl is %v\n", ipEchoUrl)
		sourceIp, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoUrl+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIp).Should(o.BeElementOf(freeIps))

		g.By("Deleting and recreating egressip object")
		egressip1.deleteEgressIPObject1(oc)
		egressip1.createEgressIPObject1(oc)

		g.By("Check source IP is EgressIP")
		wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			sourceIp, err = e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoUrl+" --connect-timeout 5")
			if !contains(freeIps, sourceIp) {
				eip, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("egressip", "-o=jsonpath={.}").Output()
				e2e.Logf(eip)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIp).Should(o.BeElementOf(freeIps))

		g.By("Deleting EgressIP object and recreating it works!!! ")
	})

})
