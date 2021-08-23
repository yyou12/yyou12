package mco

import (
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-mco] MCO", func() {

	var oc = exutil.NewCLI("mco", exutil.KubeConfigPath())

	g.It("Author:rioliu-Critical-42347-health check for machine-config-operator [Serial]", func() {
		g.By("checking mco status")

		coStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/machine-config", "-o", "jsonpath='{range .status.conditions[*]}{.type}{.status}{\"\\n\"}{end}'").Output()
		e2e.Logf(coStatus)

		o.Expect(coStatus).Should(o.ContainSubstring("ProgressingFalse"))
		o.Expect(coStatus).Should(o.ContainSubstring("UpgradeableTrue"))
		o.Expect(coStatus).Should(o.ContainSubstring("DegradedFalse"))
		o.Expect(coStatus).Should(o.ContainSubstring("AvailableTrue"))

		e2e.Logf("mco operator is healthy")

		g.By("checking mco pod status")

		podStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-machine-config-operator", "-o", "jsonpath='{.items[*].status.conditions[?(@.type==\"Ready\")].status}'").Output()
		e2e.Logf(podStatus)

		o.Expect(podStatus).ShouldNot(o.ContainSubstring("False"))

		e2e.Logf("mco pods are healthy")

		g.By("checking mcp status")

		mcpStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "-o", "jsonpath='{.items[*].status.conditions[?(@.type==\"Degraded\")].status}'").Output()
		e2e.Logf(mcpStatus)

		o.Expect(mcpStatus).ShouldNot(o.ContainSubstring("True"))

		e2e.Logf("mcps are not degraded")

	})

	g.It("Author:rioliu-Longduration-CPaasrunOnly-Critical-42361-add chrony systemd config [Disruptive]", func() {
		g.By("create new mc to apply chrony config on worker nodes")

		mcName := "change-workers-chrony-configuration"
		mcTemplate := generateTemplateAbsolutePath("change-workers-chrony-configuration.yaml")
		mc := machineConfig{name: mcName, template: mcTemplate, pool: "worker"}
		mc.create(oc)

		g.By("get one worker node to verify the config changes")
		nodeName, err := getFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		stdout, err := debugNode(oc, strings.Trim(nodeName, "'"), "cat /etc/chrony.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf(stdout)
		o.Expect(stdout).Should(o.ContainSubstring("pool 0.rhel.pool.ntp.org iburst"))
		o.Expect(stdout).Should(o.ContainSubstring("driftfile /var/lib/chrony/drift"))
		o.Expect(stdout).Should(o.ContainSubstring("makestep 1.0 3"))
		o.Expect(stdout).Should(o.ContainSubstring("rtcsync"))
		o.Expect(stdout).Should(o.ContainSubstring("logdir /var/log/chrony"))

		mc.delete(oc)
	})
})
