package image_registry

import (
	"encoding/json"
	"fmt"
	"strings"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type PrometheusResponse struct {
	Status string                 `json:"status"`
	Error  string                 `json:"error"`
	Data   prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	ResultType string       `json:"resultType"`
	Result     model.Vector `json:"result"`
}

func ListPodStartingWith(prefix string, oc *exutil.CLI, namespace string) (pod []corev1.Pod) {
	podsToAll := []corev1.Pod{}
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		e2e.Logf("Error listing pods: %v", err)
		return nil
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, prefix) {
			podsToAll = append(podsToAll, pod)
		}
	}
	return podsToAll
}

func DePodLogs(pods []corev1.Pod, oc *exutil.CLI, matchlogs string) bool {
	for _, pod := range pods {
		depOutput, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-n", pod.Namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		return strings.Contains(depOutput, matchlogs)
	}
	return false
}

func getBearerTokenURLViaPod(ns string, execPodName string, url string, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl --retry 15 --max-time 2 --retry-delay 1 -s -k -H 'Authorization: Bearer %s' %s", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func runQuery(queryUrl, ns, execPodName, bearerToken string) (*PrometheusResponse, error) {
	contents, err := getBearerTokenURLViaPod(ns, execPodName, queryUrl, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("unable to execute query %v", err)
	}
	var result PrometheusResponse
	if err := json.Unmarshal([]byte(contents), &result); err != nil {
		return nil, fmt.Errorf("unable to parse query response: %v", err)
	}
	metrics := result.Data.Result
	if result.Status != "success" {
		data, _ := json.MarshalIndent(metrics, "", "  ")
		return nil, fmt.Errorf("incorrect response status: %s with error %s", data, result.Error)
	}
	return &result, nil
}

func metricReportStatus(queryUrl, ns, execPodName, bearerToken string, value model.SampleValue) bool {
	result, err := runQuery(queryUrl, ns, execPodName, bearerToken)
	o.Expect(err).NotTo(o.HaveOccurred())
	if result.Data.Result[0].Value == value {
		return true
	}
	return false
}
