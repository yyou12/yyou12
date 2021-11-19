package storage

import (
	"fmt"
	"time"

	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
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

//  Delete the PersistentVolumeClaim
func (pvc *persistentVolumeClaim) delete(oc *exutil.CLI) {
	err := oc.WithoutNamespace().Run("delete").Args("pvc", pvc.name, "-n", pvc.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the PersistentVolumeClaim use kubeadmin
func (pvc *persistentVolumeClaim) deleteAsAdmin(oc *exutil.CLI) {
	oc.WithoutNamespace().AsAdmin().Run("delete").Args("pvc", pvc.name, "-n", pvc.namespace).Execute()
}

//  Get the PersistentVolumeClaim status
func (pvc *persistentVolumeClaim) getStatus(oc *exutil.CLI) (string, error) {
	pvcStatus, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", pvc.namespace, pvc.name, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s status in namespace %s is %q", pvc.name, pvc.namespace, pvcStatus)
	return pvcStatus, err
}

// Get the PersistentVolumeClaim bounded  PersistentVolume's volumeid
func (pvc *persistentVolumeClaim) getVolumeName(oc *exutil.CLI) string {
	pvName, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", pvc.namespace, pvc.name, "-o=jsonpath={.spec.volumeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s in namespace %s Bound pv is %q", pvc.name, pvc.namespace, pvName)
	return pvName
}

// Get the PersistentVolumeClaim bounded  PersistentVolume's volumeid
func (pvc *persistentVolumeClaim) getVolumeId(oc *exutil.CLI) string {
	pvName := pvc.getVolumeName(oc)
	volumeId, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeHandle}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PV %s volumeid is %q", pvName, volumeId)
	return volumeId
}

//  Get specified PersistentVolumeClaim status
func getPersistentVolumeClaimStatus(oc *exutil.CLI, namespace string, pvcName string) (string, error) {
	pvcStatus, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", namespace, pvcName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s status in namespace %s is %q", pvcName, namespace, pvcStatus)
	return pvcStatus, err
}

//  Get specified PersistentVolumeClaim status type during Resize
func getPersistentVolumeClaimStatusType(oc *exutil.CLI, namespace string, pvcName string) (string, error) {
	pvcStatus, err := oc.WithoutNamespace().Run("get").Args("pvc", pvcName, "-n", namespace, "-o=jsonpath={.status.conditions[0].type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s status in namespace %s is %q", pvcName, namespace, pvcStatus)
	return pvcStatus, err
}

// Apply the patch to Resize volume
func applyVolumeResizePatch(oc *exutil.CLI, pvcName string, namespace string, volumeSize string) (string, error) {
	//command1 := "{\"spec\":{\"resources\":{\"requests\":{\"storage\":\"" + volumeSize + "Gi\"}}}}"
	command1 := "{\"spec\":{\"resources\":{\"requests\":{\"storage\":\"" + volumeSize + "\"}}}}"
	command := []string{"pvc", pvcName, "-n", namespace, "-p", command1, "--type=merge"}
	e2e.Logf("The command is %s", command)
	msg, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(command...).Output()
	if err != nil {
		e2e.Logf("Execute command failed with err:%v .", err)
		return msg, err
	} else {
		e2e.Logf("The command executed successfully %s", command)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}

// Use persistent volume claim name to get the volumeSize in status.capacity
func getVolSizeFromPvc(oc *exutil.CLI, pvcName string, namespace string) (string, error) {
	volumeSize, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pvc", pvcName, "-n", namespace, "-o=jsonpath={.status.capacity.storage}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC %s volumesize is %s", pvcName, volumeSize)
	return volumeSize, err
}

// Wait for PVC Volume Size to get Resized
func (pvc *persistentVolumeClaim) waitResizeSuccess(oc *exutil.CLI, volResized string) {
	err := wait.Poll(15*time.Second, 120*time.Second, func() (bool, error) {
		status, err := getVolSizeFromPvc(oc, pvc.name, pvc.namespace)
		if err != nil {
			e2e.Logf("the err:%v, wait for volume Resize %v .", err, pvc.name)
			return false, err
		} else {
			if status == volResized {
				e2e.Logf("The volume size Reached to expected status:%v", status)
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, did not get Resized.", pvc.name))
}

// Wait for PVC Volume Size to match with Resizing status
func getPersistentVolumeClaimStatusMatch(oc *exutil.CLI, namespace string, pvcName string, expectedValue string) {
	err := wait.Poll(15*time.Second, 120*time.Second, func() (bool, error) {
		status, err := getPersistentVolumeClaimStatusType(oc, namespace, pvcName)
		if err != nil {
			e2e.Logf("the err:%v, to get volume status Type %v .", err, pvcName)
			return false, err
		} else {
			if status == expectedValue {
				e2e.Logf("The volume size Reached to expected status:%v", status)
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, did not reached expected status.", err))
}
