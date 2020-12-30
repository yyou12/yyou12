# Extended Platform Tests

This repository holds the non-kubernetes, end-to-end tests that need to pass on a running
cluster before PRs merge and/or before we ship a release.
These tests are based on ginkgo and the github.com/kubernetes/kubernetes e2e test framework.

Prerequisites
-------------

* Git installed.
* Golang installed.
* Have the environment variable `KUBECONFIG` set pointing to your cluster.

### Update The Public Repo Lib
We treat the public repo: [openshift-tests](https://github.com/openshift/openshift-tests) as a dependency lib. That means you can also run the test case of that public repo in this private repo. Run the `$ make update-public` command to update this dep lib. Or you can build the binary with `$ make all` command.

### New Test Folder
If you create a new folder for your test case, please **add the path** to the [include.go file](https://github.com/openshift/openshift-tests-private/blob/master/test/extended/include.go).

## Compile the executable binary
The generated `extended-platform-tests` binary in the `./bin/extended-platform-tests/` folder.
If you want to compile the `openshift-tests` binary, please see the [origin](https://github.com/openshift/origin).

```console
$ mkdir -p ${GOPATH}/src/github.com/openshift/
$ cd ${GOPATH}/src/github.com/openshift/
$ git clone git@github.com:openshift/openshift-tests-private.git
$ cd openshift-tests-private/
$ make clean
$ make build
```

## How to Contribute 
Below is an example of how to submit a PR. First, you should **Fork** this repo to yourself Github repo.
Note that: please use `make build` instead of the `make all`/`make update-public` command in your development. 

```console
$ git remote add <user> git@github.com:<user>/openshift-tests-private.git
$ git pull origin master
$ git checkout -b example
$ git add xxx
$ make build
$ ./bin/extended-platform-tests xxx
$ git commit -m "xxx"
$ git push <user> example:example
```

Run `./bin/extended-platform-tests --help` to get started.

```console
This command verifies behavior of an OpenShift cluster by running remote tests against the cluster API that exercise functionality. In general these tests may be disruptive or require elevated privileges - see the descriptions of each test suite.

Usage:
   [command]

Available Commands:
  help        Help about any command
  run         Run a test suite
  run-monitor Continuously verify the cluster is functional
  run-test    Run a single test by name
  run-upgrade Run an upgrade suite

Flags:
  -h, --help   help for this command
```

## How to run

You can filter your test case by using `grep`. Such as, 
For example, to filter the [OLM test cases](https://github.com/openshift/openshift-tests/blob/master/test/extended/operators/olm.go#L21), you can run this command: 

```console
$ ./bin/extended-platform-tests run all --dry-run|grep "\[Feature:Platform\] OLM should"
I0410 15:33:38.465141    7508 test_context.go:419] Tolerating taints "node-role.kubernetes.io/master" when considering if nodes are ready
"[Feature:Platform] OLM should Implement packages API server and list packagemanifest info with namespace not NULL [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should [Serial] olm version should contain the source commit id [Suite:openshift/conformance/serial]"
"[Feature:Platform] OLM should be installed with catalogsources at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with clusterserviceversions at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with installplans at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with operatorgroups at version v1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with packagemanifests at version v1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with subscriptions at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should have imagePullPolicy:IfNotPresent on thier deployments [Suite:openshift/conformance/parallel]"
```

You can save the above output to a file and run it:

```console
$ ./bin/extended-platform-tests run -f <your file path/name>
```

Or you can run it directly:

```console
$ ./bin/extended-platform-tests run all --dry-run | grep "\[Feature:Platform\] OLM should" | ./bin/extended-platform-tests run --junit-dir=./ -f -
```

### How to run a specific test case
It searches the test case title by RE(`Regular Expression`). So you need to specify the title string detailly.
For example, to run this test case: ["[Serial] olm version should contain the source commit id"](https://github.com/openshift/openshift-tests/blob/master/test/extended/operators/olm.go#L117), you can do it with 2 ways:

* You may filter the list and pass it back to the run command with the --file argument. You may also pipe a list of test names, one per line, on standard input by passing "-f -".

```console
$ ./bin/extended-platform-tests run all --dry-run|grep "\[Serial\] olm version should contain the source commit id"|./bin/extended-platform-tests run --junit-dir=./ -f -
```

* You can also run it as follows if you know which test suite it belongs to.

```console
$ ./bin/extended-platform-tests run openshift/conformance/serial --run "\[Serial\] olm version should contain the source commit id"
```

## Debug
Sometime, we want to **keep the generated namespace for debugging**. Just add the Env Var: `export DELETE_NAMESPACE=false`. These random namespaces will be keep, like below:
```console
...
Dec 18 09:39:33.448: INFO: Running AfterSuite actions on all nodes
Dec 18 09:39:33.448: INFO: Waiting up to 7m0s for all (but 100) nodes to be ready
Dec 18 09:39:33.511: INFO: Found DeleteNamespace=false, skipping namespace deletion!
Dec 18 09:39:33.511: INFO: Running AfterSuite actions on node 1
...
1 pass, 0 skip (2m50s)
[root@preserve-olm-env openshift-tests-private]# oc get ns
NAME                                               STATUS   AGE
default                                            Active   4h46m
e2e-test-olm-a-a92jyymd-lmgj6                      Active   4m28s
e2e-test-olm-a-a92jyymd-pr8hx                      Active   4m29s
...
```

## How to generate bindata
If you have some new YAML files used in your code, you have to generate the bindata first.
Run `make update` to update the bindata. For example, you can see the bindata has been updated after running the `make update`. As follows: 
```console
$ git status
	modified:   test/extended/testdata/bindata.go
	new file:   test/extended/testdata/olm/etcd-subscription-manual.yaml
```

## Running on GCE
You will get the below error when running the test cases on GCP platform. 
```
E0628 22:11:41.236497   25735 test_context.go:447] Failed to setup provider config for "gce": Error building GCE/GKE provider: google: could not find default credentials. See https://developers.google.com/accounts/docs/application-default-credentials for more information.
```
**You need to `export` the below environment variable before running test on GCP.**
```
$ export GOOGLE_APPLICATION_CREDENTIALS=`pwd`/secrets/gce/aos-qe-sa.json
```

### Update the GCE SA
You may get `400 Bad Request` error even if you have `export` the above values. This error means it's time to update the SA.
```
E0628 22:18:22.290137   26212 gce.go:876] error fetching initial token: oauth2: cannot fetch token: 400 Bad Request
Response: {"error":"invalid_grant","error_description":"Invalid JWT Signature."}
```
You can update the SA by following this [authentication](https://cloud.google.com/docs/authentication/production#cloud-console). As follows, or you can raise an issue here.
1. Click the [apis](https://console.cloud.google.com/apis/credentials/serviceaccountkey?_ga=2.126026830.216162210.1593398139-2070485991.1569310149&project=openshift-qe&folder&organizationId=54643501348)
2. From the `Service account` list, select New service account.
3. In the `Service account` name field, enter a name.
4. Click `Create`. A JSON file that contains your key downloads to your computer.


## Run Certified Operators test

```console
$ ./bin/extended-platform-tests run openshift/isv --dry-run | grep -E "<REGEX>" | ./bin/extended-platform-tests run -f -
```
