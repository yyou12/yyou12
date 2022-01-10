package storage

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/tidwall/gjson"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Get the credential from cluster
func getCredentialFromCluster(oc *exutil.CLI) {
	cloudProvider := getCloudProvider(oc)
	switch getCloudProvider(oc) {
	case "aws":
		credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
		if strings.Contains(interfaceToString(err), "not found") {
			credential, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/ebs-cloud-credentials", "-n", "openshift-cluster-csi-drivers", "-o", "json").Output()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		accessKeyIdBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).String(), gjson.Get(credential, `data.aws_secret_access_key`).String()
		accessKeyId, err1 := base64.StdEncoding.DecodeString(accessKeyIdBase64)
		o.Expect(err1).NotTo(o.HaveOccurred())
		secureKey, err2 := base64.StdEncoding.DecodeString(secureKeyBase64)
		o.Expect(err2).NotTo(o.HaveOccurred())
		clusterRegion, err3 := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
		o.Expect(err3).NotTo(o.HaveOccurred())
		os.Setenv("AWS_ACCESS_KEY_ID", string(accessKeyId))
		os.Setenv("AWS_SECRET_ACCESS_KEY", string(secureKey))
		os.Setenv("AWS_REGION", clusterRegion)
	case "vsphere":
		e2e.Logf("Get %s backend credential is under development", cloudProvider)
	case "gcp":
		e2e.Logf("Get %s backend credential is under development", cloudProvider)
	case "azure":
		e2e.Logf("Get %s backend credential is under development", cloudProvider)
	case "openstack":
		e2e.Logf("Get %s backend credential is under development", cloudProvider)
	default:
		e2e.Logf("unknown cloud provider")
	}
}

// Get the volume detail info by persistent volume id
func getAwsVolumeInfoByVolumeId(volumeId string) (string, error) {
	mySession := session.Must(session.NewSession())
	svc := ec2.New(mySession, aws.NewConfig())
	input := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("volume-id"),
				Values: []*string{
					aws.String(volumeId),
				},
			},
		},
	}
	volumeInfo, err := svc.DescribeVolumes(input)
	return interfaceToString(volumeInfo), err
}

// Get the volume status "in use" or "avaiable" by persistent volume id
func getAwsVolumeStatusByVolumeId(volumeId string) (string, error) {
	volumeInfo, err := getAwsVolumeInfoByVolumeId(volumeId)
	o.Expect(err).NotTo(o.HaveOccurred())
	volumeStatus := gjson.Get(volumeInfo, `Volumes.0.State`).Str
	e2e.Logf("The volume %s status is %q on aws backend", volumeId, volumeStatus)
	return volumeStatus, err
}

// Delete backend volume
func deleteBackendVolumeByVolumeId(oc *exutil.CLI, volumeId string) (string, error) {
	cloudProvider := getCloudProvider(oc)
	switch getCloudProvider(oc) {
	case "aws":
		mySession := session.Must(session.NewSession())
		svc := ec2.New(mySession, aws.NewConfig())
		deleteVolumeID := &ec2.DeleteVolumeInput{
			VolumeId: &volumeId,
		}
		req, resp := svc.DeleteVolumeRequest(deleteVolumeID)
		return interfaceToString(resp), req.Send()
	case "vsphere":
		e2e.Logf("Delete %s backend volume is under development", cloudProvider)
		return "under development now", nil
	case "gcp":
		e2e.Logf("Delete %s backend volume is under development", cloudProvider)
		return "under development now", nil
	case "azure":
		e2e.Logf("Delete %s backend volume is under development", cloudProvider)
		return "under development now", nil
	case "openstack":
		e2e.Logf("Delete %s backend volume is under development", cloudProvider)
		return "under development now", nil
	default:
		e2e.Logf("unknown cloud provider")
		return "under development now", nil
	}
}

//  Check the volume status becomes avaiable, status is "avaiable"
func checkVolumeAvaiableOnBackend(volumeId string) (bool, error) {
	volumeStatus, err := getAwsVolumeStatusByVolumeId(volumeId)
	avaiableStatus := []string{"available"}
	return contains(avaiableStatus, volumeStatus), err
}

//  Check the volume is deleted
func checkVolumeDeletedOnBackend(volumeId string) (bool, error) {
	volumeStatus, err := getAwsVolumeStatusByVolumeId(volumeId)
	deletedStatus := []string{""}
	return contains(deletedStatus, volumeStatus), err
}

//  Waiting the volume become avaiable
func waitVolumeAvaiableOnBackend(oc *exutil.CLI, volumeId string) {
	cloudProvider := getCloudProvider(oc)
	switch getCloudProvider(oc) {
	case "aws":
		err := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			volumeStatus, errinfo := checkVolumeAvaiableOnBackend(volumeId)
			if errinfo != nil {
				e2e.Logf("the err:%v, wait for volume %v to become avaiable.", errinfo, volumeId)
				return volumeStatus, errinfo
			}
			if !volumeStatus {
				return volumeStatus, nil
			}
			return volumeStatus, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, is not avaiable.", volumeId))
	case "vsphere":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	case "gcp":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	case "azure":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	case "openstack":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	default:
		e2e.Logf("unknown cloud provider")
	}
}

//  Waiting the volume become deleted
func waitVolumeDeletedOnBackend(oc *exutil.CLI, volumeId string) {
	cloudProvider := getCloudProvider(oc)
	switch getCloudProvider(oc) {
	case "aws":
		err := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			volumeStatus, errinfo := checkVolumeDeletedOnBackend(volumeId)
			if errinfo != nil {
				e2e.Logf("the err:%v, wait for volume %v to be deleted.", errinfo, volumeId)
				return volumeStatus, errinfo
			}
			if !volumeStatus {
				return volumeStatus, nil
			}
			return volumeStatus, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, still exist.", volumeId))
	case "vsphere":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	case "gcp":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	case "azure":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	case "openstack":
		e2e.Logf("Get %s backend volume status is under development", cloudProvider)
	default:
		e2e.Logf("unknown cloud provider")
	}
}

// Get the volume type by volume id
func getAwsVolumeTypeByVolumeId(volumeId string) string {
	volumeInfo, err := getAwsVolumeInfoByVolumeId(volumeId)
	o.Expect(err).NotTo(o.HaveOccurred())
	volumeType := gjson.Get(volumeInfo, `Volumes.0.VolumeType`).Str
	e2e.Logf("The volume %s type is %q on aws backend", volumeId, volumeType)
	return volumeType
}

// Get the volume iops by volume id
func getAwsVolumeIopsByVolumeId(volumeId string) int64 {
	volumeInfo, err := getAwsVolumeInfoByVolumeId(volumeId)
	o.Expect(err).NotTo(o.HaveOccurred())
	volumeIops := gjson.Get(volumeInfo, `Volumes.0.Iops`).Int()
	e2e.Logf("The volume %s Iops is %d on aws backend", volumeId, volumeIops)
	return volumeIops
}
