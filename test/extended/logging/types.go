package logging

import (
	v1 "k8s.io/api/core/v1"
)

/*
{
  "took" : 75,
  "timed_out" : false,
  "_shards" : {
    "total" : 14,
    "successful" : 14,
    "skipped" : 0,
    "failed" : 0
  },
  "hits" : {
    "total" : 63767,
    "max_score" : 1.0,
    "hits" : [
      {
        "_index" : "app-centos-logtest-000001",
        "_type" : "_doc",
        "_id" : "ODlhMmYzZDgtMTc4NC00M2Q0LWIyMGQtMThmMGY3NTNlNWYw",
        "_score" : 1.0,
        "_source" : {
          "kubernetes" : {
            "container_image_id" : "quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4",
            "container_name" : "centos-logtest",
            "namespace_id" : "c74f42bb-3407-418a-b483-d5f33e08f6a5",
            "flat_labels" : [
              "run=centos-logtest",
              "test=centos-logtest"
            ],
            "host" : "ip-10-0-174-131.us-east-2.compute.internal",
            "master_url" : "https://kubernetes.default.svc",
            "pod_id" : "242e7eb4-47ca-4708-9993-db0390d18268",
            "namespace_labels" : {
              "kubernetes_io/metadata_name" : "e2e-test--lg56q"
            },
            "container_image" : "quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4",
            "namespace_name" : "e2e-test--lg56q",
            "pod_name" : "centos-logtest-vnwjn"
          },
          "viaq_msg_id" : "ODlhMmYzZDgtMTc4NC00M2Q0LWIyMGQtMThmMGY3NTNlNWYw",
          "level" : "unknown",
          "message" : "{\"message\": \"MERGE_JSON_LOG=true\", \"level\": \"debug\",\"Layer1\": \"layer1 0\", \"layer2\": {\"name\":\"Layer2 1\", \"tips\":\"Decide by PRESERVE_JSON_LOG\"}, \"StringNumber\":\"10\", \"Number\": 10,\"foo.bar\":\"Dot Item\",\"{foobar}\":\"Brace Item\",\"[foobar]\":\"Bracket Item\", \"foo:bar\":\"Colon Item\",\"foo bar\":\"Space Item\" }",
          "docker" : {
            "container_id" : "b3b84d9f11cefa8abf335e8257e394414133b853dc65c8bc1d50120fc3f86da5"
          },
          "hostname" : "ip-10-0-174-131.us-east-2.compute.internal",
          "@timestamp" : "2021-07-09T01:57:44.400169+00:00",
          "pipeline_metadata" : {
            "collector" : {
              "received_at" : "2021-07-09T01:57:44.688935+00:00",
              "name" : "fluentd",
              "inputname" : "fluent-plugin-systemd",
              "version" : "1.7.4 1.6.0",
              "ipaddr4" : "10.0.174.131"
            }
          },
          "structured" : {
            "foo:bar" : "Colon Item",
            "foo.bar" : "Dot Item",
            "Number" : 10,
            "level" : "debug",
            "{foobar}" : "Brace Item",
            "foo bar" : "Space Item",
            "StringNumber" : "10",
            "layer2" : {
              "name" : "Layer2 1",
              "tips" : "Decide by PRESERVE_JSON_LOG"
            },
            "message" : "MERGE_JSON_LOG=true",
            "Layer1" : "layer1 0",
            "[foobar]" : "Bracket Item"
          }
        }
      }
    ]
  }
}
*/
type SearchResult struct {
	Took     string `json:"took"`
	TimedOut bool   `json:"timed_out"`
	Shards   Shards `json:"_shards"`
	Hits     Hits   `json:"hits"`
}

type Shards struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

type Hits struct {
	Total    int            `json:"total"`
	MaxScore float32        `json:"max_score"`
	DataHits []HitedObjects `json:"hits"`
}

type HitedObjects struct {
	Index  string  `json:"_index"`
	Type   string  `json:"_type"`
	Id     string  `json:"_id"`
	Score  float32 `json:"_score"`
	Source Source  `json:"_source"`
}

