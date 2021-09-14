package node

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	//e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] Node initContainer policy,volume,readines,quota", func() {
	defer g.GinkgoRecover()

	var (
		oc                  = exutil.NewCLI("node-"+getRandomString(), exutil.KubeConfigPath())
		buildPruningBaseDir = exutil.FixturePath("testdata", "node")
		customTemp          = filepath.Join(buildPruningBaseDir, "pod-modify.yaml")

		podModify = podModifyDescription{
			name:          "",
			namespace:     "",
			mountpath:     "",
			command:       "",
			args:          "",
			restartPolicy: "",
			user:          "",
			role:          "",
			level:         "",
			template:      customTemp,
		}
	)

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12893-Init containers with restart policy Always", func() {
		oc.SetupProject()
		podModify.name = "init-always-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "Always"

		g.By("create FAILED init container with pod restartPolicy Always")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain CrashLoopBackOff")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy Always")

		podModify.name = "init-always-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Always"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12894-Init containers with restart policy OnFailure", func() {
		oc.SetupProject()
		podModify.name = "init-onfailure-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "OnFailure"

		g.By("create FAILED init container with pod restartPolicy OnFailure")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain CrashLoopBackOff")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy OnFailure")

		podModify.name = "init-onfailure-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "OnFailure"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12896-Init containers with restart policy Never", func() {
		oc.SetupProject()
		podModify.name = "init-never-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "Never"

		g.By("create FAILED init container with pod restartPolicy Never")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusterminatedReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain Error")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy Never")

		podModify.name = "init-never-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12911-App container status depends on init containers exit code	", func() {
		oc.SetupProject()
		podModify.name = "init-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/false"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		g.By("create FAILED init container with exit code and command /bin/false")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusterminatedReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain Error")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with command /bin/true")
		podModify.name = "init-success"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/true"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12913-Init containers with volume work fine", func() {

		oc.SetupProject()
		podModify.name = "init-volume"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "echo This is OCP volume test > /work-dir/volume-test"
		podModify.restartPolicy = "Never"

		g.By("Create a pod with initContainer using volume\n")
		podModify.create(oc)
		g.By("Check pod status")
		err := podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Check Vol status\n")
		err = volStatus(oc)
		exutil.AssertWaitPollNoErr(err, "Init containers with volume do not work fine")
		g.By("Delete Pod\n")
		podModify.delete(oc)
	})
})
