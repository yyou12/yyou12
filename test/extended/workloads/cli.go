package workloads

import (
	"encoding/json"
	"regexp"
	"os/exec"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cli] Workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("oc", exutil.KubeConfigPath())
	)

	g.It("Author:yinzhou-Medium-28007-Checking oc version show clean as gitTreeState value", func() {
		out, err := oc.Run("version").Args("-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		versionInfo := &VersionInfo{}
		if err := json.Unmarshal([]byte(out), &versionInfo); err != nil {
			e2e.Failf("unable to decode version with error: %v", err)
		}
		if match, _ := regexp.MatchString("clean", versionInfo.ClientInfo.GitTreeState); !match {
			e2e.Failf("varification GitTreeState with error: %v", err)
		}

	})

	// author: yinzhou@redhat.com
        g.It("Author:yinzhou-High-43030-oc get events always show the timestamp as LAST SEEN", func() {
		g.By("Get all the namespace")
		output, err := oc.AsAdmin().Run("get").Args("projects", "-o=custom-columns=NAME:.metadata.name", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		projectList := strings.Fields(output)

		g.By("check the events per project")
		for _, projectN := range projectList {
			output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", projectN).Output()
			if match, _ := regexp.MatchString("No resources found", string(output)); match {
				e2e.Logf("No events in project: %v", projectN)
			}else {
				result, _ := exec.Command("bash", "-c", "cat "+output+" | awk '{print $1}'").Output()
				if match, _ := regexp.MatchString("unknown", string(result)); match {
					e2e.Failf("Does not show timestamp as expected: %v", result)
				}
			}
		}

	})
})

type ClientVersion struct {
	BuildDate    string `json:"buildDate"`
	Compiler     string `json:"compiler"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	GitVersion   string `json:"gitVersion"`
	GoVersion    string `json:"goVersion"`
	Major        string `json:"major"`
	Minor        string `json:"minor"`
	Platform     string `json:"platform"`
}

type ServerVersion struct {
	BuildDate    string `json:"buildDate"`
	Compiler     string `json:"compiler"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	GitVersion   string `json:"gitVersion"`
	GoVersion    string `json:"goVersion"`
	Major        string `json:"major"`
	Minor        string `json:"minor"`
	Platform     string `json:"platform"`
}

type VersionInfo struct {
	ClientInfo       ClientVersion `json:"ClientVersion"`
	OpenshiftVersion string        `json:"openshiftVersion"`
	ServerInfo       ServerVersion `json:"ServerVersion"`
}
