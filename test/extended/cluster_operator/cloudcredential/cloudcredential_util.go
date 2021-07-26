package cloudcredential

import (
	"encoding/base64"
	"strings"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type PrometheusQueryResult struct {
	Data struct {
		Result []struct {
			Metric struct {
				Name      string `json:"__name__"`
				Container string `json:"container"`
				Endpoint  string `json:"endpoint"`
				Instance  string `json:"instance"`
				Job       string `json:"job"`
				Mode      string `json:"mode"`
				Namespace string `json:"namespace"`
				Pod       string `json:"pod"`
				Service   string `json:"service"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}

func GetCloudCredentialMode(oc *exutil.CLI) (string, error) {
	var (
		mode           string
		iaasPlatform   string
		rootSecretName string
		err            error
	)
	iaasPlatform, err = GetIaasPlatform(oc)
	if err != nil {
		return "", err
	}
	rootSecretName, err = GetRootSecretName(oc)
	if err != nil {
		return "", err
	}
	modeInCloudCredential, err := oc.WithoutNamespace().Run("get").Args("cloudcredential", "cluster", "-o=jsonpath={.spec.credentialsMode}").Output()
	if err != nil {
		return "", err
	}
	if modeInCloudCredential != "Manual" {
		modeInSecretAnnotation, err := oc.WithoutNamespace().Run("get").Args("secret", rootSecretName, "-n=kube-system", "-o=jsonpath={.metadata.annotations.cloudcredential\\.openshift\\.io/mode}").Output()
		if err != nil {
			if strings.Contains(modeInSecretAnnotation, "NotFound") {
				if iaasPlatform != "aws" && iaasPlatform != "azure" && iaasPlatform != "gcp" {
					mode = "passthrough"
					return mode, nil
				}
				mode = "credsremoved"
				return mode, nil
			}
			return "", err
		}
		if modeInSecretAnnotation == "insufficient" {
			mode = "degraded"
			return mode, nil
		}
		mode = modeInSecretAnnotation
		return mode, nil
	}
	if iaasPlatform == "aws" {
		if IsSTSMode(oc) {
			mode = "manualpodidentity"
			return mode, nil
		}
	}
	mode = "manual"
	return mode, nil
}

func GetRootSecretName(oc *exutil.CLI) (string, error) {
	var rootSecretName string

	iaasPlatform, err := GetIaasPlatform(oc)
	if err != nil {
		return "", err
	}
	switch iaasPlatform {
	case "aws":
		rootSecretName = "aws-creds"
	case "gcp":
		rootSecretName = "gcp-credentials"
	case "azure":
		rootSecretName = "azure-credentials"
	case "vsphere":
		rootSecretName = "vsphere-creds"
	case "openstack":
		rootSecretName = "openstack-credentials"
	case "ovirt":
		rootSecretName = "ovirt-credentials"
	default:
		e2e.Logf("Unsupport platform: %v", iaasPlatform)
		return "", nil

	}
	return rootSecretName, nil
}

func IsSTSMode(oc *exutil.CLI) bool {
	output, _ := oc.WithoutNamespace().Run("get").Args("secret", "installer-cloud-credentials", "-n=openshift-image-registry", "-o=jsonpath={.data.credentials}").Output()
	credentials, _ := base64.StdEncoding.DecodeString(output)
	if strings.Contains(string(credentials), "web_identity_token_file") {
		return true
	}
	return false
}

func GetIaasPlatform(oc *exutil.CLI) (string, error) {
	output, err := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	if err != nil {
		return "", err
	}
	iaasPlatform := strings.ToLower(output)
	return iaasPlatform, nil

}
