package image_registry

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()
	var (
		oc           = exutil.NewCLI("default-image-oci", exutil.KubeConfigPath())
		manifestType = "application/vnd.oci.image.manifest.v1+json"
	)
	// author: wewang@redhat.com
	g.It("Author:wewang-VMonly-High-36291-OCI image is supported by API server and image registry", func() {
		oc.SetupProject()
		g.By("Import an OCI image to internal registry")
		err := oc.Run("import-image").Args("myimage", "--from", "docker.io/wzheng/busyboxoci", "--confirm", "--reference-policy=local").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "myimage", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("create").Args("serviceaccount", "registry", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "admin", "-z", "registry", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-cluster-role-from-user", "admin", "-z", "registry", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Get internal registry token")
		token, err := oc.WithoutNamespace().Run("sa").Args("get-token", "registry", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get worker nodes")
		workerNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: `node-role.kubernetes.io/worker`})
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Discovered %d worker nodes.", len(workerNodes.Items))
		o.Expect(workerNodes.Items).NotTo(o.HaveLen(0))
		worker := workerNodes.Items[0]
		g.By("Login registry in the node and inspect image")
		commandsOnNode := fmt.Sprintf("podman login image-registry.openshift-image-registry.svc:5000 -u registry -p %q ;podman pull image-registry.openshift-image-registry.svc:5000/%q/myimage;podman inspect image-registry.openshift-image-registry.svc:5000/%q/myimage", token, oc.Namespace(), oc.Namespace())
		out, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("node/"+worker.Name, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c", fmt.Sprintf("%s", commandsOnNode)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Display oci image info")
		e2e.Logf(out)
		o.Expect(out).To(o.ContainSubstring(manifestType))
	})
})