type Source struct {
	Kubernetes       KubernetesObjects  `json:"kubernetes,omitempty"`
	Systemd          Systemd            `json:"systemd,omitempty"`
	ViaqMsgID        string             `json:"viaq_msg_id"`
	Level            string             `json:"level"`
	Message          string             `json:"message"`
	Docker           DockerObjects      `json:"docker,omitempty"`
	HostName         string             `json:"hostname"`
	TimeStamp        string             `json:"@timestamp"`
	PipelineMetadata PipelineMetadata   `json:"pipeline_metadata"`
	Structured       JsonStructuredLogs `json:"structured,omitempty"`
}

type KubernetesObjects struct {
	ContainerImageID string          `json:"container_image_id"`
	ContainerName    string          `json:"container_name"`
	NamespaceID      string          `json:"namespace_id"`
	FlatLabels       []string        `json:"flat_labels"`
	Host             string          `json:"host"`
	MasterURL        string          `json:"master_url"`
	PodID            string          `json:"pod_id"`
	NamespaceLabels  NamespaceLabels `json:"namespace_labels,omitempty"`
	ContainerImage   string          `json:"container_image"`
	NamespaceName    string          `json:"namespace_name"`
	PodName          string          `json:"pod_name"`
}

type NamespaceLabels struct {
	KubernetesIOMetadataName     string `json:"kubernetes_io/metadata_name,omitempty"`
	OpenshiftIOClusterMonitoring string `json:"openshift_io/cluster-monitoring,omitempty"`
}

type Systemd struct {
	SystemdT SystemdT `json:"t"`
	SystemdU SystemdU `json:"u"`
}

type SystemdT struct {
	SystemdInvocationID string `json:"SYSTEMD_INVOCATION_ID"`
	BootID              string `json:"BOOT_ID"`
	GID                 string `json:"GID"`
	CmdLine             string `json:"CMDLINE"`
	PID                 string `json:"PID"`
	SystemSlice         string `json:"SYSTEMD_SLICE"`
	SelinuxContext      string `json:"SELINUX_CONTEXT"`
	UID                 string `json:"UID"`
	StreamID            string `json:"STREAM_ID"`
	Transport           string `json:"TRANSPORT"`
	Comm                string `json:"COMM"`
	EXE                 string
	SystemdUnit         string `json:"SYSTEMD_UNIT"`
	CapEffective        string `json:"CAP_EFFECTIVE"`
	MachineID           string `json:"MACHINE_ID"`
	SystemdCgroup       string `json:"SYSTEMD_CGROUP"`
}

type SystemdU struct {
	SyslogIdntifier string `json:"SYSLOG_IDENTIFIER"`
	SyslogFacility  string `json:"SYSLOG_FACILITY"`
}
type DockerObjects struct {
	ContainerID string `json:"container_id"`
}

type PipelineMetadata struct {
	Collector Collector `json:"collector"`
}

type Collector struct {
	ReceivedAt string `json:"received_at"`
	Name       string `json:"name"`
	InputName  string `json:"inputname"`
	Version    string `json:"version"`
	IPaddr4    string `json:"ipaddr4"`
}

type JsonStructuredLogs struct {
	Level        string                   `json:"level,omitempty"`
	StringNumber string                   `json:"StringNumber,omitempty"`
	Message      string                   `json:"message,omitempty"`
	Number       int                      `json:"Number,omitempty"`
	Layer1       string                   `json:"Layer1,omitempty"`
	FooColonBar  string                   `json:"foo:bar,omitempty"`
	FooDotBar    string                   `json:"foo.bar,omitempty"`
	BraceItem    string                   `json:"{foobar},omitempty"`
	BracketItem  string                   `json:"[foobar],omitempty"`
	Layer2       JsonStructuredLogsLayer2 `json:"layer2,omitempty"`
}

type JsonStructuredLogsLayer2 struct {
	Name string `json:"name,omitempty"`
	Tips string `json:"tips,omitempty"`
}

