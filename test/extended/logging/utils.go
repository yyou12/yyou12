package logging

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// SubscriptionObjects objects are used to create operators via OLM
type SubscriptionObjects struct {
	OperatorName  string
	Namespace     string
	OperatorGroup string // the file used to create operator group
	Subscription  string // the file used to create subscription
	PackageName   string
	CatalogSource CatalogSourceObjects `json:",omitempty"`
}

type CatalogSourceObjects struct {
	Channel         string `json:",omitempty"`
	SourceName      string `json:",omitempty"`
	SourceNamespace string `json:",omitempty"`
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func (so *SubscriptionObjects) getChannelName(oc *exutil.CLI) string {
	var channelName string
	if so.CatalogSource.Channel != "" {
		channelName = so.CatalogSource.Channel
	} else {
		/*
			clusterVersion, err := oc.AsAdmin().AdminConfigClient().ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
			if err != nil {
				return "", err
			}
			e2e.Logf("clusterversion is: %v\n", clusterVersion.Status.Desired.Version)
			channelName = strings.Join(strings.Split(clusterVersion.Status.Desired.Version, ".")[0:2], ".")
		*/
		channelName = "stable"
	}
	e2e.Logf("the channel name is: %v\n", channelName)
	return channelName
}

func (so *SubscriptionObjects) getSourceNamespace(oc *exutil.CLI) string {
	var catsrcNamespaceName string
	if so.CatalogSource.SourceNamespace != "" {
		catsrcNamespaceName = so.CatalogSource.SourceNamespace
	} else {
		catsrcNamespaceName = "openshift-marketplace"
	}
	e2e.Logf("The source namespace name is: %v\n", catsrcNamespaceName)
	return catsrcNamespaceName
}

func (so *SubscriptionObjects) getCatalogSourceName(oc *exutil.CLI) string {
	var catsrcName, catsrcNamespaceName string
	catsrcNamespaceName = so.getSourceNamespace(oc)
	catsrc, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catsrc", "-n", catsrcNamespaceName, "qe-app-registry").Output()

	err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", catsrcNamespaceName, "packagemanifests", so.PackageName).Execute()
	if err != nil {
		e2e.Logf("Can't check the packagemanifest %s existence: %v", so.PackageName, err)
	}
	if so.CatalogSource.SourceName != "" {
		catsrcName = so.CatalogSource.SourceName
	} else if catsrc != "" && !(strings.Contains(catsrc, "NotFound")) {
		catsrcName = "qe-app-registry"
	} else {
		catsrcName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests", so.PackageName, "-o", "jsonpath={.status.catalogSource}").Output()
		if err != nil {
			e2e.Logf("error getting catalog source name: %v", err)
		}
	}
	e2e.Logf("The catalog source name of %s is: %v\n", so.PackageName, catsrcName)
	return catsrcName
}

// SubscribeLoggingOperators is used to subcribe the CLO and EO
func (so *SubscriptionObjects) SubscribeLoggingOperators(oc *exutil.CLI) {
	// check if the namespace exists, if it doesn't exist, create the namespace
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(so.Namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			e2e.Logf("The project %s is not found, create it now...", so.Namespace)
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args("namespace", so.Namespace).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", so.Namespace, "openshift.io/cluster-monitoring=true").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	// check the operator group, if no object found, then create an operator group in the project
	og, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", so.Namespace, "og").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	msg := fmt.Sprintf("%v", og)
	if strings.Contains(msg, "No resources found") {
		// create operator group
		ogFile, err := oc.AsAdmin().WithoutNamespace().Run("process").Args("-n", so.Namespace, "-f", so.OperatorGroup, "-p", "OG_NAME="+so.PackageName, "NAMESPACE="+so.Namespace).OutputToFile("og.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", ogFile, "-n", so.Namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	// subscribe operator if the deployment doesn't exist
	_, err = oc.AdminKubeClient().AppsV1().Deployments(so.Namespace).Get(so.OperatorName, metav1.GetOptions{})
	if err != nil {
		// check subscription, if there is no subscription objets, then create one
		if apierrors.IsNotFound(err) {
			sub, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "-n", so.Namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			msg := fmt.Sprintf("%v", sub)
			if strings.Contains(msg, "No resources found") {
				catsrcNamespaceName := so.getSourceNamespace(oc)
				catsrcName := so.getCatalogSourceName(oc)
				channelName := so.getChannelName(oc)
				//check if the packagemanifest is exists in the source namespace or not
				packages, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", catsrcNamespaceName, "packagemanifests", "-l", "catalog="+catsrcName, "-o", "name").Output()
				o.Expect(packages).Should(o.ContainSubstring(so.PackageName))
				//create subscription object
				subscriptionFile, err := oc.AsAdmin().WithoutNamespace().Run("process").Args("-n", so.Namespace, "-f", so.Subscription, "-p", "PACKAGE_NAME="+so.PackageName, "NAMESPACE="+so.Namespace, "CHANNEL="+channelName, "SOURCE="+catsrcName, "SOURCE_NAMESPACE="+catsrcNamespaceName).OutputToFile("subscription.json")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", subscriptionFile, "-n", so.Namespace).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	}
	WaitForDeploymentPodsToBeReady(oc, so.Namespace, so.OperatorName)
}

//WaitForDeploymentPodsToBeReady waits for the specific deployment to be ready
func WaitForDeploymentPodsToBeReady(oc *exutil.CLI, namespace string, name string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of %s deployment\n", name)
				return false, nil
			}
			return false, err
		}
		if int(deployment.Status.AvailableReplicas) == int(deployment.Status.Replicas) {
			replicas := int(deployment.Status.Replicas)
			e2e.Logf("Deployment %s available (%d/%d)\n", name, replicas, replicas)
			return true, nil
		} else {
			e2e.Logf("Waiting for full availability of %s deployment (%d/%d)\n", name, deployment.Status.AvailableReplicas, deployment.Status.Replicas)
			return false, nil
		}

	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("deployment %s is not availabile", name))
}

//WaitForDaemonsetPodsToBeReady waits for all the pods controlled by the ds to be ready
func WaitForDaemonsetPodsToBeReady(oc *exutil.CLI, ns string, name string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of %s daemonset\n", name)
				return false, nil
			}
			return false, err
		}
		if int(daemonset.Status.NumberReady) == int(daemonset.Status.DesiredNumberScheduled) {
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s daemonset (%d/%d)\n", name, int(daemonset.Status.NumberReady), int(daemonset.Status.DesiredNumberScheduled))
		return false, nil
	})
	e2e.Logf("Daemonset %s is available\n", name)
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Daemonset %s is not availabile", name))
}

//GetDeploymentsNameByLabel retruns a list of deployment name which have specific labels
func GetDeploymentsNameByLabel(oc *exutil.CLI, ns string, label string) []string {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of deployment\n")
				return false, nil
			}
			return false, err
		}
		if len(deployList.Items) > 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("deployment with label %s is not availabile", label))
	if err == nil {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeployments := make([]string, 0, len(deployList.Items))
		for _, deploy := range deployList.Items {
			expectedDeployments = append(expectedDeployments, deploy.Name)
		}
		return expectedDeployments
	}
	return nil
}

