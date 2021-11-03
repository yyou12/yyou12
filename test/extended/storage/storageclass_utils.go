package storage

import (
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

type storageClass struct {
	name              string
	template          string
	provisioner       string
	reclaimPolicy     string
	volumeBindingMode string
}

// function option mode to change the default value of storageclass parameters, e.g. name, provisioner, reclaimPolicy, volumeBindingMode
type storageClassOption func(*storageClass)

// Replace the default value of storageclass name parameter
func setStorageClassName(name string) storageClassOption {
	return func(this *storageClass) {
		this.name = name
	}
}

// Replace the default value of storageclass template parameter
func setStorageClassTemplate(template string) storageClassOption {
	return func(this *storageClass) {
		this.template = template
	}
}

// Replace the default value of storageclass provisioner parameter
func setStorageClassProvisioner(provisioner string) storageClassOption {
	return func(this *storageClass) {
		this.provisioner = provisioner
	}
}

// Replace the default value of storageclass reclaimPolicy parameter
func setStorageClassReclaimPolicy(reclaimPolicy string) storageClassOption {
	return func(this *storageClass) {
		this.reclaimPolicy = reclaimPolicy
	}
}

// Replace the default value of storageclass volumeBindingMode parameter
func setStorageClassVolumeBindingMode(volumeBindingMode string) storageClassOption {
	return func(this *storageClass) {
		this.volumeBindingMode = volumeBindingMode
	}
}

//  Create a new customized storageclass object
func newStorageClass(opts ...storageClassOption) storageClass {
	defaultStorageClass := storageClass{
		name:              "mystorageclass-" + getRandomString(),
		template:          "storageclass-template.yaml",
		provisioner:       "ebs.csi.aws.com",
		reclaimPolicy:     "Delete",
		volumeBindingMode: "WaitForFirstConsumer",
	}

	for _, o := range opts {
		o(&defaultStorageClass)
	}

	return defaultStorageClass
}

//  Create a new customized storageclass
func (sc *storageClass) create(oc *exutil.CLI) {
	err := applyResourceFromTemplateAsAdmin(oc, "--ignore-unknown-parameters=true", "-f", sc.template, "-p", "SCNAME="+sc.name, "RECLAIMPOLICY="+sc.reclaimPolicy,
		"PROVISIONER="+sc.provisioner, "VOLUMEBINDINGMODE="+sc.volumeBindingMode)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete Specified storageclass
func (sc *storageClass) deleteAsAdmin(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("sc", sc.name).Execute()
}

//  Create a new customized storageclass with
func (sc *storageClass) createWithExtraParameters(oc *exutil.CLI, extraParameters map[string]interface{}) {
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", sc.template, "-p",
		"SCNAME="+sc.name, "RECLAIMPOLICY="+sc.reclaimPolicy, "PROVISIONER="+sc.provisioner, "VOLUMEBINDINGMODE="+sc.volumeBindingMode)
	o.Expect(err).NotTo(o.HaveOccurred())
}
