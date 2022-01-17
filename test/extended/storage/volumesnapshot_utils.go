package storage

import (
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type volumeSnapshot struct {
	name          string
	namespace     string
	vscname       string
	template      string
	sourcepvcname string
}

// function option mode to change the default values of VolumeSnapshot parameters, e.g. name, namespace, volumesnapshotclassname, source.pvcname etc.
type volumeSnapshotOption func(*volumeSnapshot)

// Replace the default value of VolumeSnapshot name parameter
func setVolumeSnapshotName(name string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.name = name
	}
}

// Replace the default value of VolumeSnapshot template parameter
func setVolumeSnapshotTemplate(template string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.template = template
	}
}

// Replace the default value of VolumeSnapshot namespace parameter
func setVolumeSnapshotNamespace(namespace string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.namespace = namespace
	}
}

// Replace the default value of VolumeSnapshot vsc parameter
func setVolumeSnapshotVscname(vscname string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.vscname = vscname
	}
}

// Replace the default value of VolumeSnapshot source.pvc parameter
func setVolumeSnapshotSourcepvcname(sourcepvcname string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.sourcepvcname = sourcepvcname
	}
}

//  Create a new customized VolumeSnapshot object
func newVolumeSnapshot(opts ...volumeSnapshotOption) volumeSnapshot {
	defaultVolumeSnapshot := volumeSnapshot{
		name:          "my-snapshot-" + getRandomString(),
		template:      "volumesnapshot-template.yaml",
		namespace:     "",
		vscname:       "volumesnapshotclass",
		sourcepvcname: "my-pvc",
	}

	for _, o := range opts {
		o(&defaultVolumeSnapshot)
	}

	return defaultVolumeSnapshot
}

// Create new VolumeSnapshot with customized parameters
func (vs *volumeSnapshot) create(oc *exutil.CLI) {
	if vs.namespace == "" {
		vs.namespace = oc.Namespace()
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", vs.template, "-p", "VSNAME="+vs.name, "VSNAMESPACE="+vs.namespace, "SOURCEPVCNAME="+vs.sourcepvcname, "VSCNAME="+vs.vscname)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the VolumeSnapshot
func (vs *volumeSnapshot) delete(oc *exutil.CLI) {
	oc.WithoutNamespace().Run("delete").Args("volumesnapshot", vs.name, "-n", vs.namespace).Execute()
}

//  Get the VolumeSnapshot ready_to_use status
func (vs *volumeSnapshot) getVsStatus(oc *exutil.CLI) (bool, error) {
	vsStatus, err := oc.WithoutNamespace().Run("get").Args("volumesnapshot", "-n", vs.namespace, vs.name, "-o=jsonpath={.status.readyToUse}").Output()
	e2e.Logf("The volumesnapshot %s ready_to_use status in namespace %s is %s", vs.name, vs.namespace, vsStatus)
	return strings.EqualFold("true", vsStatus), err
}

// Waiting the volumesnapshot to ready_to_use
func (vs *volumeSnapshot) waitReadyToUse(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		status, err1 := vs.getVsStatus(oc)
		if err1 != nil {
			e2e.Logf("The err:%v, wait for volumesnahotshot %v to become ready_to_use.", err1, vs.name)
			return status, err1
		}
		if !status {
			e2e.Logf("Waiting the volumesnahotshot %v in namespace %v to be ready_to_use.", vs.name, vs.namespace)
			return status, nil
		}
		e2e.Logf("The volumesnahotshot %v in namespace %v is ready_to_use.", vs.name, vs.namespace)
		return status, nil
	})

	if err != nil {
		vsDescribe := getOcDescribeInfo(oc, vs.namespace, "volumesnapshot", vs.name)
		e2e.Logf("oc describe volumesnapshot %s:\n%s", vs.name, vsDescribe)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Volumeshapshot %s is not ready_to_use", vs.name))
}
