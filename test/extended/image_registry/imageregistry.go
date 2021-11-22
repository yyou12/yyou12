package image_registry

import (
	"encoding/base64"
	"fmt"
	"os/exec"
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
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec": {"proxy": null}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
			result, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "env").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).NotTo(o.ContainSubstring("HTTP_PROXY=http://test:3128"))
			o.Expect(result).NotTo(o.ContainSubstring("HTTPS_PROXY=http://test:3128"))
		}()
		recoverRegistryDefaultPods(oc)
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
		g.Skip("According devel comments: https://bugzilla.redhat.com/show_bug.cgi?id=2014940, still not work,when find a solution, will enable it")
		g.By("Check platforms")
		platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		platforms := map[string]bool{
			"AWS":       true,
			"Azure":     true,
			"GCP":       true,
			"OpenStack": true,
		}
		if !platforms[platformtype] {
			g.Skip("Skip for non-supported platform")
		}

		var numi, numj int
		g.By("Add podAntiAffinity in image registry config")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"affinity":{"podAntiAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"podAffinityTerm":{"topologyKey":"kubernetes.io/hostname"},"weight":100}]}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"affinity":null}}`, "--type=merge").Execute()

		g.By("Set image registry replica to 3")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":3}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Set image registry replica to 2")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
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
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 3")

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
			recoverRegistryDefaultPods(oc)
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
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 4")

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
		exutil.AssertWaitPollNoErr(err, "Pulling image via icsp is failed")
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

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Medium-43664-Check ServiceMonitor of registry which will not hotloop CVO", func() {
		g.By("Check the servicemonitor of openshift-image-registry")
		out := getResource(oc, asAdmin, withoutNamespace, "servicemonitor", "-n", "openshift-image-registry", "-o=jsonpath={.items[1].spec.selector.matchLabels.name}")
		o.Expect(out).To(o.ContainSubstring("image-registry-operator"))

		g.By("Check CVO not hotloop due to registry")
		masterlogs, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("node-logs", "--role", "master", "--path=kube-apiserver/audit.log", "--raw").OutputToFile("audit.log")
		o.Expect(err).NotTo(o.HaveOccurred())

		result, err := exec.Command("bash", "-c", "cat "+masterlogs+" | grep verb.*update.*resource.*servicemonitors").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).NotTo(o.ContainSubstring("image-registry"))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-27985-Image with invalid resource name can be pruned", func() {
		g.By("Config image registry to emptydir")
		defer recoverRegistryStorageConfig(oc)
		defer recoverRegistryDefaultReplicas(oc)
		configureRegistryStorageToEmptyDir(oc)

		g.By("Import image to internal registry")
		oc.SetupProject()
		var invalidInfo = "Invalid image name foo/bar/" + oc.Namespace() + "/ruby-hello-world"
		err := oc.Run("new-build").Args("openshift/ruby:latest~https://github.com/openshift/ruby-hello-world.git").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "ruby-hello-world-1", nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs("ruby-hello-world", oc)
		}
		exutil.AssertWaitPollNoErr(err, "build is not complete")

		g.By("Add system:image-pruner role to system:serviceaccount:openshift-image-registry:registry")
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "system:image-pruner", "system:serviceaccount:openshift-image-registry:registry").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-cluster-role-from-user", "system:image-pruner", "system:serviceaccount:openshift-image-registry:registry").Execute()

		g.By("Check invaild image source can be pruned")
		err = oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "mkdir", "-p", "/registry/docker/registry/v2/repositories/foo/bar").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "cp", "-r", "/registry/docker/registry/v2/repositories/"+oc.Namespace(), "/registry/docker/registry/v2/repositories/foo/bar").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "/bin/bash", "-c", "/usr/bin/dockerregistry -prune=check").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(invalidInfo))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-High-41414-There are 2 replicas for image registry on HighAvailable workers S3/Azure/GCS/Swift storage", func() {
		g.By("Check image registry pod")
		platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		switch platformtype {
		case "AWS", "Azure", "GCP", "OpenStack":
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			o.Expect(len(podList.Items)).To(o.Equal(2))
			oc.SetupProject()
			err := oc.Run("new-build").Args("openshift/ruby:latest~https://github.com/openshift/ruby-hello-world.git").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "ruby-hello-world-1", nil, nil, nil)
			if err != nil {
				exutil.DumpBuildLogs("ruby-hello-world", oc)
			}
			exutil.AssertWaitPollNoErr(err, "build is not complete")
		default:
			g.Skip("Skip for other clusters!")
		}
	})

	//author: xiuwang@redhat.com
	g.It("Author:xiuwang-Critial-34680-Image registry storage cannot be removed if set to Unamanaged when image registry is set to Removed [Disruptive]", func() {
		g.By("Get registry storage info")
		var storageinfo1, storageinfo2, storageinfo3 string
		storageinfo1 = restoreRegistryStorageConfig(oc)
		g.By("Set image registry storage to Unmanaged, image registry operator to Removed")
		err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Removed","storage":{"managementState":"Unmanaged"}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Recover image registry change")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed","storage":{"managementState":"Managed"}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
		}()
		err = wait.Poll(25*time.Second, 2*time.Minute, func() (bool, error) {
			podList, err1 := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if err1 != nil {
				e2e.Logf("Error listing pods: %v", err)
				return false, nil
			}
			if len(podList.Items) != 0 {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry is not removed")
		storageinfo2 = restoreRegistryStorageConfig(oc)
		if strings.Compare(storageinfo1, storageinfo2) != 0 {
			e2e.Failf("Image stroage has changed")
		}
		g.By("Set image registry operator to Managed again")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(25*time.Second, 2*time.Minute, func() (bool, error) {
			podList, err1 := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if err1 != nil {
				e2e.Logf("Error listing pods: %v", err)
				return false, nil
			}
			if len(podList.Items) == 0 {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry is not recovered")
		storageinfo3 = restoreRegistryStorageConfig(oc)
		if strings.Compare(storageinfo1, storageinfo3) != 0 {
			e2e.Failf("Image stroage has changed")
		}
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Critical-21593-Check registry status by changing managementState for image-registry [Disruptive]", func() {
		g.By("Check platforms")
		platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		platforms := map[string]bool{
			"AWS":       true,
			"Azure":     true,
			"GCP":       true,
			"OpenStack": true,
		}
		if !platforms[platformtype] {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Change managementSet from Managed -> Removed")
		defer func() {
			g.By("Set image registry cluster Managed")
			oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Execute()
			recoverRegistryDefaultPods(oc)
		}()
		err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Removed"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check image-registry pods are removed")
		checkRegistrypodsRemoved(oc)

		g.By("Change managementSet from Removed to Managed")
		err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		recoverRegistryDefaultPods(oc)

		g.By("Change managementSet from Managed to Unmanaged")
		err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Unmanaged"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Update replicas to 1")
		defer oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas": 2}}`, "--type=merge").Execute()
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas": 1}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check image registry pods are still 2")
		podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(podList.Items)).Should(o.Equal(2))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-High-45952-Imported imagestreams should success in deploymentconfig", func() {
		var (
			imageRegistryBaseDir = exutil.FixturePath("testdata", "image_registry")
			statefulsetFile      = filepath.Join(imageRegistryBaseDir, "statefulset.yaml")
			statefulsetsrc       = staSource{
				namespace: "",
				name:      "example-statefulset",
				template:  statefulsetFile,
			}
		)
		g.By("Import an image stream and set image-lookup")
		oc.SetupProject()
		err := oc.Run("import-image").Args("registry.access.redhat.com/ubi8/ubi", "--scheduled", "--confirm", "--reference-policy=local").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "ubi", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set").Args("image-lookup", "ubi").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the initial statefulset")
		statefulsetsrc.namespace = oc.Namespace()
		g.By("Create statefulset")
		statefulsetsrc.create(oc)
		g.By("Check the pods are running")
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=example-statefulset", 3)

		g.By("Reapply the sample yaml")
		applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", statefulsetsrc.template, "-p", "NAME="+statefulsetsrc.name, "NAMESPACE="+statefulsetsrc.namespace)
		g.By("Check the pods are running")
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=example-statefulset", 3)

		g.By("setting a trigger, pods are still running")
		err = oc.Run("set").Args("triggers", "statefulset/example-statefulset", "--from-image=ubi:latest", "--containers", "example-statefulset").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check the pods are running")
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=example-statefulset", 3)
		interReg := "image-registry.openshift-image-registry.svc:5000/" + oc.Namespace() + "/ubi"
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("pods", "-o=jsonpath={.items[*].spec.containers[*].image}", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(interReg))
	})
})
