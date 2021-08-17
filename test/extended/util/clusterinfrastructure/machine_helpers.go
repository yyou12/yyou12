package clusterinfrastructure

import (
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"

)


const (
	machineAPINamespace = "openshift-machine-api"
)

type MachineSetDescription struct {
	Name      string
	Namespace string
	Replicas  int
	ClusterID string
	Region    string
	Zone      string
	Ami       string
	Image     string
	template  string
}

// CreateMachineSet create a new machineset
func (ms *MachineSetDescription) CreateMachineSet(oc *exutil.CLI) {
	e2e.Logf("Creating a new MachineSets ...")
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ms.template, "-p", "NAME="+ms.Name, "NAMESPACE="+machineAPINamespace, "REPLICAS="+strconv.Itoa(ms.Replicas), "CLUSTERID="+ms.ClusterID, "REGION="+ms.Region, "ZONE="+ms.Zone, "AMI="+ms.Ami, "IMAGE="+ms.Image)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// DeleteMachineSet delete a machineset
func (ms *MachineSetDescription) DeleteMachineSet(oc *exutil.CLI) error {
	e2e.Logf("Deleting a MachineSets ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("machineset", ms.Name, "-n", machineAPINamespace).Execute()
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "machine-config.json")
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("The resource is %s", jsonCfg)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", jsonCfg).Execute()
}

// ListWorkerMachineSets list all worker machineSets
func ListWorkerMachineSets(oc *exutil.CLI) []string {
	e2e.Logf("Listing all MachineSets ...")
	machineSetNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-o=jsonpath={.items[*].metadata.name}", "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineSetNames, " ")
}

// ListWorkerMachines list all worker machines
func ListWorkerMachines(oc *exutil.CLI) []string {
	e2e.Logf("Listing all Machines ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machine", "-o=jsonpath={.items[*].metadata.name}", "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// GetRandomMachineSetName get a random MachineSet name
func GetRandomMachineSetName(oc *exutil.CLI) string {
	e2e.Logf("Getting a random MachineSet ...")
	return ListWorkerMachineSets(oc)[0]
}

// ScaleMachineSet scale a MachineSet by replicas
func ScaleMachineSet(oc *exutil.CLI, machineSetName string, replicas int) {
	e2e.Logf("Scaling MachineSets ...")
	_, err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), "machineset", machineSetName, "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// WaitForMachinesRunning check if all the machines are Running in a MachineSet
func WaitForMachinesRunning(oc *exutil.CLI, machineNumber int, machineSetName string) {
	e2e.Logf("Waiting for the machines Running ...")
	pollErr := wait.Poll(60*time.Second, 420*time.Second, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machineSetName, "-o=jsonpath={.status.readyReplicas}", "-n", machineAPINamespace).Output()
		machinesRunning, _ := strconv.Atoi(msg)
		if machinesRunning != machineNumber {
			e2e.Logf("Expected %v  machine are not Running yet and waiting up to 1 minutes ...", machineNumber)
			return false, nil
		}
		e2e.Logf("Expected %v  machines are Running", machineNumber)
		return true, nil
	})
	if pollErr != nil {
		e2e.Failf("Expected %v  machines are not Running after waiting up to 7 minutes ...", machineNumber)
	}
	e2e.Logf("All machines are Running ...")
}

func CheckPlatform(oc *exutil.CLI) string {
	output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	return strings.ToLower(output)
}

// SkipConditionally check the total number of Running machines, if greater than zero, we think machines are managed by machine api operator.
func SkipConditionally(oc *exutil.CLI) {
	msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines", "--no-headers", "-n", machineAPINamespace).Output()
	machinesRunning := strings.Count(msg, "Running")
	if machinesRunning == 0 {
		g.Skip("Expect at least one Running machine. Found none!!!")
	}
}

// SetMachineSetTemplate update machineset template by iaasPlatform
func SetMachineSetTemplate(oc *exutil.CLI, iaasPlatform string, clusterInfrastructureBaseDir string) (ms MachineSetDescription) {
	e2e.Logf("Setting MachineSets template ...")
	machineSetTemplate := ""
	if iaasPlatform == "aws" {
		machineSetTemplate = filepath.Join(clusterInfrastructureBaseDir, "aws-machineset.yaml")
		clusterID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", GetRandomMachineSetName(oc), "-o=jsonpath={.spec.template.spec.providerSpec.value.placement.region}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		zone, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", GetRandomMachineSetName(oc), "-o=jsonpath={.spec.template.spec.providerSpec.value.placement.availabilityZone}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		ami, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", GetRandomMachineSetName(oc), "-o=jsonpath={.spec.template.spec.providerSpec.value.ami.id}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		ms = MachineSetDescription{
			Name:      "machineset-default",
			Namespace: machineAPINamespace,
			Replicas:  1,
			ClusterID: clusterID,
			Region:    region,
			Zone:      zone,
			Ami:       ami,
			template:  machineSetTemplate,
		}
	} else if iaasPlatform == "azure" {
		machineSetTemplate = filepath.Join(clusterInfrastructureBaseDir, "azure-machineset.yaml")
		clusterID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", GetRandomMachineSetName(oc), "-o=jsonpath={.spec.template.spec.providerSpec.value.location}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		ms = MachineSetDescription{
			Name:      "machineset-default",
			Namespace: machineAPINamespace,
			Replicas:  1,
			Region:    region,
			ClusterID: clusterID,
			template:  machineSetTemplate,
		}
	} else if iaasPlatform == "gcp" {
		machineSetTemplate = filepath.Join(clusterInfrastructureBaseDir, "gcp-machineset.yaml")
		clusterID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", GetRandomMachineSetName(oc), "-o=jsonpath={.spec.template.spec.providerSpec.value.region}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		zone, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", GetRandomMachineSetName(oc), "-o=jsonpath={.spec.template.spec.providerSpec.value.zone}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		image, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", GetRandomMachineSetName(oc), "-o=jsonpath={.spec.template.spec.providerSpec.value.disks[0].image}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		ms = MachineSetDescription{
			Name:      "machineset-default",
			Namespace: machineAPINamespace,
			Replicas:  1,
			ClusterID: clusterID,
			Region:    region,
			Zone:      zone,
			Image:     image,
			template:  machineSetTemplate,
		}
	} else if iaasPlatform == "vsphere" {
		machineSetTemplate = filepath.Join(clusterInfrastructureBaseDir, "vsphere-machineset.yaml")
		clusterID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		ms = MachineSetDescription{
			Name:      "machineset-default",
			Namespace: machineAPINamespace,
			Replicas:  1,
			ClusterID: clusterID,
			template:  machineSetTemplate,
		}
	} else if iaasPlatform == "openstack" {
		machineSetTemplate = filepath.Join(clusterInfrastructureBaseDir, "osp-machineset.yaml")
		clusterID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		ms = MachineSetDescription{
			Name:      "machineset-default",
			Namespace: machineAPINamespace,
			Replicas:  1,
			ClusterID: clusterID,
			template:  machineSetTemplate,
		}
	} else {
		e2e.Failf("IAAS platform: %s is not automated yet", iaasPlatform)
	}
	e2e.Logf("Finished setting MachineSets template ...")
	return ms
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
