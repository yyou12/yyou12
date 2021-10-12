package logging

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
