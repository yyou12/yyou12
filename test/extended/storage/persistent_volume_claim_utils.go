package storage

import (
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type persistentVolumeClaim struct {
	name       string
	namespace  string
	scname     string
	template   string
	volumemode string
	accessmode string
	capacity   string
}

// function option mode to change the default values of PersistentVolumeClaim parameters, e.g. name, namespace, accessmode, capacity, volumemode etc.
type persistentVolumeClaimOption func(*persistentVolumeClaim)

// Replace the default value of PersistentVolumeClaim name parameter
func setPersistentVolumeClaimName(name string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.name = name
	}
}

// Replace the default value of PersistentVolumeClaim template parameter
func setPersistentVolumeClaimTemplate(template string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.template = template
	}
}

// Replace the default value of PersistentVolumeClaim namespace parameter
func setPersistentVolumeClaimNamespace(namespace string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.namespace = namespace
	}
}

// Replace the default value of PersistentVolumeClaim accessmode parameter
func setPersistentVolumeClaimAccessmode(accessmode string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.accessmode = accessmode
	}
}

// Replace the default value of PersistentVolumeClaim scname parameter
func setPersistentVolumeClaimStorageClassName(scname string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.scname = scname
	}
}

// Replace the default value of PersistentVolumeClaim capacity parameter
func setPersistentVolumeClaimCapacity(capacity string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.capacity = capacity
	}
}

// Replace the default value of PersistentVolumeClaim volumemode parameter
func setPersistentVolumeClaimVolumemode(volumemode string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.volumemode = volumemode
	}
}

//  Create a new customized PersistentVolumeClaim object
func newPersistentVolumeClaim(opts ...persistentVolumeClaimOption) persistentVolumeClaim {
	defaultPersistentVolumeClaim := persistentVolumeClaim{
		name:       "my-pvc-" + getRandomString(),
		template:   "pvc-template.yaml",
		namespace:  "default",
		capacity:   "10Gi",
		volumemode: "Filesystem",
		scname:     "gp2-csi",
		accessmode: "ReadWriteOnce",
	}

	for _, o := range opts {
		o(&defaultPersistentVolumeClaim)
	}

	return defaultPersistentVolumeClaim
}

// Create new PersistentVolumeClaim with customized parameters
func (pvc *persistentVolumeClaim) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pvc.template, "-p", "PVCNAME="+pvc.name, "PVCNAMESPACE="+pvc.namespace, "SCNAME="+pvc.scname,
		"ACCESSMODE="+pvc.accessmode, "VOLUMEMODE="+pvc.volumemode, "PVCCAPACITY="+pvc.capacity)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete specified PersistentVolumeClaim
func (pvc *persistentVolumeClaim) delete(oc *exutil.CLI) {
	oc.WithoutNamespace().Run("delete").Args("pvc", pvc.name, "-n", pvc.namespace).Execute()
}

//  Get specified PersistentVolumeClaim status
func getPersistentVolumeClaimStatus(oc *exutil.CLI, namespace string, pvcName string) (string, error) {
	pvcStatus, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", namespace, pvcName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s status in namespace %s is %q", pvcName, namespace, pvcStatus)
	return pvcStatus, err
}
