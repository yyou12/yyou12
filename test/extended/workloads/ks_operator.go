package workloads

import (
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-apps] Workloads", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: yinzhou@redhat.com
	//It is destructive case, will make kube-scheduler roll out, so adding [Disruptive]. One rollout costs about 5mins, so adding [Slow]
	g.It("Author:yinzhou-Medium-31939-Verify logLevel settings in kube scheduler operator [Disruptive][Slow]", func() {
		patchYamlToRestore := `[{"op": "replace", "path": "/spec/logLevel", "value":"Normal"}]`

		g.By("Set the loglevel to TraceAll")
		patchYamlTraceAll := `[{"op": "replace", "path": "/spec/logLevel", "value":"TraceAll"}]`
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			e2e.Logf("Restoring the scheduler cluster's logLevel")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check the scheduler operator should be in Progressing")
			err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
				if err != nil {
					e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
					e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Wait for the scheduler operator to rollout")
			err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
				if err != nil {
					e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
					e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		g.By("Check the scheduler operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for the scheduler operator to rollout")
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the loglevel setting for the pod")
		output, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pods", "-n", "openshift-kube-scheduler", "-l", "app=openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString("-v=10", output); matched {
			e2e.Logf("clusteroperator kube-scheduler is running with logLevel 10\n")
		}

		g.By("Set the loglevel to Trace")
		patchYamlTrace := `[{"op": "replace", "path": "/spec/logLevel", "value":"Trace"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlTrace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the scheduler operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for the scheduler operator to rollout")
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the loglevel setting for the pod")
		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("pods", "-n", "openshift-kube-scheduler", "-l", "app=openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString("-v=6", output); matched {
			e2e.Logf("clusteroperator kube-scheduler is running with logLevel 6\n")
		}

		g.By("Set the loglevel to Debug")
		patchYamlDebug := `[{"op": "replace", "path": "/spec/logLevel", "value":"Debug"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlDebug).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the scheduler operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for the scheduler operator to rollout")
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the loglevel setting for the pod")
		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("pods", "-n", "openshift-kube-scheduler", "-l", "app=openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString("-v=4", output); matched {
			e2e.Logf("clusteroperator kube-scheduler is running with logLevel 4\n")
		}
	})
})
