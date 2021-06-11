package opm

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	db "github.com/openshift/openshift-tests-private/test/extended/util/db"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] OLM opm should", func() {
	defer g.GinkgoRecover()

	var opmCLI = NewOpmCLI()

	// author: jiazha@redhat.com
	g.It("Author:jiazha-Medium-27620-Validate operator bundle Image and Contents", func() {

		bundleImages := []struct {
			image  string
			expect string
		}{
			{"quay.io/olmqe/etcd-bundle:0.9.4", "All validation tests have been completed successfully"},
			{"quay.io/olmqe/etcd-bundle:wrong", "Bundle validation errors"},
		}
		opmCLI.showInfo = true
		for _, b := range bundleImages {
			g.By(fmt.Sprintf("Validating the %s", b.image))
			output, err := opmCLI.Run("alpha").Args("bundle", "validate", "-b", "none", "-t", b.image).Output()

			if strings.Contains(output, b.expect) {
				e2e.Logf(fmt.Sprintf("That's expected! %s", b.image))
			} else {
				e2e.Failf(fmt.Sprintf("Failed to validating the %s, error: %v", b.image, err))
			}

		}

	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-Medium-34016-opm can prune operators from catalog", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		indexDB := filepath.Join(opmBaseDir, "index_34016.db")
		output, err := opmCLI.Run("registry").Args("prune", "-d", indexDB, "-p", "lib-bucket-provisioner").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "deleting packages") || !strings.Contains(output, "pkg=planetscale") {
			e2e.Failf(fmt.Sprintf("Failed to obtain the removed packages from prune : %s", output))
		}
	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-VMonly-Low-30318-Bundle build understands packages", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		testDataPath := filepath.Join(opmBaseDir, "aqua")
		opmCLI.ExecCommandPath = testDataPath
		defer DeleteDir(testDataPath, "fixture-testdata")

		g.By("step: opm alpha bundle generate")
		output, err := opmCLI.Run("alpha").Args("bundle", "generate", "-d", "1.0.1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		if !strings.Contains(output, "Writing annotations.yaml") || !strings.Contains(output, "Writing bundle.Dockerfile") {
			e2e.Failf("Failed to execute opm alpha bundle generate : %s", output)
		}
	})
})

