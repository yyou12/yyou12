package networking

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"net"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	ci "github.com/openshift/openshift-tests-private/test/extended/util/clusterinfrastructure"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Get AWS credential from cluster
func getAwsCredentialFromCluster(oc *exutil.CLI) {
	if ci.CheckPlatform(oc) != "aws" {
		g.Skip("it is not aws platform and can not get credential, and then skip it.")
	}
	credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			credential, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/ebs-cloud-credentials", "-n", "openshift-cluster-csi-drivers", "-o", "json").Output()
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	accessKeyIdBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).String(), gjson.Get(credential, `data.aws_secret_access_key`).String()
	accessKeyId, err1 := base64.StdEncoding.DecodeString(accessKeyIdBase64)
	o.Expect(err1).NotTo(o.HaveOccurred())
	secureKey, err2 := base64.StdEncoding.DecodeString(secureKeyBase64)
	o.Expect(err2).NotTo(o.HaveOccurred())
	clusterRegion, err3 := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
	o.Expect(err3).NotTo(o.HaveOccurred())
	os.Setenv("AWS_ACCESS_KEY_ID", string(accessKeyId))
	os.Setenv("AWS_SECRET_ACCESS_KEY", string(secureKey))
	os.Setenv("AWS_REGION", clusterRegion)

}

// Get AWS int svc instance ID
func getAwsIntSvcInstanceID(a *exutil.Aws_client, oc *exutil.CLI) (string, error) {
	clusterPrefixName := exutil.GetClusterPrefixName(oc)
	instanceName := clusterPrefixName + "-int-svc"
	instanceID, err := a.GetAwsInstanceID(instanceName)
	if err != nil {
		e2e.Logf("Get bastion instance id failed with error %v .", err)
		return "", err
	}
	return instanceID, nil
}

// Get int svc instance private ip and public ip
func getAwsIntSvcIPs(a *exutil.Aws_client, oc *exutil.CLI) map[string]string {
	instanceID, err := getAwsIntSvcInstanceID(a, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	ips, err := a.GetAwsIntIPs(instanceID)
	o.Expect(err).NotTo(o.HaveOccurred())
	return ips
}

//Update int svc instance ingress rule to allow destination port
func updateAwsIntSvcSecurityRule(a *exutil.Aws_client, oc *exutil.CLI, dstPort int64) {
	instanceID, err := getAwsIntSvcInstanceID(a, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = a.UpdateAwsIntSecurityRule(instanceID, dstPort)
	o.Expect(err).NotTo(o.HaveOccurred())

}

func installIpEchoServiceOnAWS(a *exutil.Aws_client, oc *exutil.CLI) (string, error) {
	user := os.Getenv("SSH_CLOUD_PRIV_AWS_USER")
	if user == "" {
		user = "ec2-user"
	}

	sshkey := os.Getenv("SSH_CLOUD_PRIV_KEY")
	if sshkey == "" {
		sshkey = "../internal/config/keys/openshift-qe.pem"
	}
	command := "sudo netstat -ntlp | grep 9095 || sudo podman run --name ipecho -d -p 9095:80 quay.io/openshifttest/ip-echo:multiarch"
	e2e.Logf("Run command", command)

	ips := getAwsIntSvcIPs(a, oc)
	publicIP, ok := ips["publicIP"]
	if !ok {
		return "", fmt.Errorf("No public IP found for Int Svc instance")
	}
	privateIP, ok := ips["privateIP"]
	if !ok {
		return "", fmt.Errorf("No private IP found for Int Svc instance")
	}

	sshClient := exutil.SshClient{User: user, Host: publicIP, Port: 22, PrivateKey: sshkey}
	err := sshClient.Run(command)
	if err != nil {
		e2e.Logf("Failed to run %v: %v", command, err)
		return "", err
	}

	updateAwsIntSvcSecurityRule(a, oc, 9095)

	ipEchoUrl := net.JoinHostPort(privateIP, "9095")
	return ipEchoUrl, nil
}

func getIfaddrFromNode(nodeName string, oc *exutil.CLI) string {
	egressIpconfig, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("node", nodeName, "-o=jsonpath={.metadata.annotations.cloud\\.network\\.openshift\\.io/egress-ipconfig}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The egressipconfig is %v", egressIpconfig)
	ifaddr := strings.Split(egressIpconfig, "\"")[9]
	e2e.Logf("The subnet of node %s is %v .", nodeName, ifaddr)
	return ifaddr
}

func findUnUsedIPsOnNode(oc *exutil.CLI, nodeName, cidr string, number int) []string {
	ipRange, _ := Hosts(cidr)
	var ipUnused = []string{}
	//shuffle the ips slice
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(ipRange), func(i, j int) { ipRange[i], ipRange[j] = ipRange[j], ipRange[i] })
	for _, ip := range ipRange {
		if len(ipUnused) < number {
			pingCmd := "ping -c4 -t1 " + ip
			msg, err := execCommandInOVNPodOnNode(oc, nodeName, pingCmd)
			if err != nil && strings.Contains(msg, "Destination Host Unreachable") {
				e2e.Logf("%s is not used!\n", ip)
				ipUnused = append(ipUnused, ip)
			} else if err != nil {
				break
			}
		} else {
			break
		}

	}
	return ipUnused
}

func execCommandInOVNPodOnNode(oc *exutil.CLI, nodeName, command string) (string, error) {
	ovnPodName, err := exutil.GetPodName(oc, "openshift-ovn-kubernetes", "app=ovnkube-node", nodeName)
	o.Expect(err).NotTo(o.HaveOccurred())
	msg, err := exutil.RemoteShPodWithBash(oc, "openshift-ovn-kubernetes", ovnPodName, command)
	if err != nil {
		e2e.Logf("Execute ovn command failed with  err:%v .", err)
		return msg, err
	}
	return msg, nil
}