/*
{
  "count" : 453558,
  "_shards" : {
    "total" : 39,
    "successful" : 39,
    "skipped" : 0,
    "failed" : 0
  }
}
*/
type CountResult struct {
	Count  int    `json:"count"`
	Shards Shards `json:"_shards"`
}

/*
  {
    "health": "green",
    "status": "open",
    "index": "infra-000015",
    "uuid": "uHqlf91RQAqit072gI9LaA",
    "pri": "3",
    "rep": "1",
    "docs.count": "37323",
    "docs.deleted": "0",
    "store.size": "58.8mb",
    "pri.store.size": "29.3mb"
  }
*/
type ESIndex struct {
	Health       string `json:"health"`
	Status       string `json:"status"`
	Index        string `json:"index"`
	UUID         string `json:"uuid"`
	PrimaryCount string `json:"pri"`
	ReplicaCount string `json:"rep"`
	DocsCount    string `json:"docs.count"`
	DocsDeleted  string `json:"docs.deleted"`
	StoreSize    string `json:"store.size"`
	PriStoreSize string `json:"pri.store.size"`
}

// packagemanifest
type PackageManifest struct {
	Status struct {
		CatalogSource          string `json:"catalogSource"`
		CatalogSourceNamespace string `json:"catalogSourceNamespace"`
		Channels               []struct {
			CurrentCSV string `json:"currentCSV"`
			Name       string `json:"name"`
		} `json:"channels"`
		DefaultChannel string `json:"defaultChannel"`
	} `json:"status"`
}

type OperatorHub struct {
	Status struct {
		Sources []struct {
			Disabled bool   `json:"disabled"`
			Name     string `json:"name"`
			Status   string `json:"status"`
		} `json:"sources"`
	} `json:"status"`
}

// elasticsearch/elasticsearch
type Elasticsearch struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   `json:"metadata"`
	Spec       struct {
		IndexManagement struct {
			Mappings []struct {
				Aliases   []string `json:"aliases"`
				Name      string   `json:"name"`
				PolicyRef string   `json:"policyRef"`
			} `json:"mappings"`
			Policies []struct {
				Name   string `json:"name"`
				Phases struct {
					Delete struct {
						MinAge string `json:"minAge"`
					} `json:"delete"`
					Hot struct {
						Actions struct {
							Rollover struct {
								MaxAge string `json:"maxAge"`
							} `json:"rollover"`
						} `json:"actions"`
					} `json:"hot"`
				} `json:"phases"`
				PollInterval string `json:"pollInterval"`
			} `json:"policies"`
		} `json:"indexManagement"`
		ManagementState string `json:"managementState"`
		NodeSpec        struct {
			ProxyResources ResourcesSpec `json:"proxyResources,omitempty"`
			Resources      ResourcesSpec `json:"resources,omitempty"`
		} `json:"nodeSpec"`
		Nodes            []ESNode `json:"nodes"`
		RedundancyPolicy string   `json:"redundancyPolicy"`
	} `json:"spec"`
	Status struct {
		Cluster    ElasticsearchClusterHealth `json:"cluster"`
		Conditions []Conditions               `json:"conditions"`
		Nodes      []struct {
			DeploymentName string `json:"deploymentName"`
			UpgradeStatus  struct {
				ScheduledCertRedeploy string `json:"scheduledCertRedeploy,omitempty"`
			} `json:"upgradeStatus,omitempty"`
			StatefulSetName string `json:"statefulSetName,omitempty"`
		} `json:"nodes"`
		Pods struct {
			Client PodsStatus `json:"client"`
			Data   PodsStatus `json:"data"`
			Master PodsStatus `json:"master"`
		} `json:"pods"`
		ShardAllocationEnabled string `json:"shardAllocationEnabled"`
	} `json:"status"`
}

type Metadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type ElasticsearchClusterHealth struct {
	ActivePrimaryShards int32  `json:"activePrimaryShards"`
	ActiveShards        int32  `json:"activeShards"`
	InitializingShards  int32  `json:"initializingShards"`
	NumDataNodes        int32  `json:"numDataNodes"`
	NumNodes            int32  `json:"numNodes"`
	PendingTasks        int32  `json:"pendingTasks"`
	RelocatingShards    int32  `json:"relocatingShards"`
	Status              string `json:"status"`
	UnassignedShards    int32  `json:"unassignedShards"`
}
type ESNode struct {
	GenUUID        string        `json:"genUUID"`
	NodeCount      int32         `json:"nodeCount"`
	ProxyResources ResourcesSpec `json:"proxyResources,omitempty"`
	Resources      ResourcesSpec `json:"resources,omitempty"`
	Roles          []string      `json:"roles"`
	Storage        StorageSpec   `json:"storage,omitempty"`
}

type Conditions struct {
	LastTransitionTime string `json:"lastTransitionTime"`
	Status             string `json:"status"`
	Type               string `json:"type"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
}

type StorageSpec struct {
	Size             string `json:"size"`
	StorageClassName string `json:"storageClassName"`
}

type PodsStatus struct {
	Failed   []string `json:"failed,omitempty"`
	NotReady []string `json:"notReady,omitempty"`
	Ready    []string `json:"ready,omitempty"`
}

type ResourcesSpec struct {
	Limits   ResourceList `json:"limits,omitempty"`
	Requests ResourceList `json:"requests,omitempty"`
}

type ResourceList struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}

// clusterlogging/instance
type ClusterLogging struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   `json:"metadata"`
	Spec       ClusterLoggingSpec   `json:"spec"`
	Status     ClusterLoggingStatus `json:"status,omitempty"`
}

type ClusterLoggingSpec struct {
	CollectionSpec    `json:"collection,omitempty"`
	LogStoreSpec      `json:"logStore,omitempty"`
	ManagementState   string `json:"managementState"`
	VisualizationSpec `json:"visualization,omitempty"`
}

type CollectionSpec struct {
	Logs LogCollectionSpec `json:"logs"`
}

type LogCollectionSpec struct {
	Type        string `json:"type"`
	FluentdSpec `json:"fluentd"`
}

type FluentdSpec struct {
	Resources    ResourcesSpec     `json:"resources"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Tolerations  []v1.Toleration   `json:"tolerations,omitempty"`
}

type LogStoreSpec struct {
	Type              string `json:"type"`
	ElasticsearchSpec `json:"elasticsearch,omitempty"`
	RetentionPolicy   `json:"retentionPolicy,omitempty"`
}

type ElasticsearchSpec struct {
	Resources        ResourcesSpec     `json:"resources"`
	NodeCount        int32             `json:"nodeCount"`
	NodeSelector     map[string]string `json:"nodeSelector,omitempty"`
	Tolerations      []v1.Toleration   `json:"tolerations,omitempty"`
	Storage          StorageSpec       `json:"storage"`
	RedundancyPolicy string            `json:"redundancyPolicy"`
	ProxySpec        struct {
		Resources ResourcesSpec `json:"resources"`
	} `json:"proxy,omitempty"`
}

type RetentionPolicy struct {
	App   *RetentionPolicySpec `json:"application,omitempty"`
	Infra *RetentionPolicySpec `json:"infra,omitempty"`
	Audit *RetentionPolicySpec `json:"audit,omitempty"`
}

type RetentionPolicySpec struct {
	MaxAge                  string           `json:"maxAge"`
	PruneNamespacesInterval string           `json:"pruneNamespacesInterval,omitempty"`
	Namespaces              []PruneNamespace `json:"namespaceSpec,omitempty"`
}

type PruneNamespace struct {
	Namespace string `json:"namespace"`
	MinAge    string `json:"minAge,omitempty"`
}

type VisualizationSpec struct {
	Type       string     `json:"type"`
	KibanaSpec KibanaSpec `json:"kibana,omitempty"`
}

