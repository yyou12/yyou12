#!/bin/bash
set -e

function run {
  if [ "${SCENARIO}" == "" ] ; then
    echo "please input value for SCENARIO"
    exit 1
  fi
  source ~/.bash_profile
  export PATH=$PATH:${WORKSPACE}"/private/pipeline"
  if [ ${REPO_OWNER} == "openshift" ]; then
    WORKBUILDDIR=${WORKSPACE}"/private"
  else
    WORKBUILDDIR=${WORKSPACE}"/public"
  fi
  cd ${WORKBUILDDIR}

  cleanup_gobuild_diskspace
  config_env_for_cluster
  execute
  result_report
}

#configure kubeconfig, azure authentication or client proxy for the cluster
function config_env_for_cluster {
  FLEXYURLBASE="https://mastern-jenkins-csb-openshift-qe.cloud.paas.psi.redhat.com/job/Launch%20Environment%20Flexy/"
  KUBECONFIG_FILE=""
  if [ "${CONFIG}" != "" ] ; then
    rm -fr handleconfig.py && eval cp -fr ${WORKSPACE}"/private/pipeline/handleconfig.py" .
    value=` python3 handleconfig.py -a get -y "${CONFIG}" -p environments:ocp4:admin_creds_spec || true `
    if [ "${value}" != "None" ] && [ "${value}" != "failtogetvalue" ] && [ "${value}" != "" ]; then
      KUBECONFIG_FILE=${value}
    else
      echo "there is no kubeconfig or wrong in CONF, please check it !!! and try to config it from flexy"
    fi
  fi
  if [ "${KUBECONFIG_FILE}" != "" ]; then
    echo "the kubeconfig is set directly with CONFIG"
    ck "${KUBECONFIG_FILE}"
  else
    echo "the configuration is set from flexy build"
    if [ "${FLEXY_BUILD}" == "" ]; then
      echo "please input FLEXY_BUILD or set kubeconfig in CONF"
      exit 1
    fi
    # configure kubeconfig
    KUBECONFIG_FILE=${FLEXYURLBASE}${FLEXY_BUILD}"/artifact/workdir/install-dir/auth/kubeconfig"
    ck "${KUBECONFIG_FILE}"
    # configure azure authentication script
    AZURE_AUTH_LOCATION_FILE=${FLEXYURLBASE}${FLEXY_BUILD}"/artifact/workdir/install-dir/terraform.azure.auto.tfvars.json"
    ret_code=`curl -s -k ${AZURE_AUTH_LOCATION_FILE} -o /dev/null  -w "%{http_code}"`
    if [ "${ret_code}" == "200" ]; then
      cz "${AZURE_AUTH_LOCATION_FILE}" 
    fi
    #config client proxy if necessary
    export http_proxy= && export https_proxy=
    CLIENT_PROXY_SETTING_SH=${FLEXYURLBASE}${FLEXY_BUILD}"/artifact/workdir/install-dir/client_proxy_setting.json"
    ret_code=`curl -s -k ${CLIENT_PROXY_SETTING_SH} -o ./client_proxy_setting.json  -w "%{http_code}"`
    if [ "${ret_code}" == "200" ]; then
      http_proxy_url=`cat ./client_proxy_setting.json | jq .http_proxy` && eval export http_proxy=${http_proxy_url}
      https_proxy_url=`cat ./client_proxy_setting.json | jq .https_proxy` && eval export https_proxy=${https_proxy_url}
    fi
  fi
}

function execute {
  echo ${SCENARIO}
  echo ${IMPORTANCE}
  if [ ${IMPORTANCE} == "all" ]; then
    IMPORTANCE=""
  fi
  eval rm -fr ${WORKSPACE}"/private/junit_e2e_*.xml" ${WORKSPACE}"/public/junit_e2e_*.xml"
  cd ${WORKBUILDDIR}

  case "$REPO_OWNER" in
    openshift)
      echo "run case with oropenshift-tests-private under openshift or your account"
      echo "${TIERN_REPO_OWNER} ${SCENARIO} ${IMPORTANCE}"
      ocrd ${TIERN_REPO_OWNER} "${SCENARIO}" ${IMPORTANCE}  || true
      ;;
    *)
      echo "run case with oropenshift-tests under your account"
      echo "${SCENARIO} ${IMPORTANCE}"
      ocru "${SCENARIO}" ${IMPORTANCE} || true
      ;;
  esac
}

function result_report {
  set +x

  cd ${WORKBUILDDIR}
  resultfile=`ls -rt -1 junit_e2e_* 2>&1 || true`
  echo $resultfile
  if (echo $resultfile | grep -E "no matches found") || (echo $resultfile | grep -E "No such file or directory") ; then
    echo "there is no result file generated"
    exit 1
  fi
  current_time=`date "+%Y-%m-%d-%H-%M-%S"`
  newresultfile="junit_e2e_"${current_time}".xml"
  rm -fr handleresult.py && eval cp -fr ${WORKSPACE}"/private/pipeline/handleresult.py" .
  python3 handleresult.py -a replace -i ${resultfile} -o ${newresultfile} && rm -fr ${resultfile}
  resultsummary=`python3 handleresult.py -a get -i ${newresultfile} 2>&1 || true`
  if (echo $resultsummary | grep -q -E "FAIL") ; then
    finalresult="FAIL"
  else
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

  set -x
}

#check go-build disk usage and cleanup it
function cleanup_gobuild_diskspace {
  output_du=`du -s -k /data/go-build`
  size=`echo ${output_du} | awk -F " " '{print $1}'`
  size_threshold=30000000
  if [ ${size} -gt ${size_threshold} ]; then
    go clean -cache
  fi
}

run
