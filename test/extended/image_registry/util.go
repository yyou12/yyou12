package image_registry

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	asAdmin          = true
	withoutNamespace = true
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

type bcSource struct {
	outname   string
	name      string
	namespace string
	template  string
}

func (bcsrc *bcSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", bcsrc.template, "-p", "OUTNAME="+bcsrc.outname, "NAME="+bcsrc.name, "NAMESPACE="+bcsrc.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}
func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
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

//the method is to get something from resource. it is "oc get xxx" actaully
func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("output is %v, error is %v, and try next", result, err)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("$oc get %v, the returned resource:%v", parameters, result)
	return result
}

//the method is to do something with oc.
func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}
