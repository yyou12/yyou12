package util

import (
	"strings"
)

// GetClusterVersion returns the cluster version as string value (Ex: 4.8) and cluster build (Ex: 4.8.0-0.nightly-2021-09-28-165247)
func GetClusterVersion(oc *CLI) (string, string, error) {
	clusterBuild, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o", "jsonpath={..desired.version}").Output()
	if err != nil {
		return "", "", err
	}
	splitValues := strings.Split(clusterBuild, ".")
	clusterVersion := splitValues[0] + "." + splitValues[1]
	return clusterVersion, clusterBuild, err
}
