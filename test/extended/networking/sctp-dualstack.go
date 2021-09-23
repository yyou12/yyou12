package networking

import (
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-sctp", exutil.KubeConfigPath())

	// author: weliang@redhat.com
	g.It("Longduration-Author:weliang-Medium-28757-Establish pod to pod SCTP connections. [Disruptive]", func() {
        var (
		    buildPruningBaseDir  = exutil.FixturePath("testdata", "networking/sctp")
		    sctpClientPod        = filepath.Join(buildPruningBaseDir, "sctpclient.yaml")
			sctpServerPod        = filepath.Join(buildPruningBaseDir, "sctpserver.yaml")
			sctpModule           = filepath.Join(buildPruningBaseDir, "load-sctp-module.yaml")
			sctpServerPodName    = "sctpserver"
		    sctpClientPodname    = "sctpclient"
		)

		g.By("install load-sctp-module in all workers")
		installSctpModule(oc, sctpModule)

		g.By("check load-sctp-module in all workers")
		workerNodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			g.Skip("Can not find any woker nodes in the cluster")
		}
		for i := range workerNodeList.Items {
			checkSctpModule(oc, workerNodeList.Items[i].Name)
		}

		g.By("create new namespace")
                oc.SetupProject()

		g.By("create sctpClientPod")
		createResourceFromFile(oc, oc.Namespace(), sctpClientPod)
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "name=sctpclient")
		exutil.AssertWaitPollNoErr(err1, "sctpClientPod is not running")

		g.By("create sctpServerPod")
		createResourceFromFile(oc, oc.Namespace(), sctpServerPod)
		err2 := waitForPodWithLabelReady(oc, oc.Namespace(), "name=sctpserver")
		exutil.AssertWaitPollNoErr(err2, "sctpServerPod is not running")

		ipStackType := checkIpStackType(oc)

		g.By("test ipv4 in ipv4 cluster or dualstack cluster")
		if ipStackType == "ipv4single" || ipStackType == "dualstack" {
			g.By("get ipv4 address from the sctpServerPod")
			sctpServerPodIP := getPodIPv4(oc, oc.Namespace(), sctpServerPodName)
			
			g.By("sctpserver pod start to wait for sctp traffic")
			_,_,_, err := oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/nc", "-l", "30102",  "--sctp").Background()
			o.Expect(err).NotTo(o.HaveOccurred())
			time.Sleep(5 * time.Second)
			
			g.By("check sctp process enabled in the sctp server pod")
			msg, err := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/nc -l 30102 --sctp")).To(o.BeTrue())
			
			g.By("sctpclient pod start to send sctp traffic")
			_, err1 := e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "nc -v "+sctpServerPodIP+" 30102 --sctp <<< 'test-openshift'")
			o.Expect(err1).NotTo(o.HaveOccurred())

			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err1 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err1).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/nc -l 30102 --sctp"))
		}

		g.By("test ipv6 in ipv6 cluster or dualstack cluster")
		if ipStackType == "ipv6single" || ipStackType == "dualstack"{
			g.By("get ipv6 address from the sctpServerPod")
			sctpServerPodIP := getPodIPv6(oc, oc.Namespace(), sctpServerPodName, ipStackType)
			
			g.By("sctpserver pod start to wait for sctp traffic")
			oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/nc", "-l", "30102",  "--sctp").Background()
			time.Sleep(5 * time.Second)
			
			g.By("check sctp process enabled in the sctp server pod")
			msg, err := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/nc -l 30102 --sctp")).To(o.BeTrue())
			
			g.By("sctpclient pod start to send sctp traffic")
			e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "nc -v "+sctpServerPodIP+" 30102 --sctp <<< 'test-openshift'")
			
			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err1 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err1).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/nc -l 30102 --sctp"))
		}	
	})
})
