
## NAME
updateresultrp - update the result of the test itme (cases) on reportportal in CLI.

## How to get it
The script `updateresultrp.py` is located in [openshift-tests-private](https://github.com/openshift/openshift-tests-private).
Please clone the [openshift-tests-private](https://github.com/openshift/openshift-tests-private) and execute the script under
the root of the repo.

```console
$ git clone git@github.com:openshift/openshift-tests-private.git
$ cd openshift-tests-private/
```

## SYNOPSIS
**python3 pipeline/updateresultrp.py**  _action_ [options]

## DESCRIPTION
Currently it supports to change the case from Failed to Passed.

  _action_ use the "-a cls" to change the failed cases to passed.


## OPTIONS

**-s** _subteam_

The sub team name. It must be same to that in your g.Describe when developing cases (please use the values list in the sub-team field of the Polarion (Note: it is case sensitive), and please use the “_” instead of the blank).  
For example, `OLM` or `Operator_SDK` etc.

**-l** _launchname_

The launch name. You could get launch name from the reportportal.  
For example, `20210331-1800` etc.

**-td** _timeduration_

The time duraiton to select the launches. Its format follow the time format of the time of the launch in reportportal (year-month-date hour:minute:second).
You could copy the time of the launch from the reportportal.  
The starTime and endTime is separated by comma.  
For example,  
`2021-03-31 15:44:35,2021-04-05 11:20:15` means to select the launches between `2021-03-31 15:44:35` and `2021-04-05 11:20:15`  
`2021-03-31 15:44:35` means to select the launches between `2021-03-31 15:44:35` and now  

**-ak** _attributekey_

The attribue key to filter the launches. You could get the attribue key of the launche from reportportal.  
If you want to take multipe attribue key, please use comma.  
For example, `version,launchtype` etc.

**-av** _attributevalue_

The attribue value to filter the launches. You could get the attribue value of the launche from reportportal.  
If you want to take multipe attribue value, please use comma.  
For example, `4_8,golang` etc.

**-dt** _defecttype_

The defect type to select the cases. Currently it supports "PB", "AB", "SI", "NI", and "TI".  
It does not support multple defect type at same time in one CLI.  
For example, `NI` etc.

**-at** _author_

The author to select the cases. It is same to that in your g.It.  
For example, `kuiwang` etc.  


## EXAMPLES

To change the failed case of the launch name 20210331-1800 of subteam OLM:
```sh
$ python3 pipeline/updateresultrp.py -a cls -s "OLM" -l "20210331-0517"
```

To change the failed case of the launch between `2021-03-31 15:44:35` and `2021-04-05 11:20:15` of subteam OLM:
```sh
$ python3 pipeline/updateresultrp.py -a cls -s "OLM" -dt "2021-03-31 15:44:35,2021-04-05 11:20:15"
```

To change the failed case of the launch between `2021-03-31 15:44:35` till now of subteam OLM:
```sh
$ python3 pipeline/updateresultrp.py -a cls -s "OLM" -dt "2021-03-31 15:44:35"
```

To change the failed case of the launch of subteam OLM which defect type is NI (not issue) for release 4.8:
```sh
$ python3 pipeline/updateresultrp.py -a cls -s "OLM" -ak "version,launchtype" -av "4_8,golang" -dt "NI"
```

To change the failed case of the launch of subteam OLM which defect type is NI (not issue) for release 4.8 for kuiwang's case:
```sh
$ python3 pipeline/updateresultrp.py -a cls -s "OLM" -ak "version,launchtype" -av "4_8,golang" -dt "NI" -at "kuiwang"
```

To change the failed case of the launch of all sub teams which defect type is NI (not issue) for release 4.8:
```sh
$ python3 pipeline/updateresultrp.py -a cls -ak "version,launchtype" -av "4_8,golang" -dt "NI"
```

