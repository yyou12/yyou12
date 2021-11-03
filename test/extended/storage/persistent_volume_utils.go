package storage

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
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
