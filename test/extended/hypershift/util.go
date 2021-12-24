package hypershift

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"strings"
)

type OcpClientVerb string

const (
	OcpGet    OcpClientVerb = "get"
	OcpPatch  OcpClientVerb = "patch"
	OcpWhoami OcpClientVerb = "whoami"
)

func doOcpReq(oc *exutil.CLI, verb OcpClientVerb, notEmpty bool, args []string) string {
	e2e.Logf("running command : oc %s %s \n", string(verb), strings.Join(args, " "))
	res, err := oc.AsAdmin().WithoutNamespace().Run(string(verb)).Args(args...).Output()
	o.Expect(err).ShouldNot(o.HaveOccurred())
	if notEmpty {
		o.Expect(res).ShouldNot(o.BeEmpty())
	}
	return res
}

func checkSubstring(src string, expect []string) {
	if expect == nil || len(expect) <= 0 {
		o.Expect(expect).ShouldNot(o.BeEmpty())
	}

	for i := 0; i < len(expect); i++ {
		o.Expect(src).To(o.ContainSubstring(expect[i]))
	}
}
