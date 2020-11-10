package apiserver_and_auth

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinary] Apiserver_and_Auth", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: kewang@redhat.com
	g.It("Medium-32383-bug 1793694 init container setup should have the proper securityContext", func() {
		checkItems := []struct {
			namespace string
			container string
		}{
			{
				namespace: "openshift-kube-apiserver",
				container: "kube-apiserver",
			},
			{
				namespace: "openshift-apiserver",
				container: "openshift-apiserver",
			},
		}

		for _, checkItem := range checkItems {
			g.By("Get one pod name of " + checkItem.namespace)
			e2e.Logf("namespace is :%s", checkItem.namespace)
			podName, err := oc.AsAdmin().Run("get").Args("-n", checkItem.namespace, "pods", "-l apiserver", "-o=jsonpath={.items[0].metadata.name}").Output()
			if err != nil {
				e2e.Failf("Failed to get kube-apiserver pod name and returned error: %v", err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Get the kube-apiserver pod name: %s", podName)

			g.By("Get privileged value of container " + checkItem.container + " of pod " + podName)
			jsonpath := "-o=jsonpath={range .spec.containers[?(@.name==\"" + checkItem.container + "\")]}{.securityContext.privileged}"
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, jsonpath, "-n", checkItem.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("true"))
			e2e.Logf("#### privileged value: %s ####", msg)

			g.By("Get privileged value of initcontainer of pod " + podName)
			jsonpath = "-o=jsonpath={.spec.initContainers[].securityContext.privileged}"
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, jsonpath, "-n", checkItem.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("true"))
			e2e.Logf("#### privileged value: %s ####", msg)
		}
	})
})