var _ = g.Describe("[sig-operators] OLM opm with podman", func() {
	defer g.GinkgoRecover()

	var podmanCLI = container.NewPodmanCLI()
	var opmCLI = NewOpmCLI()
	var sqlit = db.NewSqlit()
	var quayCLI = container.NewQuayCLI()
	var oc = exutil.NewCLI("vmonly-"+getRandomString(), exutil.KubeConfigPath())

	// author: xzha@redhat.com
	g.It("Author:xzha-VMonly-Medium-25955-opm Ability to generate scaffolding for Operator Bundle", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "learn_operator")
		opmCLI.ExecCommandPath = TestDataPath
		defer DeleteDir(TestDataPath, "fixture-testdata")
		imageTag := "quay.io/olmqe/25955-operator-" + getRandomString() + ":v0.0.1"

		g.By("step: opm alpha bundle generate")
		output, err := opmCLI.Run("alpha").Args("bundle", "generate", "-d", "package/0.0.1", "-p", "25955-operator", "-c", "alpha", "-e", "alpha").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(output)
		if !strings.Contains(output, "Writing annotations.yaml") || !strings.Contains(output, "Writing bundle.Dockerfile") {
			e2e.Failf("Failed to execute opm alpha bundle generate : %s", output)
		}

		g.By("step: opm alpha bundle build")
		e2e.Logf("clean test data")
		DeleteDir(TestDataPath, "fixture-testdata")
		opmBaseDir = exutil.FixturePath("testdata", "opm")
		TestDataPath = filepath.Join(opmBaseDir, "learn_operator")
		opmCLI.ExecCommandPath = TestDataPath
		_, err = podmanCLI.RemoveImage(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("run opm alpha bundle build")
		defer podmanCLI.RemoveImage(imageTag)
		output, _ = opmCLI.Run("alpha").Args("bundle", "build", "-d", "package/0.0.1", "-b", "podman", "--tag", imageTag, "-p", "25955-operator", "-c", "alpha", "-e", "alpha", "--overwrite").Output()
		e2e.Logf(output)
		if !strings.Contains(output, "COMMIT "+imageTag) {
			e2e.Failf("Failed to execute opm alpha bundle build : %s", output)
		}

		e2e.Logf("step: check image %s exist", imageTag)
		existFlag, err := podmanCLI.CheckImageExist(imageTag)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("check image exist is %v", existFlag)
		o.Expect(existFlag).To(o.BeTrue())
	})

	// author: xzha@redhat.com
	g.It("Author:xzha-VMonly-Medium-37294-OPM can strand packages with prune stranded", func() {
		containerTool := "podman"
		containerCLI := podmanCLI
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "temp")
		opmCLI.ExecCommandPath = TestDataPath
		defer DeleteDir(TestDataPath, "fixture-testdata")
		indexImage := "quay.io/olmqe/etcd-index:test-37294"
		indexImageSemver := "quay.io/olmqe/etcd-index:test-37294-semver"

		g.By("step: check etcd-index:test-37294, operatorbundle has two records, channel_entry has one record")
		indexdbpath1 := filepath.Join(TestDataPath, getRandomString())
		err := os.Mkdir(TestDataPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.Mkdir(indexdbpath1, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImage, "--path", "/database/index.db:"+indexdbpath1).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath1, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err := sqlit.DBMatch(path.Join(indexdbpath1, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath1, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("step: prune-stranded this index image")
		indexImageTmp1 := indexImage + getRandomString()
		defer containerCLI.RemoveImage(indexImageTmp1)
		output, err := opmCLI.Run("index").Args("prune-stranded", "-f", indexImage, "--tag", indexImageTmp1, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexImageTmp1).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(indexImageTmp1, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: check index image operatorbundle has one record")
		indexdbpath2 := filepath.Join(TestDataPath, getRandomString())
		err = os.Mkdir(indexdbpath2, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImageTmp1, "--path", "/database/index.db:"+indexdbpath2).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath2, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.DBMatch(path.Join(indexdbpath2, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath2, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("test 2")
		g.By("step: step: check etcd-index:test-37294-semver, operatorbundle has two records, channel_entry has two records")
		indexdbpath3 := filepath.Join(TestDataPath, getRandomString())
		err = os.Mkdir(indexdbpath3, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImageSemver, "--path", "/database/index.db:"+indexdbpath3).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath3, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.DBMatch(path.Join(indexdbpath3, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath3, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("step: prune-stranded this index image")
		indexImageTmp2 := indexImage + getRandomString()
		defer containerCLI.RemoveImage(indexImageTmp2)
		output, err = opmCLI.Run("index").Args("prune-stranded", "-f", indexImageSemver, "--tag", indexImageTmp2, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexImageTmp2).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(indexImageTmp2, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: check index image has both v0.9.2 and v0.9.2")
		indexdbpath4 := filepath.Join(TestDataPath, getRandomString())
		err = os.Mkdir(indexdbpath4, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImageTmp2, "--path", "/database/index.db:"+indexdbpath4).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexdbpath4, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err = sqlit.DBMatch(path.Join(indexdbpath4, "index.db"), "operatorbundle", "name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBMatch(path.Join(indexdbpath4, "index.db"), "channel_entry", "operatorbundle_name", []string{"etcdoperator.v0.9.0", "etcdoperator.v0.9.2"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		e2e.Logf("step: check index image has both v0.9.2 and v0.9.2 SUCCESS")
	})

	// author: xzha@redhat.com
	g.It("Author:xzha-VMonly-Medium-40530-The index image generated by opm index prune should not leave unrelated images", func() {
		containerCLI := podmanCLI
		containerTool := "podman"
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "temp")
		opmCLI.ExecCommandPath = TestDataPath
		defer DeleteDir(TestDataPath, "fixture-testdata")
		indexImage := "quay.io/olmqe/redhat-operator-index:40530"
		defer containerCLI.RemoveImage(indexImage)

		g.By("step: check the index image has other bundles except cluster-logging")
		indexTmpPath1 := filepath.Join(TestDataPath, getRandomString())
		err := os.MkdirAll(indexTmpPath1, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImage, "--path", "/database/index.db:"+indexTmpPath1).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexTmpPath1, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())

		rows, err := sqlit.QueryDB(path.Join(indexTmpPath1, "index.db"), "select distinct(operatorbundle_name) from related_image where operatorbundle_name not in (select operatorbundle_name from channel_entry)")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer rows.Close()
		var OperatorBundles []string
		var name string
		for rows.Next() {
			rows.Scan(&name)
			OperatorBundles = append(OperatorBundles, name)
		}
		o.Expect(OperatorBundles).NotTo(o.BeEmpty())

		g.By("step: Prune the index image to keep cluster-logging only")
		indexImage1 := indexImage + getRandomString()
		defer containerCLI.RemoveImage(indexImage1)
		output, err := opmCLI.Run("index").Args("prune", "-f", indexImage, "-p", "cluster-logging", "-t", indexImage1, "-c", containerTool).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = containerCLI.Run("push").Args(indexImage1).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(indexImage1, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: check database, there is no related images")
		indexTmpPath2 := filepath.Join(TestDataPath, getRandomString())
		err = os.MkdirAll(indexTmpPath2, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", indexImage1, "--path", "/database/index.db:"+indexTmpPath2).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexTmpPath2, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())

		rows2, err := sqlit.QueryDB(path.Join(indexTmpPath2, "index.db"), "select distinct(operatorbundle_name) from related_image where operatorbundle_name not in (select operatorbundle_name from channel_entry)")
		o.Expect(err).NotTo(o.HaveOccurred())
		OperatorBundles = nil
		defer rows2.Close()
		for rows2.Next() {
			rows2.Scan(&name)
			OperatorBundles = append(OperatorBundles, name)
		}
		o.Expect(OperatorBundles).To(o.BeEmpty())

		g.By("step: check the image mirroring mapping")
		manifestsPath := filepath.Join(TestDataPath, getRandomString())
		output, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("catalog", "mirror", indexImage1, "localhost:5000", "--manifests-only", "--to-manifests="+manifestsPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("/database/index.db"))

		result, err := exec.Command("bash", "-c", "cat "+manifestsPath+"/mapping.txt").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).NotTo(o.BeEmpty())

		result, _ = exec.Command("bash", "-c", "cat "+manifestsPath+"/mapping.txt|grep -v ose-cluster-logging|grep -v ose-logging|grep -v redhat-operator-index:40530").Output()
		o.Expect(result).To(o.BeEmpty())
		g.By("step: 40530 SUCCESS")

	})

	// author: bandrade@redhat.com
	g.It("Author:bandrade-VMonly-Medium-34049-opm can prune operators from index", func() {
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "temp")
		indexTmpPath := filepath.Join(TestDataPath, getRandomString())
		defer DeleteDir(TestDataPath, indexTmpPath)
		err := os.MkdirAll(indexTmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		containerCLI := podmanCLI
		containerTool := "podman"
		sourceImageTag := "quay.io/olmqe/multi-index:2.0"
		imageTag := "quay.io/olmqe/multi-index:3.0." + getRandomString()
		defer podmanCLI.RemoveImage(imageTag)
		defer podmanCLI.RemoveImage(sourceImageTag)
		output, err := opmCLI.Run("index").Args("prune", "-f", sourceImageTag, "-p", "planetscale", "-t", imageTag, "-c", containerTool).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "deleting packages") || !strings.Contains(output, "pkg=lib-bucket-provisioner") {
			e2e.Failf(fmt.Sprintf("Failed to obtain the removed packages from prune : %s", output))
		}

		output, err = containerCLI.Run("push").Args(imageTag).Output()
		if err != nil {
			e2e.Logf(output)
		}
		defer quayCLI.DeleteTag(strings.Replace(imageTag, "quay.io/", "", 1))
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", imageTag, "--path", "/database/index.db:"+indexTmpPath).Output()
		e2e.Logf("get index.db SUCCESS, path is %s", path.Join(indexTmpPath, "index.db"))
		o.Expect(err).NotTo(o.HaveOccurred())

		result, err := sqlit.DBMatch(path.Join(indexTmpPath, "index.db"), "channel_entry", "operatorbundle_name", []string{"lib-bucket-provisioner.v1.0.0"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeFalse())

	})

	g.It("Author:xzha-VMonly-Medium-26594-Related Images", func() {
		var initializerCLI = NewInitializer()
		opmBaseDir := exutil.FixturePath("testdata", "opm")
		TestDataPath := filepath.Join(opmBaseDir, "etcd_operator", "etcd_community")
		TmpDataPath := filepath.Join(opmBaseDir, "tmp")
		err := os.MkdirAll(TmpDataPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		initializerCLI.ExecCommandPath = TmpDataPath
		defer exec.Command("kill", "-9", "$(lsof -t -i:26594)").Output()
		defer DeleteDir(TestDataPath, "fixture-testdata")

		g.By("step: Run the initializer binary to get a database.")
		dbFilePath := filepath.Join(TmpDataPath, "bundles.db")
		output, err := initializerCLI.Run("-m").Args(TestDataPath, "-o", dbFilePath).Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: Check if the related images stores in this database")
		image := "quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b"

		result, err := sqlit.DBMatch(dbFilePath, "package", "name", []string{"etcd"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())
		result, err = sqlit.DBHas(dbFilePath, "related_image", "image", []string{image})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.BeTrue())

		g.By("step: Run the registry-server binary to load manifest and serves a grpc API to query it.")
		e2e.Logf("step: Run the registry-server ")
		cmd := exec.Command("registry-server", "-d", dbFilePath, "-t", filepath.Join(TmpDataPath, "26594.log"), "-p", "26594")
		cmd.Dir = TmpDataPath
		err = cmd.Start()
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(time.Second * 1)
		e2e.Logf("step: check api.Registry/ListPackages")
		outputCurl, err := exec.Command("grpcurl", "-plaintext", "localhost:26594", "api.Registry/ListPackages").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(outputCurl)).To(o.ContainSubstring("etcd"))
		e2e.Logf("step: check api.Registry/GetBundleForChannel")
		outputCurl, err = exec.Command("grpcurl", "-plaintext", "-d", "{\"pkgName\":\"etcd\",\"channelName\":\"singlenamespace-alpha\"}", "localhost:26594", "api.Registry/GetBundleForChannel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(outputCurl)).To(o.ContainSubstring(image))
		outputCurl, err = exec.Command("grpcurl", "-plaintext", "-d", "{\"pkgName\":\"etcd\",\"channelName\":\"clusterwide-alpha\"}", "localhost:26594", "api.Registry/GetBundleForChannel").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(outputCurl)).To(o.ContainSubstring(image))
		cmd.Process.Kill()
		g.By("step: SUCCESS")

	})

})
