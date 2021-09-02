###### The purpose of this tool is to collect automated results as a substitute in case of reportPortal problems such as 504 timeout.

#### Method of use：

```shell
$ sudo python collect_result_to_googlesheet.py -c ./config_olm.yaml -j ginkgo-test -n 100

-c	The path of the configuration file

-j	Jobname such as ginkgo-test/ginkgo-test-vm

-n	The number of builds which will be collected result from. If this parameter is not specified, the default execution continues until the latest.
```



##### Config yaml

```yaml
end_num:	##the last end number and it's begin number this time

google:
  key_file:	##the key for write into google sheet api
                ##reference：https://gitlab.cee.redhat.com/aosqe/openshift-misc/-/blob/master/jenkins/build_corp/openshift-qe-buildcorp.json
  sheet_file: ##which google sheet file do you want to write
```

###### If you still have some other questions, you can contact me use email.

jitli@redhat.com

##### Thanks.
