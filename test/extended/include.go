package extended

import (
	_ "k8s.io/kubernetes/test/e2e"

	// test sources
	_ "k8s.io/kubernetes/test/e2e/apimachinery"
	_ "k8s.io/kubernetes/test/e2e/apps"
	_ "k8s.io/kubernetes/test/e2e/auth"
	_ "k8s.io/kubernetes/test/e2e/autoscaling"
	_ "k8s.io/kubernetes/test/e2e/common"
	_ "k8s.io/kubernetes/test/e2e/instrumentation"
	_ "k8s.io/kubernetes/test/e2e/kubectl"

	_ "k8s.io/kubernetes/test/e2e/network"
	_ "k8s.io/kubernetes/test/e2e/node"
	_ "k8s.io/kubernetes/test/e2e/scheduling"
	_ "k8s.io/kubernetes/test/e2e/servicecatalog"
	_ "k8s.io/kubernetes/test/e2e/storage"

	_ "github.com/openshift/openshift-tests-private/test/extended/apiserver_and_auth"
	_ "github.com/openshift/openshift-tests-private/test/extended/cluster_operator/cloudcredential"
	_ "github.com/openshift/openshift-tests-private/test/extended/etcd"
	_ "github.com/openshift/openshift-tests-private/test/extended/image_registry"
	_ "github.com/openshift/openshift-tests-private/test/extended/networking"
	_ "github.com/openshift/openshift-tests-private/test/extended/node"
	_ "github.com/openshift/openshift-tests-private/test/extended/operators"
	_ "github.com/openshift/openshift-tests-private/test/extended/operatorsdk"
	_ "github.com/openshift/openshift-tests-private/test/extended/opm"
	_ "github.com/openshift/openshift-tests-private/test/extended/router"
	_ "github.com/openshift/openshift-tests-private/test/extended/securityandcompliance"
	_ "github.com/openshift/openshift-tests-private/test/extended/winc"
	_ "github.com/openshift/openshift-tests-private/test/extended/workloads"
	_ "github.com/openshift/openshift-tests-private/test/extended/apiserver_and_auth"
	_ "github.com/openshift/openshift-tests-private/test/extended/router"
	_ "github.com/openshift/openshift-tests-private/test/extended/node"
	_ "github.com/openshift/openshift-tests-private/test/extended/networking"
	_ "github.com/openshift/openshift-tests-private/test/extended/ota/osus"
	_ "github.com/openshift/openshift-tests-private/test/extended/ota/cvo"
	_ "github.com/openshift/openshift-tests/test/extended/operators"
	_ "github.com/openshift/openshift-tests-private/test/extended/clusterinfrastructure"
)
