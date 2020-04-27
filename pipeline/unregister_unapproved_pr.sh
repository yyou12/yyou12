cp -f /root/prdb/prdbhandler.py .
cp -f /root/prdb/getprs.sh .
if [ ${ACTION} == "get" ] ; then
  ./getprs.sh
  exit $?
fi

if echo $PRURL | grep -E '^https://github.com/openshift/(origin|openshift-tests)/pull/([0-9]+)$'; then
    echo "the PR URL is valid"
else
    echo "the PR URL is not valid, it is like https://github.com/openshift/origin/pull/<number> or https://github.com/openshift/openshift-tests/pull/<number>"
    exit 1
fi

current=`date "+%Y-%m-%d %H:%M:%S"`  
date_str=`echo ${current} | awk -F " " '{print $1}'`
date_str=`echo ${date_str}|sed -e "s/-//g"`
time_str=`echo ${current} | awk -F " " '{print $2}'`
time_str=`echo ${time_str}|sed -e "s/://g"`
backupprsdbnum=`ls -rt -1 /root/prdb/prs.db-*|wc -l`
if [ $backupprsdbnum -ge 100 ]; then
	rm -fr /root/prdb/prs.db-*
fi
cp /root/prdb/prs.db /root/prdb/prs.db"-"${date_str}"-"${time_str}

if [ ${ACTION} == "delete" ] ; then
  python3 prdbhandler.py -a delete -p ${PRURL}
  exit $?
fi

if [ "${SCENARIO}" == "" ]; then
  echo "SCENARIO should not be empty"
  exit 1
fi

org_name=`echo $PRURL | awk -F "/" '{print $4}'`
repo_name=`echo $PRURL | awk -F "/" '{print $5}'`
pr_num=`echo $PRURL | awk -F "/" '{print $7}'`
prinfo=$(python3 prdbhandler.py -a query -q "https://api.github.com/repos/"${org_name}"/"${repo_name}"/pulls/"${pr_num})
ref=$(echo $prinfo | jq '.ref')
ref=`echo ${ref#*\"}`
ref=`echo ${ref%\"*}`
echo $ref
if [ $ref == "null" ]; then
  echo "the PR $PRURL does not exist."
  exit 1
fi
if [ $ref == "connectionerror" ]; then
  echo "please try it after a while because it could be caused by github access rate limitation"
  exit 1
fi
ssh_url=$(echo $prinfo | jq '.ssh_url')
ssh_url=`echo ${ssh_url#*\"}`
ssh_url=`echo ${ssh_url%\"*}`
echo $ssh_url

python3 prdbhandler.py -a ${ACTION} -p ${PRURL} -r ${ssh_url} -b ${ref} -c "${SCENARIO}"

