package util

import (
	"strconv"
	"strings"
)

// GetClusterVersion returns the cluster version as float value (Ex: 4.8) and cluster build (Ex: 4.8.0-0.nightly-2021-09-28-165247)
func GetClusterVersion(oc *CLI) (float64, string, error) {
	clusterBuild, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o", "jsonpath={..desired.version}").Output()
	if err != nil {
		return 0, "", err
	}
	splitValues := strings.Split(clusterBuild, ".")
	clusterVersion, err := strconv.ParseFloat(splitValues[0]+"."+splitValues[1], 64)
	return clusterVersion, clusterBuild, err
}