//WaitForEFKPodsToBeReady checks if the EFK pods could become ready or not
func WaitForEFKPodsToBeReady(oc *exutil.CLI, ns string) {
	//wait for ES
	esDeployNames := GetDeploymentsNameByLabel(oc, ns, "cluster-name=elasticsearch")
	for _, name := range esDeployNames {
		WaitForDeploymentPodsToBeReady(oc, ns, name)
	}
	// wait for Kibana
	WaitForDeploymentPodsToBeReady(oc, ns, "kibana")
	// wait for fluentd
	WaitForDaemonsetPodsToBeReady(oc, ns, "fluentd")
}

type resource struct {
	kind      string
	name      string
	namespace string
}

//WaitUntilResourceIsGone waits for the resource to be removed cluster
func (r resource) WaitUntilResourceIsGone(oc *exutil.CLI) error {
	return wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", r.namespace, r.kind, r.name).Output()
		if err != nil {
			errstring := fmt.Sprintf("%v", output)
			if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
}

//delete the objects in the cluster
func (r resource) clear(oc *exutil.CLI) error {
	msg, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", r.namespace, r.kind, r.name).Output()
	if err != nil {
		errstring := fmt.Sprintf("%v", msg)
		if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") {
			return nil
		}
		return err
	}
	err = r.WaitUntilResourceIsGone(oc)
	return err
}

