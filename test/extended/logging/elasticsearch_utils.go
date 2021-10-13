package logging

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func getESIndices(oc *exutil.CLI, ns string, pod string, cmd string) ([]ESIndex, error) {
	if cmd == "" {
		cmd = "es_util --query=_cat/indices?format=JSON"
	}
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 9*time.Second)
	indices := []ESIndex{}
	json.Unmarshal([]byte(stdout), &indices)
	return indices, err
}

func getESIndicesByName(oc *exutil.CLI, ns string, pod string, indexName string) ([]ESIndex, error) {
	cmd := "es_util --query=_cat/indices/" + indexName + "*?format=JSON"
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 9*time.Second)
	indices := []ESIndex{}
	json.Unmarshal([]byte(stdout), &indices)
	return indices, err
}

func waitForIndexAppear(oc *exutil.CLI, ns string, pod string, indexName string, cmd string) {
	if cmd == "" {
		cmd = "es_util --query=_cat/indices?format=JSON"
	}
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		indices, err := getESIndices(oc, ns, pod, cmd)
		count := 0
		for _, index := range indices {
			if strings.Contains(index.Index, indexName) {
				if index.Health != "red" {
					doc_count, _ := strconv.Atoi(index.DocsCount)
					count += doc_count
				}
			}
		}
		if count > 0 && err == nil {
			return true, nil
		} else {
			return false, err
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("index %s is not counted", indexName))
}

func getDocCountPerNamespace(oc *exutil.CLI, ns string, pod string, projectName string, indexName string) (int, error) {
	cmd := "es_util --query=" + indexName + "*/_count?pretty -d '{\"query\": {\"match\": {\"kubernetes.namespace_name\": \"" + projectName + "\"}}}'"
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 30*time.Second)
	res := CountResult{}
	json.Unmarshal([]byte(stdout), &res)
	return res.Count, err
}

func waitForProjectLogsAppear(oc *exutil.CLI, ns string, pod string, projectName string, indexName string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		logCount, err := getDocCountPerNamespace(oc, ns, pod, projectName, indexName)
		if err != nil {
			return false, err
		} else {
			if logCount > 0 {
				return true, nil
			} else {
				return false, nil
			}
		}
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("log of index %s is not got", indexName))
}

func searchInES(oc *exutil.CLI, ns string, pod string, searchCMD string) SearchResult {
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, searchCMD, 3*time.Second, 30*time.Second)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	res := SearchResult{}
	//data := bytes.NewReader([]byte(stdout))
	//_ = json.NewDecoder(data).Decode(&res)
	json.Unmarshal([]byte(stdout), &res)
	return res
}

func getDocCountByK8sLabel(oc *exutil.CLI, ns string, pod string, indexName string, labelName string) (int, error) {
	cmd := "es_util --query=" + indexName + "*/_count?pretty -d '{\"query\": {\"terms\": {\"kubernetes.flat_labels\": [\"" + labelName + "\"]}}}'"
	stdout, err := e2e.RunHostCmdWithRetries(ns, pod, cmd, 3*time.Second, 30*time.Second)
	res := CountResult{}
	json.Unmarshal([]byte(stdout), &res)
	return res.Count, err
}
