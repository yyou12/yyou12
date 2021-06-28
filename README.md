# Extended Platform Tests
This repository holds the non-kubernetes, end-to-end tests that need to pass on a running
cluster before PRs merge and/or before we ship a release.
These tests are based on ginkgo and the github.com/kubernetes/kubernetes e2e test framework.

Prerequisites
-------------
* Git installed. See: [Installing Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)
* Golang installed. See: [Installing Golang](https://golang.org/doc/install), the new the better.
* Have the environment variable `KUBECONFIG` set pointing to your cluster.

### Include test cases of the Public Repo
If you want to run the Public [openshift-tests](https://github.com/openshift/openshift-tests) test cases together, you can include the coresponding package [here](https://github.com/openshift/openshift-tests-private/blob/master/test/extended/include.go). For example, include the Public [extended/operators](https://github.com/openshift/openshift-tests-private/blob/master/test/extended/include.go#L28) test cases. And then, run the `$ make update-public` command to get it, or you can run `$ make all` command.

### Include new test folder
If you create a new folder for your test case, please **add the path** to the [include.go](https://github.com/openshift/openshift-tests-private/blob/master/test/extended/include.go).

### Create go-bindata for new YAML files
If you have some **new YAML files** used in your code, you have to generate the bindata first.
Run `make update` to update the bindata. For example, you can see the bindata has been updated after running the `make update`. As follows:
```console
$ git status
	modified:   test/extended/testdata/bindata.go
	new file:   test/extended/testdata/olm/etcd-subscription-manual.yaml
```

### Compile the executable binary
Note that: we use the `go module` for package management, the previous `go path` is deprecated.
```console
$ git clone git@github.com:openshift/openshift-tests-private.git
$ cd openshift-tests-private/
$ make build
mkdir -p "bin"
export GO111MODULE="on" && export GOFLAGS="" && go build -o "bin" "./cmd/extended-platform-tests"
$ ls -hl ./bin/extended-platform-tests 
-rwxrwxr-x. 1 cloud-user cloud-user 165M Jun 24 22:17 ./bin/extended-platform-tests
```

## Contribution 
Below are the general steps about submitting a PR. First, you should **Fork** this repo to yourself Github repo.
```console
$ git remote add <Your Name> git@github.com:<Your Github Account>/openshift-tests-private.git
$ git pull origin master
$ git checkout -b <Branch Name>
$ git add xxx
$ make build
$ ./bin/extended-platform-tests run all --dry-run |grep <Test Case ID>|./bin/extended-platform-tests run -f -
$ git commit -m "xxx"
$ git push <Your Name> <Branch Name>:<Branch Name>
```
And then, there will be have a prompt in your Github repo console, and then click it.

### Run the automation test case
The binary find the test case via searching the test case title. It searches the test case title by RE(`Regular Expression`).So, you can filter your test case by using `grep`. Such as, I want to run all [OLM test cases](https://github.com/openshift/openshift-tests/blob/master/test/extended/operators/olm.go#L21), and all of them containns the `OLM` letter, so I can use the `grep OLM` to filter them, as follows: 
```console
$ ./bin/extended-platform-tests run all --dry-run | grep "OLM" | ./bin/extended-platform-tests run -f -
I0624 22:48:36.599578 2404223 test_context.go:419] Tolerating taints "node-role.kubernetes.io/master" when considering if nodes are ready
"[sig-operators] OLM for an end user handle common object Author:kuiwang-Medium-22259-marketplace operator CR status on a running cluster [Exclusive] [Serial]"
...
```
You can save the above output to a file and run it:
```console
$ ./bin/extended-platform-tests run -f <your file path/name>
```
If you want to run a test case, such as, `g.It("Author:jiazha-Critical-23440-can subscribe to the etcd operator  [Serial]"` Since the `TestCaseID` is uniqueness, you can do:
```console
$ ./bin/extended-platform-tests run all --dry-run|grep "23440"|./bin/extended-platform-tests run --junit-dir=./ -f -
```

### Debugging
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
### Running test cases on GCP
You will get the below error when running the test cases on GCP platform. 
```
E0628 22:11:41.236497   25735 test_context.go:447] Failed to setup provider config for "gce": Error building GCE/GKE provider: google: could not find default credentials. See https://developers.google.com/accounts/docs/application-default-credentials for more information.
```
**You need to `export` the below environment variable before running test on GCP.**
```
$ export GOOGLE_APPLICATION_CREDENTIALS=`pwd`/secrets/gce/aos-qe-sa.json
```
#### Update the GCP SA
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

### Running test cases on Azure
In order to execute case on the cluster built on Azure platform, you have to configure the `AZURE_AUTH_LOCATION` env variable which includes Azure subscriptionId, clientId, and clientSecret etc. You can get the `config/credentials/azure.json` from the private repo: `cucushift-internal`.
Note that: if you cannot get the Azure secret successfully, you can still debug/run your test cases via the **Jenkins job**.
```
export AZURE_AUTH_LOCATION=<path to azure.json>
```

#### Update the Azure secret
- Add your ssh key to https://code.engineering.redhat.com/gerrit/#/settings/ssh-keys
- Clone the repo: ssh://<your-kerberos-id>@code.engineering.redhat.com:22/cucushift-internal, for example,
```console
[root@preserve-olm-env data]# git clone ssh://jiazha@code.engineering.redhat.com:22/cucushift-internal
Cloning into 'cucushift-internal'...
remote: Total 1367 (delta 0), reused 1367 (delta 0)
Receiving objects: 100% (1367/1367), 263.87 KiB | 0 bytes/s, done.
Resolving deltas: 100% (516/516), done.
[root@preserve-olm-env data]# cd cucushift-internal/
[root@preserve-olm-env cucushift-internal]# ls config/credentials/
azure.json  crw                                  gce.json      micro_eng                      openshift-qe-regional_v4.json    ssp
ccx-qe      deprecated.openshift-qe-gce_v4.json  gce-ocf.json  msg-client-aos-automation.pem  openshift-qe-shared-vpc_v4.json  vmc.json
cfme        dockerhub                            gce_v4.json   openshift-qe-gce_v4.json       perf-eng
```

## Jenkins
You can take the [ginkgo-test job](https://mastern-jenkins-csb-openshift-qe.apps.ocp4.prod.psi.redhat.com/job/ocp-common/job/ginkgo-test/) to run your test case with your repo. As follows:

Here are the parameters:  
> - SCENARIO: input your case ID  
> - FLEXY_BUILD: the [Launch Environment Flexy](https://mastern-jenkins-csb-openshift-qe.apps.ocp4.prod.psi.redhat.com/job/Launch%20Environment%20Flexy/) build ID to build the cluster you use  
> - TIERN_REPO_OWNER: your GitHub account  
> - TIERN_REPO_BRANCH: your branch for the debug case code  
> - JENKINS_SLAVE: gocxx, xx is your cluster relase version, for example, goc47 for 4.7 cluster  
> - For other parameters, please take default value.  

Here are the procedures:
1. push the case code into your repo with your branch. For example, [example-branch](https://github.com/exampleaccount/openshift-tests-private/tree/examplebranch)
2. launch build with parameters. For example, push the code of case which ID is `12345` into [example-branch](https://github.com/exampleaccount/openshift-tests-private/tree/examplebranch), and your Flexy job is `6789` with using 4.7 release. After you push code to your repo, you could launch a `ginkgo-test` job, as follows:
> - SCENARIO: 12345  
> - FLEXY_BUILD: 6789  
> - TIERN_REPO_OWNER: exampleaccount  
> - TIERN_REPO_BRANCH: examplebranch  
> - JENKINS_SLAVE: goc47  

