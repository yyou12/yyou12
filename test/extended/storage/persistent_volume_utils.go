package storage

import (
	"fmt"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Use the bounded persistent volume claim name get the persistent volume name
func getPersistentVolumeNameByPersistentVolumeClaim(oc *exutil.CLI, namespace string, pvcName string) string {
	pvName, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", namespace, pvcName, "-o=jsonpath={.spec.volumeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s in namespace %s Bound pv is %q", pvcName, namespace, pvName)
	return pvName
}

// Get the persistent volume status
func getPersistentVolumeStatus(oc *exutil.CLI, namespace string, pvName string) string {
	pvStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", "-n", namespace, pvName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PV  %s status in namespace %s is %q", pvName, namespace, pvStatus)
	return pvStatus
}

// Use persistent volume name get the volumeid
func getVolumeIdByPersistentVolumeName(oc *exutil.CLI, pvName string) string {
	volumeId, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeHandle}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PV %s volumeid is %q", pvName, volumeId)
	return volumeId
}

// Use persistent volume claim name get the volumeid
func getVolumeIdByPersistentVolumeClaimName(oc *exutil.CLI, namespace string, pvcName string) string {
	pvName := getPersistentVolumeNameByPersistentVolumeClaim(oc, namespace, pvcName)
	return getVolumeIdByPersistentVolumeName(oc, pvName)
}

// Use persistent volume name to get the volumeSize
func getVolSizeFromPv(oc *exutil.CLI, pvcName string, namespace string) (string, error) {
	pvName := getPersistentVolumeNameByPersistentVolumeClaim(oc, namespace, pvcName)
	volumeSize, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.capacity.storage}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PV %s volumesize is %s", pvName, volumeSize)
	return volumeSize, err
}

// Wait for PVC Volume Size to get Resized
func waitPVVolSizeToGetResized(oc *exutil.CLI, namespace string, pvcName string, volResized string) {
	err := wait.Poll(15*time.Second, 120*time.Second, func() (bool, error) {
		status, err := getVolSizeFromPv(oc, pvcName, namespace)
		if err != nil {
			e2e.Logf("the err:%v, wait for volume Resize %v .", err, pvcName)
			return false, err
		} else {
			if status == volResized {
				e2e.Logf("The volume Resize reached to expect status %s", status)
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, did not get Resized.", pvcName))
}
