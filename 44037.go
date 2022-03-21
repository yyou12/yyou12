// author: yyou@redhat.com
	g.It("VMonly-Author:yyou-Critical-44037-Could configure swift authentication using application credentials ", func() {

		g.By("Check secret of openshift-image-registry ")
		output, err := oc.AsAdmin().Run("get").Args("secret", "image-registry-private-configuration", "-n", "openshift-image-registry", "-o=jsonpath={.data.credentials}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)

		g.By("Check image registry pod")
		pod, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.data.credentials}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(pod)

		g.By("Configure image-registry-private-configuration  secret to use new application credentials")
		secret, err := oc.AsAdmin().WithoutNamespace().Run("set").Args("data", "secret/pull-secret", "-n", "openshift-config", "--from-file=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALID=applicationid --from-file=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALNAME=applicationname --from-file=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALSECRET=applicationsecret", "-n", "openshift-image-registry").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(secret)

		g.By("Check the new secret")
		credentials, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/installer-cloud-credentials", "-n", "openshift-image-registry", "-o=jsonpath={.data.credentials}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Sprintf("credentials is %s", credentials)
		e2e.Logf(credentials)

		g.By("Check the pods are running")
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=example-statefulset", 3)

		g.By("Create a build app")
		oc.SetupProject()
		app := oc.Run("new-app").Args("httpd:latest~http://github.com/sclorg/httpd-ex").Execute()
		o.Expect(app).NotTo(o.HaveOccurred())

		g.By("push/pull image to registry")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-44037", oc.Namespace())

	})
