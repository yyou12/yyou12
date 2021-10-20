package storage

import (
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	o "github.com/onsi/gomega"
)

type deployment struct {
	name       string
	namespace  string
	replicasno string
	mpath      string
	pvcname    string
	template   string
}

// function option mode to change the default value of deployment parameters,eg. name, replicasno, mpath
type deployOption func(*deployment)

// Replace the default value of Deployment name parameter
func setDeploymentName(name string) deployOption {
	return func(this *deployment) {
		this.name = name
	}
}

// Replace the default value of Deployment template parameter
func setDeploymentTemplate(template string) deployOption {
	return func(this *deployment) {
		this.template = template
	}
}

// Replace the default value of Deployment namespace parameter
func setDeploymentNamespace(namespace string) deployOption {
	return func(this *deployment) {
		this.namespace = namespace
	}
}

// Replace the default value of Deployment replicasno parameter
func setDeploymentReplicasNumber(replicasno string) deployOption {
	return func(this *deployment) {
		this.replicasno = replicasno
	}
}

// Replace the default value of Deployment mountpath parameter
func setDeploymentMountpath(mpath string) deployOption {
	return func(this *deployment) {
		this.mpath = mpath
	}
}

// Replace the default value of Deployment pvcname parameter
func setDeploymentPVCName(pvcname string) deployOption {
	return func(this *deployment) {
		this.pvcname = pvcname
	}
}

//  Create a new customized Deployment object
func newDeployment(opts ...deployOption) deployment {
	defaultDeployment := deployment{
		name:       "my-dep-" + getRandomString(),
		template:   "dep-template.yaml",
		namespace:  "default",
		replicasno: "1",
		mpath:      "/mnt/storage",
		pvcname:    "my-pvc",
	}

	for _, o := range opts {
		o(&defaultDeployment)
	}

	return defaultDeployment
}

// Create new Deployment with customized parameters
func (dep *deployment) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", dep.template, "-p", "DNAME="+dep.name, "DNAMESPACE="+dep.namespace, "PVCNAME="+dep.pvcname, "REPLICASNUM="+dep.replicasno, "MPATH="+dep.mpath)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete Deployment from the namespace
func (dep *deployment) delete(oc *exutil.CLI) {
	err := oc.WithoutNamespace().Run("delete").Args("deployment", dep.name, "-n", dep.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Scale Replicas for the Deployment
func (dep *deployment) scaleReplicas(oc *exutil.CLI, replicasno string) {
	err := oc.WithoutNamespace().Run("scale").Args("deployment", dep.name, "--replicas="+replicasno, "-n", dep.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Scaling has been done successfully to pod %s", dep.name)
}
