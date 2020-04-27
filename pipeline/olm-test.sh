if [ "${KUBECONFIG_FILE}" == "" ] || [ "${REPO}" == "" ] || [ "${BRANCH}" == "" ] || [ "${SCENARIO}" == "" ] ; then
  echo "please input value for all parameters"
  exit 1
fi

if echo ${REPO} | grep -E '^git@github.com:(.{0,39})/(origin|openshift-tests).git$'; then
    echo "the REPO URL ${REPO} is valid"
else
    echo "the REPO URL ${REPO} is not valid, it looks like git@github.com:<account>/origin.git or git@github.com:<account>/openshift-tests.git"
    exit 1
fi

source ~/.bash_profile
output_du=`du -s -k /data/go-build`
size=`echo ${output_du} | awk -F " " '{print $1}'`
size_threshold=20000000
if [ ${size} -gt ${size_threshold} ]; then
  go clean -cache
fi

ck "${KUBECONFIG_FILE}"
platformtype=`oc get infrastructures.config.openshift.io cluster -o=jsonpath={.status.platformStatus.type}`
if [ "${platformtype}" == "Azure" ] ; then
  if [ "${AZURE_AUTH_LOCATION_FILE}" == "" ] ; then
    echo "please input AZURE_AUTH_LOCATION"
    exit 1
  fi
  cz "${AZURE_AUTH_LOCATION_FILE}"
fi

eval "$(ssh-agent -s)"
#addsshkey


#git_url_part1=`echo ${REPO} | awk -F ":" '{print $1}'`
git_url_part2=`echo ${REPO} | awk -F ":" '{print $2}'`
github_account=`echo ${git_url_part2} | awk -F "/" '{print $1}'`
github_repo=`echo ${git_url_part2} | awk -F "/" '{print $2}'`
github_reponame=`echo ${github_repo} | awk -F "." '{print $1}'`
echo ${github_account}"---"${github_reponame}

base_dir=`pwd`
repo_dir=${base_dir}/${github_account}"-"${github_reponame}
repo_host="github.com-"${github_account}
echo ${github_account}"-"${github_reponame}"-"${repo_dir}"-"${repo_host}

if [ ! -d ${repo_dir} ]; then
  git clone ${REPO} ${github_account}"-"${github_reponame}
fi
cd ${repo_dir}
ret=`git checkout master 2>&1 || true`
if (echo $ret | grep -E "have diverged") || (echo $ret | grep -E "resolve your current index first") ; then
  echo "there is confilct"
  cd ..
  rm -fr ${repo_dir}
  git clone ${REPO} ${github_account}"-"${github_reponame}
  cd ${repo_dir}
fi
ret=`git pull 2>&1 || true`
if echo $ret | grep -E "Merge conflict in" ; then
  echo "there is confilct"
  cd ..
  rm -fr ${repo_dir}
  git clone ${REPO} ${github_account}"-"${github_reponame}
  cd ${repo_dir}
  git checkout master
fi


ret=`git checkout ${BRANCH} 2>&1 || true`
if echo $ret | grep -E "did not match any file" ; then
  echo "the branch ${BRANCH} does not exist"
  exit 1
fi
if echo $ret | grep -E "have diverged" ; then
  echo "there is confilct"
  cd ..
  rm -fr ${repo_dir}
  git clone ${REPO} ${github_account}"-"${github_reponame}
  cd ${repo_dir}
  git checkout ${BRANCH}
fi
ret=`git pull 2>&1 || true`
if echo $ret | grep -E "Merge conflict in" ; then
  echo "there is confilct"
  cd ..
  rm -fr ${repo_dir}
  git clone ${REPO} ${github_account}"-"${github_reponame}
  cd ${repo_dir}
  git checkout ${BRANCH}
fi

echo ${SCENARIO}
echo ${IMPORTANCE}
if [ ${github_reponame} == "origin" ]; then
  rm -fr junit_e2e_*.xml
  echo "it is origin"
  if [ ${IMPORTANCE} == "all" ] || [ ${IMPORTANCE} == "" ] ; then
    ocrorigin "${SCENARIO}" "" || true
  else
    ocrorigin "${SCENARIO}" ${IMPORTANCE} || true
  fi
elif [ ${github_reponame} == "openshift-tests" ]; then
  rm -fr junit_e2e_*.xml
  echo "it is openshift-tests"
  if [ ${IMPORTANCE} == "all" ] || [ ${IMPORTANCE} == "" ] ; then
    ocropenshift "${SCENARIO}" "" || true
  else
    ocropenshift "${SCENARIO}" ${IMPORTANCE} || true
  fi
fi

set +x

resultfile=`ls -rt -1 junit_e2e_* 2>&1 || true`
echo $resultfile
if (echo $resultfile | grep -E "no matches found") || (echo $resultfile | grep -E "No such file or directory") ; then
  echo "there is no result file generated"
  exit 1
fi
current=`date "+%Y-%m-%d %H:%M:%S"`
date_str=`echo ${current} | awk -F " " '{print $1}'`
date_str=`echo ${date_str}|sed -e "s/-//g"`
time_str=`echo ${current} | awk -F " " '{print $2}'`
time_str=`echo ${time_str}|sed -e "s/://g"`
newresultfile="junit_e2e_"${date_str}"-"${time_str}".xml"
rm -fr handleresult.py
cp -fr /root/bin/handleresult.py .
python3 handleresult.py -a replace -i ${resultfile} -o ${newresultfile}
rm -fr ${resultfile}
# resultsummary=`python3 handleresult.py -a get -i ${newresultfile} 2>&1 `
resultsummary=`python3 handleresult.py -a get -i ${newresultfile} 2>&1 || true`
finalresult=""
if (echo $resultsummary | grep -q -E "FAIL") ; then
  echo "FAIL"
  finalresult="FAIL"
else
  echo "SUCCESS"
  finalresult="SUCCESS"
fi
echo -e "\n\n\n"
echo -e ${resultsummary}
if [ "${finalresult}" == "SUCCESS" ] ; then
  echo "the build is SUCCESS"
  exit 0
else
  echo "the build is FAIL"
  exit 1
fi