func (r resource) WaitForResourceToAppear(oc *exutil.CLI) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", r.namespace, r.kind, r.name).Output()
		if err != nil {
			msg := fmt.Sprintf("%v", output)
			if strings.Contains(msg, "NotFound") {
				return false, nil
			}
			return false, err
		}
		e2e.Logf("Find %s %s", r.kind, r.name)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %s is not appear", r.name))
}

func (r resource) applyFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("process").Args(parameters...).OutputToFile(getRandomString() + ".json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile, "-n", r.namespace).Execute()
	r.WaitForResourceToAppear(oc)
	return err
}

//DeleteClusterLogging deletes the clusterlogging instance and ensures the related resources are removed
func (r resource) deleteClusterLogging(oc *exutil.CLI) {
	err := r.clear(oc)
	if err != nil {
		e2e.Logf("could not delete %s/%s", r.kind, r.name)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("could not delete %s/%s", r.kind, r.name))
	//make sure other resources are removed
	resources := []resource{{"elasticsearch", "elasticsearch", r.namespace}, {"kibana", "kibana", r.namespace}, {"daemonset", "fluentd", r.namespace}}
	for i := 0; i < len(resources); i++ {
		err = resources[i].WaitUntilResourceIsGone(oc)
		if err != nil {
			e2e.Logf("%s/%s is not deleted", resources[i].kind, resources[i].name)
		}
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s/%s is not deleted", resources[i].kind, resources[i].name))
	}
	// remove all the pvcs in the namespace
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", r.namespace, "pvc", "--all").Execute()
}

func (r resource) createClusterLogging(oc *exutil.CLI, parameters ...string) {
	// delete clusterlogging instance first
	r.deleteClusterLogging(oc)
	err := r.applyFromTemplate(oc, parameters...)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func DeleteNamespace(oc *exutil.CLI, ns string) {
	err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = nil
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func WaitForIMCronJobToAppear(oc *exutil.CLI, ns string, name string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		_, err = oc.AdminKubeClient().BatchV1beta1().CronJobs(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of cronjob\n")
				return false, nil
			} else {
				return false, err
			}
		} else {
			return true, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cronjob %s is not availabile", name))
}

func waitForIMJobsToComplete(oc *exutil.CLI, ns string, timeout time.Duration) {
	// wait for jobs to appear
	err := wait.Poll(5*time.Second, timeout, func() (done bool, err error) {
		jobList, err := oc.AdminKubeClient().BatchV1().Jobs(ns).List(metav1.ListOptions{LabelSelector: "component=indexManagement"})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of jobs\n")
				return false, nil
			}
			return false, err
		}
		if len(jobList.Items) >= 3 {
			return true, nil
		} else {
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("jobs %s with is not availabile", "component=indexManagement"))
	// wait for jobs to complete
	jobList, err := oc.AdminKubeClient().BatchV1().Jobs(ns).List(metav1.ListOptions{LabelSelector: "component=indexManagement"})
	if err != nil {
		panic(err)
	}
	for _, job := range jobList.Items {
		err := wait.Poll(2*time.Second, 60*time.Second, func() (bool, error) {
			job, err := oc.AdminKubeClient().BatchV1().Jobs(ns).Get(job.Name, metav1.GetOptions{})
			//succeeded, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "job", job.Name, "-o=jsonpath={.status.succeeded}").Output()
			if err != nil {
				return false, err
			} else {
				if job.Status.Succeeded == 1 {
					e2e.Logf("job %s completed successfully", job.Name)
					return true, nil
				} else {
					e2e.Logf("job %s is not completed yet", job.Name)
					return false, nil
				}
			}
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("job %s is not completed yet", job.Name))
	}
}

func getStorageClassName(oc *exutil.CLI) (string, error) {
	var scName string
	defaultSC := ""
	SCs, err := oc.AdminKubeClient().StorageV1().StorageClasses().List(metav1.ListOptions{})
	for _, sc := range SCs.Items {
		if sc.ObjectMeta.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultSC = sc.Name
			break
		}
	}
	if defaultSC != "" {
		scName = defaultSC
	} else {
		scName = SCs.Items[0].Name
	}
	return scName, err
}
