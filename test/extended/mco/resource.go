package mco

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/types"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type ocGetter struct {
	oc        *exutil.CLI
	kind      string
	namespace string
	name      string
}

// Resource will provide the functionality to hanlde general openshift resources
type Resource struct {
	ocGetter
}

// Get uses the CLI to retrieve the return value for this jsonpath
func (r *ocGetter) Get(jsonPath string, extraParams ...string) (string, error) {
	params := []string{r.kind}
	if r.name != "" {
		params = append(params, r.name)
	}

	if r.namespace != "" {
		params = append([]string{"-n", r.namespace}, params...)
	}

	params = append(params, extraParams...)

	params = append(params, []string{"-o", fmt.Sprintf("jsonpath=%s", jsonPath)}...)

	result, err := r.oc.WithoutNamespace().Run("get").Args(params...).Output()

	return result, err
}

// GetSafe uses the CLI to retrieve the return value for this jsonpath, if the resource does not exist, it returns the defaut value
func (r *ocGetter) GetSafe(jsonPath string, defaultValue string, extraParams ...string) string {
	ret, err := r.Get(jsonPath, extraParams...)
	if err != nil {
		return defaultValue
	}

	return ret
}

// GetOrFail uses the CLI to retrieve the return value for this jsonpath, if the resource does not exist, it fails the test
func (r *ocGetter) GetOrFail(jsonPath string, extraParams ...string) string {
	ret, err := r.Get(jsonPath, extraParams...)
	if err != nil {
		e2e.Failf("%v", err)
	}

	return ret
}

// PollValue returns a function suitable to be used with the gomega Eventually/Consistently checks
func (r *ocGetter) Poll(jsonPath string) func() string {
	return func() string {
		ret, _ := r.Get(jsonPath)
		return ret
	}
}

// NewResource constructs a Resource struct for a not-namespaced resource
func NewResource(oc *exutil.CLI, kind string, name string) *Resource {
	return &Resource{ocGetter{oc, kind, "", name}}
}

// NewNamespacedResource constructs a Resource struct for a namespaced resource
func NewNamespacedResource(oc *exutil.CLI, kind string, namespace string, name string) *Resource {
	return &Resource{ocGetter{oc, kind, namespace, name}}
}

// Delete removes the resource from openshift cluster
func (r *Resource) Delete() error {
	params := []string{r.kind}
	if r.name != "" {
		params = append(params, r.name)
	}

	if r.namespace != "" {
		params = append([]string{"-n", r.namespace}, params...)
	}

	_, err := r.oc.WithoutNamespace().Run("delete").Args(params...).Output()
	if err != nil {
		e2e.Logf("%v", err)
	}

	return err
}

// Exists returns true if the resource exists and false if not
func (r *Resource) Exists() bool {
	_, err := r.Get("{.}")
	return err == nil
}

// String implements the Stringer interface
func (r *Resource) String() string {
	return fmt.Sprintf("<Kind: %s, Name: %s, Namespace: %s>", r.kind, r.name, r.namespace)
}

// ResourceList provides the functionality to handle lists of openshift resources
type ResourceList struct {
	ocGetter
	extraParams []string
}

// NewResourceList constructs a ResourceList struct for not-namespaced resources
func NewResourceList(oc *exutil.CLI, kind string) *ResourceList {
	return &ResourceList{ocGetter{oc.AsAdmin(), kind, "", ""}, []string{}}
}

// NewNamespacedResourceList constructs a ResourceList struct for namespaced resources
func NewNamespacedResourceList(oc *exutil.CLI, kind string, namespace string) *ResourceList {
	return &ResourceList{ocGetter{oc.AsAdmin(), kind, namespace, ""}, []string{}}
}

// SortByTimestamp will configure the list to be sorted by creation timestamp
func (l *ResourceList) SortByTimestamp() *ResourceList {
	l.extraParams = append(l.extraParams, "--sort-by=metadata.creationTimestamp")
	return l
}

// GetAllResources returns a list of Resource structs with the resources found in this list
func (l ResourceList) GetAllResources() ([]Resource, error) {
	allItemsNames, err := l.Get("{.items[*].metadata.name}", l.extraParams...)
	if err != nil {
		e2e.Failf("%v", err)
	}
	allNames := strings.Split(allItemsNames, " ")

	allResources := []Resource{}
	for _, name := range allNames {
		newResource := Resource{ocGetter{l.oc, l.kind, l.namespace, name}}
		allResources = append(allResources, newResource)
	}

	return allResources, nil
}

// Exist returns a gomega matcher that checks if a resource exists or not
func Exist() types.GomegaMatcher {
	return &existMatcher{}
}

type existMatcher struct {
}

func (matcher *existMatcher) Match(actual interface{}) (success bool, err error) {
	resource, ok := actual.(*Resource)
	if !ok {
		return false, fmt.Errorf("Exist matcher expects a Resource in case %v", g.CurrentGinkgoTestDescription().TestText)
	}

	return resource.Exists(), nil
}

func (matcher *existMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected resource\n\t%s\nto exist", actual)
}

func (matcher *existMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected resource\n\t%s\nnot to exist", actual)
}