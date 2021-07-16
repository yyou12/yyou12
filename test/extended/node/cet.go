package node

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	//e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] Node Container Engine Tools crio,scc", func() {
	defer g.GinkgoRecover()

	var (
		oc				= exutil.NewCLI("node-"+getRandomString(), exutil.KubeConfigPath())
		buildPruningBaseDir		= exutil.FixturePath("testdata", "node")
		customTemp			= filepath.Join(buildPruningBaseDir, "pod-modify.yaml")
		customctrcfgTemp		= filepath.Join(buildPruningBaseDir, "containerRuntimeConfig.yaml")

		podModify = podModifyDescription{
			name:            "",
			namespace:       "",
			mountpath:	 "",
			command:         "",
			args:            "",
			restartPolicy:   "",
			user:            "",
			role:            "",
			level:           "",
			template:        customTemp,
		}

		ctrcfg = ctrcfgDescription{
			loglevel:	"",
			overlay:	"",
			logsizemax:	"",
			command:	"",
			configFile:	"",
			template:        customctrcfgTemp,
		}
	)

		// author: pmali@redhat.com
		g.It("Author:pmali-Medium-13117-SeLinuxOptions in pod should apply to container correctly", func() {

			oc.SetupProject()

			podModify.name		    = "hello-pod"
			podModify.namespace	    = oc.Namespace()
			podModify.mountpath	    = "/init-test"
			podModify.command	    = "/bin/bash"
			podModify.args		    = "sleep 30"
			podModify.restartPolicy	    = "Always"
			podModify.user              = "unconfined_u"
			podModify.role              = "unconfined_r"
			podModify.level             = "s0:c25,c968"

			g.By("Create a pod with selinux options\n")
			podModify.create(oc)
			g.By("Check pod status\n")
			err := podStatus(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Check Container SCC Status\n")
			err = ContainerSccStatus(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Delete Pod\n")
			podModify.delete(oc)
		})

		// author: pmali@redhat.com
		g.It("Longduration-Author:pmali-Medium-22093-CRIO configuration can be modified via containerruntimeconfig CRD[Disruptive][Slow]", func() {

			oc.SetupProject()
			ctrcfg.loglevel     = "debug"
			ctrcfg.overlay      = "2G"
			ctrcfg.logsizemax   = "-1"

			g.By("Create Container Runtime Config \n")
			ctrcfg.create(oc)
			defer cleanupObjectsClusterScope(oc, objectTableRefcscope{"ContainerRuntimeConfig", "parameter-testing"})
			g.By("Verify that the settings were applied in CRI-O\n")
			err := ctrcfg.checkCtrcfgParameters(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
