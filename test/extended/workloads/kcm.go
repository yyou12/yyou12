package workloads

import (
	"time"
	"regexp"
	"strings"
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
	g.It("Author:yinzhou-High-28001-bug 1749478 KCM should recover when its temporary secrets are deleted [Disruptive]", func() {
		var namespace = "openshift-kube-controller-manager"
		var temporarySecretsList []string

		g.By("get all the secrets in kcm project")
		output, err := oc.AsAdmin().Run("get").Args("secrets", "-n", namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		secretsList := strings.Fields(output)
		
		g.By("filter out all the none temporary secrets")
		for _, secretsname := range secretsList {
			secretsAnnotations, err := oc.AsAdmin().Run("get").Args("secrets", "-n", namespace, secretsname, "-o=jsonpath={.metadata.annotations}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if matched, _ := regexp.MatchString("kubernetes.io/service-account.name", secretsAnnotations); matched {
				continue
			} else {
				secretOwnerKind, err := oc.AsAdmin().Run("get").Args("secrets", "-n", namespace, secretsname,  "-o=jsonpath={.metadata.ownerReferences[0].kind}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Compare(secretOwnerKind,"ConfigMap") == 0 {
					continue
				} else {
					temporarySecretsList = append(temporarySecretsList, secretsname)
				}
			}
		}

		g.By("delete all the temporary secrets")
		for _, secretsD := range temporarySecretsList {
			_, err = oc.AsAdmin().Run("delete").Args("secrets", "-n", namespace, secretsD).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Check the KCM operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("co", "kube-controller-manager").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-controller-manager not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-controller-manager is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for the KCM operator to recover")
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("co", "kube-controller-manager").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-controller-manager, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-controller-manager is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
