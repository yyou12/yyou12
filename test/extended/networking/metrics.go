package networking

import (
	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-metrics", exutil.KubeConfigPath())

	g.It("Author:weliang-Medium-47524-Metrics for ovn-appctl stopwatch/show command.", func() {
		var (
			namespace = "openshift-ovn-kubernetes"
			cmName    = "ovn-kubernetes-master"
		)
		leaderNodeIP := getLeaderNodeIP(oc, namespace, cmName)
		prometheus_url := "https://" + leaderNodeIP + ":9105/metrics"
		metric_name1 := "ovn_controller_if_status_mgr_run_total_samples"
		metric_name2 := "ovn_controller_if_status_mgr_run_long_term_avg"
		metric_name3 := "ovn_controller_bfd_run_total_samples"
		metric_name4 := "ovn_controller_bfd_run_long_term_avg"
		metric_name5 := "ovn_controller_flow_installation_total_samples"
		metric_name6 := "ovn_controller_flow_installation_long_term_avg"
		metric_name7 := "ovn_controller_if_status_mgr_run_total_samples"
		metric_name8 := "ovn_controller_if_status_mgr_run_long_term_avg"
		metric_name9 := "ovn_controller_if_status_mgr_update_total_samples"
		metric_name10 := "ovn_controller_if_status_mgr_update_long_term_avg"
		metric_name11 := "ovn_controller_flow_generation_total_samples"
		metric_name12 := "ovn_controller_flow_generation_long_term_avg"
		metric_name13 := "ovn_controller_pinctrl_run_total_samples"
		metric_name14 := "ovn_controller_pinctrl_run_long_term_avg"
		metric_name15 := "ovn_controller_ofctrl_seqno_run_total_samples"
		metric_name16 := "ovn_controller_ofctrl_seqno_run_long_term_avg"
		metric_name17 := "ovn_controller_patch_run_total_samples"
		metric_name18 := "ovn_controller_patch_run_long_term_avg"
		metric_name19 := "ovn_controller_ct_zone_commit_total_samples"
		metric_name20 := "ovn_controller_ct_zone_commit_long_term_avg"
		checkSDNMetrics(oc, prometheus_url, metric_name1)
		checkSDNMetrics(oc, prometheus_url, metric_name2)
		checkSDNMetrics(oc, prometheus_url, metric_name3)
		checkSDNMetrics(oc, prometheus_url, metric_name4)
		checkSDNMetrics(oc, prometheus_url, metric_name5)
		checkSDNMetrics(oc, prometheus_url, metric_name6)
		checkSDNMetrics(oc, prometheus_url, metric_name7)
		checkSDNMetrics(oc, prometheus_url, metric_name8)
		checkSDNMetrics(oc, prometheus_url, metric_name9)
		checkSDNMetrics(oc, prometheus_url, metric_name10)
		checkSDNMetrics(oc, prometheus_url, metric_name11)
		checkSDNMetrics(oc, prometheus_url, metric_name12)
		checkSDNMetrics(oc, prometheus_url, metric_name13)
		checkSDNMetrics(oc, prometheus_url, metric_name14)
		checkSDNMetrics(oc, prometheus_url, metric_name15)
		checkSDNMetrics(oc, prometheus_url, metric_name16)
		checkSDNMetrics(oc, prometheus_url, metric_name17)
		checkSDNMetrics(oc, prometheus_url, metric_name18)
		checkSDNMetrics(oc, prometheus_url, metric_name19)
		checkSDNMetrics(oc, prometheus_url, metric_name20)
	})

	g.It("Author:weliang-Medium-47471-Record update to cache versus port binding.", func() {
		var (
			namespace = "openshift-ovn-kubernetes"
			cmName    = "ovn-kubernetes-master"
		)
		leaderNodeIP := getLeaderNodeIP(oc, namespace, cmName)
		prometheus_url := "https://" + leaderNodeIP + ":9102/metrics"
		metric_name1 := "ovnkube_master_pod_first_seen_lsp_created_duration_seconds_count"
		metric_name2 := "ovnkube_master_pod_lsp_created_port_binding_duration_seconds_count"
		metric_name3 := "ovnkube_master_pod_port_binding_port_binding_chassis_duration_seconds_count"
		metric_name4 := "ovnkube_master_pod_port_binding_chassis_port_binding_up_duration_seconds_count"
		checkSDNMetrics(oc, prometheus_url, metric_name1)
		checkSDNMetrics(oc, prometheus_url, metric_name2)
		checkSDNMetrics(oc, prometheus_url, metric_name3)
		checkSDNMetrics(oc, prometheus_url, metric_name4)
	})

	g.It("Author:weliang-Medium-45841-Add OVN flow count metric.", func() {
		var (
			namespace = "openshift-ovn-kubernetes"
			cmName    = "ovn-kubernetes-master"
		)
		leaderNodeIP := getLeaderNodeIP(oc, namespace, cmName)
		prometheus_url := "https://" + leaderNodeIP + ":9105/metrics"
		metric_name := "ovn_controller_integration_bridge_openflow"
		checkSDNMetrics(oc, prometheus_url, metric_name)
	})
})