type KibanaSpec struct {
	Resources    ResourcesSpec     `json:"resources"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Tolerations  []v1.Toleration   `json:"tolerations,omitempty"`
	Replicas     int32             `json:"replicas"`
	ProxySpec    struct {
		Resources ResourcesSpec `json:"resources"`
	} `json:"proxy,omitempty"`
}

type ClusterLoggingStatus struct {
	ClusterConditions []Conditions `json:"clusterConditons,omitempty"`

	Collection struct {
		Logs struct {
			FluentdStatus struct {
				DaemonSet string            `json:"daemonSet"`
				Nodes     map[string]string `json:"nodes"`
				Pods      PodsStatus        `json:"pods"`
			} `json:"fluentdStatus"`
		} `json:"logs"`
	} `json:"collection"`

	Visualization struct {
		KibanaStatus []struct {
			Deployment  string     `json:"deployment"`
			Pods        PodsStatus `json:"pods"`
			ReplicaSets []string   `json:"replicaSets"`
			Replicas    *int32     `json:"replicas"`
		} `json:"kibanaStatus"`
	} `json:"visualization"`

	LogStore struct {
		ElasticsearchStatus []struct {
			ClusterName   string                     `json:"clusterName"`
			NodeCount     int32                      `json:"nodeCount"`
			ReplicaSets   []string                   `json:"replicaSets,omitempty"`
			Deployments   []string                   `json:"deployments,omitempty"`
			StatefulSets  []string                   `json:"statefulSets,omitempty"`
			ClusterHealth string                     `json:"clusterHealth,omitempty"`
			Cluster       ElasticsearchClusterHealth `json:"cluster"`
			Pods          struct {
				Client PodsStatus `json:"client"`
				Data   PodsStatus `json:"data"`
				Master PodsStatus `json:"master"`
			} `json:"pods"`
			ShardAllocationEnabled string                `json:"shardAllocationEnabled"`
			ClusterConditions      []Conditions          `json:"clusterConditions,omitempty"`
			NodeConditions         map[string]Conditions `json:"nodeConditions,omitempty"`
		} `json:"elasticsearchStatus"`
	} `json:"logStore"`
}

//Loki Logs (Audit, Infra and App)
/*
{
	"status": "success",
	"data": {
		"resultType": "streams",
		"result": [{
			"stream": {
				"kubernetes_container_name": "centos-logtest",
				"kubernetes_host": "ip-10-0-161-168.us-east-2.compute.internal",
				"kubernetes_namespace_name": "test",
				"kubernetes_pod_name": "centos-logtest-qt6pz",
				"log_type": "application",
				"tag": "kubernetes.var.log.containers.centos-logtest-qt6pz_test_centos-logtest-da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617.log",
				"fluentd_thread": "flush_thread_0"
			},
			"values": [
				["1637005525936482085", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:26.753126+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:25.936482+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"NmM5YWIyOGMtM2M4MS00MTFkLWJjNjEtZGIxZDE4MWViNzk0\",\"log_type\":\"application\"}"],
				["1637005524935237626", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:25.753025+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:24.935237+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"NWViNDMzYWMtMmFhZC00MDIwLTliNTgtMjcxNGIzMjE1Y2Mz\",\"log_type\":\"application\"}"],
				["1637005521931879895", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:22.753512+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:21.931879+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"NzE4NzMyOTctMjJkMy00YTU5LTljOTEtMWNjODIzYTI4ZmU0\",\"log_type\":\"application\"}"],
				["1637005520930877672", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:21.753673+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:20.930877+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"ZjFhMjJlYTctMzAzMy00YzA1LTk5M2EtZjQ0MzU3NzU5OTRj\",\"log_type\":\"application\"}"],
				["1637005518928817746", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:19.753113+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:18.928817+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"ZTBiNzlhOWYtYThlMy00N2VmLWFkZDMtZDcxZDQxNTkzMzU3\",\"log_type\":\"application\"}"],
				["1637005516927007443", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:17.753765+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:16.927007+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"MzNiNGYzOGEtNmE1Mi00ZTNlLTg3OWEtZTcxOTk2MDQyZjJh\",\"log_type\":\"application\"}"],
				["1637005514925104671", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:15.753362+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:14.925104+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"YTVjOTMwY2QtYjRjZS00Nzg4LWIzMTctY2RhZDU0MjE1NzE1\",\"log_type\":\"application\"}"],
				["1637005512923008944", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:13.753713+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:12.923008+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"ZWM1YTNlZjEtMDBkNi00NTZlLWE4MDQtZWZhMmQ5NGNhNDBj\",\"log_type\":\"application\"}"],
				["1637005510920511706", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:11.752977+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:10.920511+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"NTAxODg4NDctYWEzZC00YWE3LWJkNjctZGYzNWMyMGVlMjlk\",\"log_type\":\"application\"}"],
				["1637005508917899152", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:09.753156+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:08.917899+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"ZGJkOTBiNWMtNjMzZi00NjEzLWIwNjQtYTkxNDkxMDQ3ZTc2\",\"log_type\":\"application\"}"],
				["1637005506915819098", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:07.752994+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:06.915819+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"Yjk0ZDEwMGYtMDEwOS00ZTQ4LTlkZDYtZjU2YjFkNTM1ODUz\",\"log_type\":\"application\"}"],
				["1637005505914840599", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:06.753093+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:05.914840+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"ZmE3NWY3NjItMzQwMS00ZjQxLWJlMDUtNmYwOTE0YzU4ZGUy\",\"log_type\":\"application\"}"],
				["1637005504913701662", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:05.752892+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:04.913701+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"MTkyNjNkMzItNmJkNC00YzlkLWJjMTItZmRiOWQwNGMwYjEz\",\"log_type\":\"application\"}"],
				["1637005502910706363", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:03.753368+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:02.910706+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"MGY4Zjc1MDItMTY2NC00MzcyLWJhZjAtNjlmMjAwZjFhNGE5\",\"log_type\":\"application\"}"],
				["1637005499905812370", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:00.754884+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:44:59.905812+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"M2IzYzQ0YjUtYzI0My00YzMxLWJkY2EtYzAyOTA2ZGFkM2M0\",\"log_type\":\"application\"}"]
			]
		}, {
			"stream": {
				"kubernetes_host": "ip-10-0-161-168.us-east-2.compute.internal",
				"kubernetes_namespace_name": "test",
				"kubernetes_pod_name": "centos-logtest-qt6pz",
				"log_type": "application",
				"tag": "kubernetes.var.log.containers.centos-logtest-qt6pz_test_centos-logtest-da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617.log",
				"fluentd_thread": "flush_thread_1",
				"kubernetes_container_name": "centos-logtest"
			},
			"values": [
				["1637005526937766424", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:27.753129+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:26.937766+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"NzM3MDc0YTktZTk2Ny00MjVkLWIyZjktNTEyYWUzMzc5NGI3\",\"log_type\":\"application\"}"],
				["1637005523933992437", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:24.753189+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:23.933992+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"MDI4ODNjODgtMDY1Mi00YjFhLWJmM2ItZmE5ZTNjNzZiMjNj\",\"log_type\":\"application\"}"],
				["1637005522932747640", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:23.753267+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:22.932747+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"ZjExNDc5ZDAtNGYxZS00ZWRkLWE4YTAtNjYxMjQyNjY3ZDdh\",\"log_type\":\"application\"}"],
				["1637005519930054570", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:20.752974+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:19.930054+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"YzVjODk0YzUtYWQ0ZS00M2IxLTg4ZTUtMjhiNDE0YjJjMzZk\",\"log_type\":\"application\"}"],
				["1637005517927831354", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:18.753285+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:17.927831+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"YTJhM2QwZDEtNzc2NC00Mzg0LWFmNDMtMjkwZjcwNTM3MWQx\",\"log_type\":\"application\"}"],
				["1637005515925824796", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:16.752947+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:15.925824+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"NzllYzc0MGUtZGIyMy00NDk3LTlmNjUtYTlmNWE2ZDUyZDQz\",\"log_type\":\"application\"}"],
				["1637005513923842479", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:14.753474+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:13.923842+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"N2Y1Mzg4ZDEtMWNmZi00YjZmLTlhNWQtYTllMGIzODY5YzY0\",\"log_type\":\"application\"}"],
				["1637005511921767657", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:12.754080+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:11.921767+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"MGMwZWI2NjAtZDM4MS00NTJlLTg3NzctMzFiN2NhYjhjZGI0\",\"log_type\":\"application\"}"],
				["1637005509919265834", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:10.752996+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:09.919265+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"Y2QxMjRjZTctNmFjOC00ZjQ2LTg5MGYtYjkxZGQxZjMxYWQy\",\"log_type\":\"application\"}"],
				["1637005507917073864", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:08.753313+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:07.917073+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"NzI1OTRlOTgtNzlhZi00ZGQ2LWI4NjktZWY1NGUwMjkyODA3\",\"log_type\":\"application\"}"],
				["1637005503912198012", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:04.753195+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:03.912198+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"ZjAxMDYwOTAtMWMyYi00YzU4LTk3ZjEtOGVkZjJlYjkwNjZh\",\"log_type\":\"application\"}"],
				["1637005501909387338", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:02.753040+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:01.909387+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"OWVjZWYzOGEtYWNjNi00OTA0LWIyODctODMyYzA4MTg0Yzky\",\"log_type\":\"application\"}"],
				["1637005500907904677", "{\"docker\":{\"container_id\":\"da3cf8c0493625dc4f42c8592954aad95f3f4ce2a2098ab97ab6a4ad58276617\"},\"kubernetes\":{\"container_name\":\"centos-logtest\",\"namespace_name\":\"test\",\"pod_name\":\"centos-logtest-qt6pz\",\"container_image\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"container_image_id\":\"quay.io/openshifttest/ocp-logtest@sha256:f23bea6f669d125f2f317e3097a0a4da48e8792746db32838725b45efa6c64a4\",\"pod_id\":\"d77cae4f-2b8a-4c30-8142-417756aa3daf\",\"pod_ip\":\"10.129.2.66\",\"host\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"labels\":{\"run\":\"centos-logtest\",\"test\":\"centos-logtest\"},\"master_url\":\"https://kubernetes.default.svc\",\"namespace_id\":\"18a06953-fdca-4760-b265-a4ef9b98128e\",\"namespace_labels\":{\"kubernetes_io/metadata_name\":\"test\"}},\"message\":\"{\\\"message\\\": \\\"MERGE_JSON_LOG=true\\\", \\\"level\\\": \\\"debug\\\",\\\"Layer1\\\": \\\"layer1 0\\\", \\\"layer2\\\": {\\\"name\\\":\\\"Layer2 1\\\", \\\"tips\\\":\\\"Decide by PRESERVE_JSON_LOG\\\"}, \\\"StringNumber\\\":\\\"10\\\", \\\"Number\\\": 10,\\\"foo.bar\\\":\\\"Dot Item\\\",\\\"{foobar}\\\":\\\"Brace Item\\\",\\\"[foobar]\\\":\\\"Bracket Item\\\", \\\"foo:bar\\\":\\\"Colon Item\\\",\\\"foo bar\\\":\\\"Space Item\\\" }\",\"level\":\"unknown\",\"hostname\":\"ip-10-0-161-168.us-east-2.compute.internal\",\"pipeline_metadata\":{\"collector\":{\"ipaddr4\":\"10.0.161.168\",\"inputname\":\"fluent-plugin-systemd\",\"name\":\"fluentd\",\"received_at\":\"2021-11-15T19:45:01.753261+00:00\",\"version\":\"1.7.4 1.6.0\"}},\"@timestamp\":\"2021-11-15T19:45:00.907904+00:00\",\"viaq_index_name\":\"app-write\",\"viaq_msg_id\":\"Yzc1MmJkZDQtNzk4NS00NzA5LWFkN2ItNTlmZmE3NTgxZmUy\",\"log_type\":\"application\"}"]
			]
		}],
		"stats": {
			"summary": {
				"bytesProcessedPerSecond": 48439028,
				"linesProcessedPerSecond": 39619,
				"totalBytesProcessed": 306872,
				"totalLinesProcessed": 251,
				"execTime": 0.006335222
			},
			"store": {
				"totalChunksRef": 0,
				"totalChunksDownloaded": 0,
				"chunksDownloadTime": 0,
				"headChunkBytes": 0,
				"headChunkLines": 0,
				"decompressedBytes": 0,
				"decompressedLines": 0,
				"compressedBytes": 0,
				"totalDuplicates": 0
			},
			"ingester": {
				"totalReached": 1,
				"totalChunksMatched": 2,
				"totalBatches": 1,
				"totalLinesSent": 28,
				"headChunkBytes": 41106,
				"headChunkLines": 85,
				"decompressedBytes": 265766,
				"decompressedLines": 166,
				"compressedBytes": 13457,
				"totalDuplicates": 0
			}
		}
	}
}
*/

type LokiLogQuery struct {
	Lokistatus string `json:"status"`
	Data       Data   `json:"data"`
}

type Data struct {
	ResultType string   `json:"resultType"`
	Result     []Result `json:"result"`
	Stats      Stats    `json:"stats"`
}

type Result struct {
	Stream Stream   `json:"stream"`
	Values []string `json:"values"`
}

type Stream struct {
	LogType                 string `json:"log_type"`
	Tag                     string `json:"tag"`
	FluentdThread           string `json:"fluentd_thread"`
	KubernetesContainerName string `json:"kubernetes_container_name"`
	KubernetesHost          string `json:"kubernetes_host"`
	KubernetesNamespaceName string `json:"kubernetes_namespace_name"`
	KubernetesPodName       string `json:"kubernetes_pod_name"`
}

type Stats struct {
	Summary  SummaryObject  `json:"summary"`
	Store    StoreObject    `json:"store"`
	Ingester IngesterObject `json:"ingester"`
}

type SummaryObject struct {
	BytesProcessedPerSecond int     `json:"bytesProcessedPerSecond"`
	LinesProcessedPerSecond int     `json:"linesProcessedPerSecond"`
	TotalBytesProcessed     int     `json:"totalBytesProcessed"`
	TotalLinesProcessed     int     `json:"totalLinesProcessed"`
	ExecTime                float32 `json:"execTime"`
}
type StoreObject struct {
	TotalChunksRef        int `json:"totalChunksRef"`
	TotalChunksDownloaded int `json:"totalChunksDownloaded"`
	ChunksDownloadTime    int `json:"chunksDownloadTime"`
	HeadChunkBytes        int `json:"headChunkBytes"`
	HeadChunkLines        int `json:"headChunkLines"`
	DecompressedBytes     int `json:"decompressedBytes"`
	DecompressedLines     int `json:"decompressedLines"`
	CompressedBytes       int `json:"compressedBytes"`
	TotalDuplicates       int `json:"totalDuplicates"`
}

type IngesterObject struct {
	TotalReached       int `json:"totalReached"`
	TotalChunksMatched int `json:"totalChunksMatched"`
	TotalBatches       int `json:"totalBatches"`
	TotalLinesSent     int `json:"totalLinesSent"`
	HeadChunkBytes     int `json:"headChunkBytes"`
	HeadChunkLines     int `json:"headChunkLines"`
	DecompressedBytes  int `json:"decompressedBytes"`
	DecompressedLines  int `json:"decompressedLines"`
	CompressedBytes    int `json:"compressedBytes"`
	TotalDuplicates    int `json:"totalDuplicates"`
}

//Logs sent to Loki
/*
 {
	"status": "success",
	"data": ["__name__", "fluentd_thread", "kubernetes_container_name", "kubernetes_host", "kubernetes_namespace_name", "kubernetes_pod_name", "log_type", "tag"]
}
*/

type LokiSearch struct {
	SearchStatus string   `json:"status"`
	Data         []string `json:"data"`
}
