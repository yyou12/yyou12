package etcd

import (
	o "github.com/onsi/gomega"

	"strings"
	"time"
	"math/rand"
	"regexp"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)


func getRandomString() string {
        chars := "abcdefghijklmnopqrstuvwxyz0123456789"
        seed := rand.New(rand.NewSource(time.Now().UnixNano()))
        buffer := make([]byte, 8)
        for index := range buffer {
                buffer[index] = chars[seed.Intn(len(chars))]
        }
        return string(buffer)
}

func getNodeListByLabel(oc *exutil.CLI, labelKey string) []string {
        output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", labelKey, "-o=jsonpath={.items[*].metadata.name}").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        nodeNameList := strings.Fields(output)
        return nodeNameList
}

func runDRBackup(oc *exutil.CLI, nodeNameList []string) (nodeName string, etcddb string) {
	var nodeN,etcdDb string
	for nodeindex, node := range nodeNameList {
		backupout, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", oc.Namespace(), "node/"+node, "--", "chroot", "/host", "/usr/local/bin/cluster-backup.sh", "/home/core/assets/backup").Output()
		if strings.Contains(backupout, "Snapshot saved at") && err == nil {
			e2e.Logf("backup on master %v ", node)
			regexp,_ := regexp.Compile("/home/core/assets/backup/snapshot.*db")
			etcdDb = regexp.FindString(backupout)
			nodeN = node
			break
		} else if err != nil && nodeindex < len(nodeNameList)  {
			e2e.Logf("Try for next master!")
		} else {
			e2e.Failf("Failed to run the backup!")
		}
	}
	return nodeN,etcdDb
}
