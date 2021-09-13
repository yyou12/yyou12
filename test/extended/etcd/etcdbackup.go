package etcd

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-etcd] ETCD", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Critical-42183-backup and restore should perform consistency checks on etcd snapshots", func() {
		g.By("Test for case OCP-42183 backup and restore should perform consistency checks on etcd snapshots")
		g.By("create new namespace")
		oc.SetupProject()

		g.By("select all the master node")
		masterNodeList := getNodeListByLabel(oc, "node-role.kubernetes.io/master=")

		g.By("Run the backup")
		masterN, etcdDb := runDRBackup(oc, masterNodeList)
		defer oc.AsAdmin().Run("debug").Args("-n", oc.Namespace(), "node/"+masterN, "--", "chroot", "/host", "rm", "-rf", "/home/core/assets/backup").Execute()

		g.By("Corrupt the etcd db file ")
		_, err := oc.AsAdmin().Run("debug").Args("-n", oc.Namespace(), "node/"+masterN, "--", "chroot", "/host", "truncate", "-s", "126k", etcdDb).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Run the restore")
		output, err := oc.AsAdmin().Run("debug").Args("-n", oc.Namespace(), "node/"+masterN, "--", "chroot", "/host", "/usr/local/bin/cluster-restore.sh", "/home/core/assets/backup").Output()
		o.Expect(err).Should(o.HaveOccurred())
		fmt.Sprintf("The output for restore is %v", output)
	})
})
