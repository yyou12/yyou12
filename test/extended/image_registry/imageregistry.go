package image_registry

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()
	var (
		oc           = exutil.NewCLI("default-image-registry", exutil.KubeConfigPath())
		bcName       = "rails-postgresql-example"
		bcNameOne    = fmt.Sprintf("%s-1", bcName)
		logInfo      = `Unsupported value: "abc": supported values: "", "Normal", "Debug", "Trace", "TraceAll"`
		updatePolicy = `"maxSurge":0,"maxUnavailable":"10%"`
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

	// author: wewang@redhat.com
	g.It("Author:wewang-Critial-24262-Image registry operator can read/overlap gloabl proxy setting [Disruptive]", func() {
		var (
			imageRegistryBaseDir = exutil.FixturePath("testdata", "image_registry")
			buildFile            = filepath.Join(imageRegistryBaseDir, "inputimage.yaml")
			buildsrc             = bcSource{
				outname:   "inputimage",
				namespace: "",
				name:      "imagesourcebuildconfig",
				template:  buildFile,
			}
		)

		g.By("Check if it's a proxy cluster")
		output, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("proxy/cluster", "-o=jsonpath={.spec}").Output()
		if !strings.Contains(output, "httpProxy") {
			g.Skip("Skip for non-proxy platform")
		}
		g.By("Start a build and pull image from internal registry")
		oc.SetupProject()
		buildsrc.namespace = oc.Namespace()
		g.By("Create buildconfig")
		buildsrc.create(oc)
		g.By("starting a build to output internal imagestream")
		err := oc.Run("start-build").Args(buildsrc.outname).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("waiting for build to finish")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), fmt.Sprintf("%s-1", buildsrc.outname), nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs(buildsrc.outname, oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("starting a build using internal registry image")
		err = oc.Run("start-build").Args(buildsrc.name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("waiting for build to finish")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildsrc.name+"-1", nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs(buildsrc.name, oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set wrong proxy to imageregistry cluster")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"proxy":{"http": "http://test:3128","https":"http://test:3128","noProxy":"test.no-proxy.com"}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Remove proxy for imageregistry cluster")
			oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec": {"proxy": null}}`, "--type=merge").Execute()
			err = wait.Poll(25*time.Second, 1*time.Minute, func() (bool, error) {
				podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
				if len(podList.Items) != 2 {
					e2e.Logf("Continue to next round")
					return false, nil
				} else {
					for _, pod := range podList.Items {
						if pod.Status.Phase != corev1.PodRunning {
							e2e.Logf("Continue to next round")
							return false, nil
						}
					}
					return true, nil
				}

			})
			o.Expect(err).NotTo(o.HaveOccurred())
			result, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "env").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).NotTo(o.ContainSubstring("HTTP_PROXY=http://test:3128"))
			o.Expect(result).NotTo(o.ContainSubstring("HTTPS_PROXY=http://test:3128"))
		}()
		err = wait.Poll(25*time.Second, 1*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 2 {
				e2e.Logf("Continue to next round")
				return false, nil
			} else {
				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						e2e.Logf("Continue to next round")
						return false, nil
					}
				}
				return true, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "env").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("HTTP_PROXY=http://test:3128"))
		o.Expect(result).To(o.ContainSubstring("HTTPS_PROXY=http://test:3128"))
		g.By("starting  a build again and waiting for failure")
		br, err := exutil.StartBuildAndWait(oc, buildsrc.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		br.AssertFailure()
		g.By("expecting the build logs to indicate the image was rejected")
		buildLog, err := br.LogsNoTimestamp()
		o.Expect(err).NotTo(o.HaveOccurred())
		errInfo := fmt.Sprintf("Error initializing source docker://image-registry.openshift-image-registry.svc:5000/%s/inputimage", oc.Namespace())
		o.Expect(buildLog).To(o.ContainSubstring(errInfo))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Critial-22893-PodAntiAffinity should work for image registry pod", func() {
		var numi, numj int
		g.By("Add podAntiAffinity in image registry config")
		err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"affinity":{"podAntiAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"podAffinityTerm":{"topologyKey":"kubernetes.io/hostname"},"weight":100}]}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"affinity":null}}`, "--type=merge").Execute()

		g.By("Set image registry replica to 3")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":3}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Set image registry replica to 2")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(50*time.Second, 2*time.Minute, func() (bool, error) {
				podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
				if len(podList.Items) != 2 {
					e2e.Logf("Continue to next round")
					return false, nil
				} else {
					for _, pod := range podList.Items {
						if pod.Status.Phase != corev1.PodRunning {
							e2e.Logf("Continue to next round")
							return false, nil
						}
					}
					return true, nil
				}
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Confirm 3 pods scaled up")
		err = wait.Poll(1*time.Minute, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 3 {
				e2e.Logf("Continue to next round")
				return false, nil
			} else {
				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						e2e.Logf("Continue to next round")
						return false, nil
					}
				}
				return true, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("At least 2 pods in different nodes")
		_, numj = comparePodHostIp(oc)
		o.Expect(numj >= 2).To(o.BeTrue())

		g.By("Set image registry replica to 4")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":4}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Set image registry replica to 2")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(50*time.Second, 2*time.Minute, func() (bool, error) {
				podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
				if len(podList.Items) != 2 {
					e2e.Logf("Continue to next round")
					return false, nil
				} else {
					for _, pod := range podList.Items {
						if pod.Status.Phase != corev1.PodRunning {
							e2e.Logf("Continue to next round")
							return false, nil
						}
					}
					return true, nil
				}
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Confirm 4 pods scaled up")
		err = wait.Poll(50*time.Second, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 4 {
				e2e.Logf("Continue to next round")
				return false, nil
			} else {
				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						e2e.Logf("Continue to next round")
						return false, nil
					}
				}
				return true, nil
			}
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check 2 pods in the same node")
		numi, _ = comparePodHostIp(oc)
		o.Expect(numi >= 1).To(o.BeTrue())
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Low-43669-Update openshift-image-registry/node-ca DaemonSet using maxUnavailable", func() {
		g.By("Check node-ca updatepolicy")
		out := getResource(oc, asAdmin, withoutNamespace, "daemonset/node-ca", "-n", "openshift-image-registry", "-o=jsonpath={.spec.updateStrategy.rollingUpdate}")
		o.Expect(out).To(o.ContainSubstring(updatePolicy))
	})

	// author: xiuwang@redhat.com
	g.It("DisconnectedOnly-Author:xiuwang-High-43715-Image registry pullthough should support pull image from the mirror registry with auth via imagecontentsourcepolicy", func() {
		g.By("Check the imagestream imported with digest id using pullthrough policy")
		out := getResource(oc, asAdmin, withoutNamespace, "is/jenkins", "-n", "openshift", "-o=jsonpath={.spec.tags[0]['from.name', 'referencePolicy.type']}")
		o.Expect(out).To(o.ContainSubstring("Local"))
		o.Expect(out).To(o.ContainSubstring("@sha256"))

		g.By("Create a pod using the imagestream")
		oc.SetupProject()
		err := oc.Run("new-app").Args("jenkins-ephemeral").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(25*time.Second, 1*time.Minute, func() (bool, error) {
			podList, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: "deploymentconfig=jenkins"})
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(podList.Items) == 1 {
				return true, nil
			}
			return false, nil

		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-27961-Create imagestreamtag with insufficient permissions [Disruptive]", func() {
		var (
			imageRegistryBaseDir = exutil.FixturePath("testdata", "image_registry")
			roleFile             = filepath.Join(imageRegistryBaseDir, "role.yaml")
			rolesrc              = authRole{
				namespace: "",
				rolename:  "tag-bug-role",
				template:  roleFile,
			}
		)
		g.By("Import an image")
		oc.SetupProject()
		err := oc.Run("import-image").Args("test-img", "--from", "registry.access.redhat.com/rhel7", "--confirm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create role with insufficient permissions")
		rolesrc.namespace = oc.Namespace()
		rolesrc.create(oc)
		err = oc.Run("create").Args("sa", "tag-bug-sa").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("policy").Args("add-role-to-user", "view", "-z", "tag-bug-sa", "--role-namespace", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("policy").Args("remove-role-from-user", "view", "tag-bug-sa", "--role-namespace", oc.Namespace()).Execute()
		out, _ := oc.Run("get").Args("sa", "tag-bug-sa", "-o=jsonpath={.secrets[0].name}", "-n", oc.Namespace()).Output()
		token, _ := oc.Run("get").Args("secret/"+out, "-o", `jsonpath={.data.\.dockercfg}`).Output()
		sDec, err := base64.StdEncoding.DecodeString(token)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("config").Args("set-credentials", "tag-bug-sa", fmt.Sprintf("--token=%s", sDec)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defuser, err := oc.Run("config").Args("get-users").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err = oc.Run("config").Args("current-context").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("config").Args("set-context", out, "--user=tag-bug-sa").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.Run("config").Args("set-context", out, "--user="+defuser).Execute()

		g.By("Create imagestreamtag with insufficient permissions")
		err = oc.AsAdmin().Run("tag").Args("test-img:latest", "test-img:v1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check if new imagestreamtag created")
		out = getResource(oc, true, withoutNamespace, "istag", "-n", oc.Namespace())
		o.Expect(out).To(o.ContainSubstring("test-img:latest"))
		o.Expect(out).To(o.ContainSubstring("test-img:v1"))
	})
})
