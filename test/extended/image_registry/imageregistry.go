package image_registry

import (
	"encoding/base64"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()
	var (
		oc        = exutil.NewCLI("default-image-registry", exutil.KubeConfigPath())
		bcName    = "rails-postgresql-example"
		bcNameOne = fmt.Sprintf("%s-1", bcName)
		logInfo   = `Unsupported value: "abc": supported values: "", "Normal", "Debug", "Trace", "TraceAll"`
	)
	// author: wewang@redhat.com
	g.It("Author:wewang-High-39027-Check AWS secret and access key with an OpenShift installed in a regular way", func() {
		output, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Check AWS secret and access key inside image registry pod")
		result, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "cat", "/var/run/secrets/cloud/credentials").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("aws_access_key_id"))
		o.Expect(result).To(o.ContainSubstring("aws_secret_access_key"))
		g.By("Check installer-cloud-credentials secret")
		credentials, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/installer-cloud-credentials", "-n", "openshift-image-registry", "-o=jsonpath={.data.credentials}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Sprintf("credentials is %s", credentials)
		sDec, err := base64.StdEncoding.DecodeString(credentials)
		if err != nil {
			fmt.Printf("Error decoding string: %s ", err.Error())
		}
		o.Expect(sDec).To(o.ContainSubstring("aws_access_key_id"))
		o.Expect(sDec).To(o.ContainSubstring("aws_secret_access_key"))
		g.By("push/pull image to registry")
		oc.SetupProject()
		err = oc.Run("import-image").Args("myimage", "--from", "busybox", "--confirm", "--reference-policy=local").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "myimage", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("new-app").Args("rails-postgresql-example").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for build to finish")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), bcNameOne, nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs(bcName, oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-High-34992-Add logLevel to registry config object with invalid value", func() {
		g.By("Change spec.loglevel with invalid values")
		out, _ := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"logLevel":"abc"}}`, "--type=merge").Output()
		o.Expect(out).To(o.ContainSubstring(logInfo))
		g.By("Change spec.operatorLogLevel with invalid values")
		out, _ = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"operatorLogLevel":"abc"}}`, "--type=merge").Output()
		o.Expect(out).To(o.ContainSubstring(logInfo))
	})
})
