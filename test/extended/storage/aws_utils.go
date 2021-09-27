package storage

import (
	"encoding/base64"
	"fmt"
	"os"
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

// Get the secret from cluster
func getCreditFromCluster(oc *exutil.CLI) (error, error) {
	credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	accessKeyIdBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).Str, gjson.Get(credential, `data.aws_secret_access_key`).Str
	accessKeyId, err1 := base64.StdEncoding.DecodeString(accessKeyIdBase64)
	o.Expect(err1).NotTo(o.HaveOccurred())
	secureKey, err2 := base64.StdEncoding.DecodeString(secureKeyBase64)
	o.Expect(err2).NotTo(o.HaveOccurred())
	return os.Setenv("AWS_ACCESS_KEY_ID", string(accessKeyId)), os.Setenv("AWS_SECRET_ACCESS_KEY", string(secureKey))
}

// Get the volume status "in use" or "avaiable" by persistent volume id
func getAwsVolumeStatusByVolumeId(volumeId string) (string, error) {
	mySession := session.Must(session.NewSession())
	svc := ec2.New(mySession, aws.NewConfig().WithRegion("us-east-2"))
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
	result, err := svc.DescribeVolumes(input)
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Get the volume:%v status failed.", volumeId))
	volumeStatus := gjson.Get(interfaceToString(result), `Volumes.0.State`).Str
	e2e.Logf("The volume %s status is %q on aws backend", volumeId, volumeStatus)
	return volumeStatus, err
}

//  Check the volume status becomes avaiable, status is "avaiable"
func checkVolumeAvaiable(volumeId string) (bool, error) {
	volumeStatus, err := getAwsVolumeStatusByVolumeId(volumeId)
	avaiableStatus := []string{"available"}
	return contains(avaiableStatus, volumeStatus), err
}

//  Check the volume is deleted
func checkVolumeDeleted(volumeId string) (bool, error) {
	volumeStatus, err := getAwsVolumeStatusByVolumeId(volumeId)
	deletedStatus := []string{""}
	return contains(deletedStatus, volumeStatus), err
}

//  Waiting the volume become avaiable
func waitVolumeAvaiable(volumeId string) {
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		volumeStatus, errinfo := checkVolumeAvaiable(volumeId)
		if errinfo != nil {
			e2e.Logf("the err:%v, wait for volume %v to become avaiable.", errinfo, volumeId)
			return volumeStatus, errinfo
		}
		if !volumeStatus {
			return volumeStatus, nil
		}
		return volumeStatus, nil
	})

	if err != nil {
		e2e.Logf("Error occured: %v, the volume:%v is still in-use.", err, volumeId)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, is still in-use.", volumeId))
}

//  Waiting the volume become deleted
func waitVolumeDeleted(volumeId string) {
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		volumeStatus, errinfo := checkVolumeDeleted(volumeId)
		if errinfo != nil {
			e2e.Logf("the err:%v, wait for volume %v to be deleted.", errinfo, volumeId)
			return volumeStatus, errinfo
		}
		if !volumeStatus {
			return volumeStatus, nil
		}
		return volumeStatus, nil
	})

	if err != nil {
		e2e.Logf("Error occured: %v, the volume:%v still exist.", err, volumeId)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, still exist.", volumeId))
}

// Get the volume type by volume id
func getAwsVolumeTypeByVolumeId(volumeId string) (string, error) {
	mySession := session.Must(session.NewSession())
	svc := ec2.New(mySession, aws.NewConfig().WithRegion("us-east-2"))
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
	result, err := svc.DescribeVolumes(input)
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Get the volume:%v type failed.", volumeId))
	volumeType := gjson.Get(interfaceToString(result), `Volumes.0.VolumeType`).Str
	e2e.Logf("The volume %s type is %q on aws backend", volumeId, volumeType)
	return volumeType, err
}

// Get the volume iops by volume id
func getAwsVolumeIopsByVolumeId(volumeId string) (int64, error) {
	mySession := session.Must(session.NewSession())
	svc := ec2.New(mySession, aws.NewConfig().WithRegion("us-east-2"))
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
	result, err := svc.DescribeVolumes(input)
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Get the volume:%v Iops failed.", volumeId))
	volumeIops := gjson.Get(interfaceToString(result), `Volumes.0.Iops`).Int()
	e2e.Logf("The volume %s Iops is %d on aws backend", volumeId, volumeIops)
	return volumeIops, err
}
