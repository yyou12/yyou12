#!/bin/bash
set -e
env

function run {
  if [ "${SCENARIO}" == "" ] ; then
    echo "please input value for SCENARIO"
    exit 1
  fi
  if ! echo ${JENKINS_SLAVE} | grep -E '^goc([0-9]{2})$'; then
    echo "wrong slave node ${JENKINS_SLAVE}"
    exit 1
  fi
  PIPELINESCRIPT_DIR=${WORKSPACE}"/private/pipeline" && export PATH=${PIPELINESCRIPT_DIR}:$PATH
  if [ ${REPO_OWNER} == "openshift" ]; then
    WORKBUILDDIR=${WORKSPACE}"/private"
  else
    WORKBUILDDIR=${WORKSPACE}"/public"
  fi
  cd ${WORKBUILDDIR}

  put_fake_launch_for_each_profile
  config_env_for_cluster
  id
  date
  select_fail_case_for_official_rerun
  execute
  result_report
}

function config_env_for_cluster {
  mkdir -p /home/jenkins/kubeconf && mkdir -p /home/jenkins/azureauth && \
  echo "export KUBECONFIG=/home/jenkins/kubeconf/kubeconfig" >> ~/.bash_profile && \
  echo "export AZURE_AUTH_LOCATION=/home/jenkins/azureauth/azure_auth.json" >> ~/.bash_profile && \
  echo 'export GOROOT=/usr/local/go' >> ~/.bash_profile && \
  echo 'export GOPATH=/goproject' >> ~/.bash_profile && \
  echo 'export GOCACHE=/gocache' >> ~/.bash_profile && \
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bash_profile && \
  source ~/.bash_profile
  echo "configure kubeconfig, azure authentication or client proxy for the cluster"
  source ${PIPELINESCRIPT_DIR}"/occe4c" ${WORKSPACE} "null"${FLEXY_BUILD} "${CONFIG}"
}
function result_report {
  echo "get result and parse it"
  #LAUNCHTRIAL="yes"
  #if we do not set it, it means LAUNCHTRIAL is no and the launch is treated as official if LAUNCH_NAME and PROFILE_NAME are official
  #if we do not set it as yes, it means LAUNCHTRIAL is yes and the launch is treated as trial although LAUNCH_NAME and PROFILE_NAME are official
  #if LAUNCH_NAME or PROFILE_NAME are not official, the launch will be treated as personal launch.
  ocgr ${WORKBUILDDIR} ${WORKSPACE} ${JENKINS_SLAVE} "null"${LAUNCH_NAME} "null""${PROFILE_NAME}" "null""${LAUNCHTRIAL}" "${TIERN_REPO_OWNER}""-""${REPO_OWNER}" ${BUILD_NUMBER} "null""${FILTERS}" "null""${PAYLOAD_VERSION}"
}

#execute cases
function execute {
  echo "the scenario is \"${SCENARIO}\", and the importance is \"${IMPORTANCE}\""
  eval rm -fr ${WORKSPACE}"/private/junit_e2e_*.xml" ${WORKSPACE}"/public/junit_e2e_*.xml"
  cd ${WORKBUILDDIR}

  case "$REPO_OWNER" in
    openshift)
      echo "run case with oropenshift-tests-private under openshift or your account. similar to ocrd"
      echo "ocr ${TIERN_REPO_OWNER} \"${SCENARIO}\" ${IMPORTANCE} \"null${FILTERS}\""
      ocr ${TIERN_REPO_OWNER} "${SCENARIO}" ${IMPORTANCE} "null${FILTERS}" || true
      ;;
    *)
      echo "run case with oropenshift-tests under your account. similar to ocru"
      echo "ocr null \"${SCENARIO}\" ${IMPORTANCE} \"null${FILTERS}\""
      ocr "null" "${SCENARIO}" ${IMPORTANCE} "null${FILTERS}"|| true
      ;;
  esac
}

#reselect case for rerun only for offical nightly
function select_fail_case_for_official_rerun {
  #ROOT_BUILD_CAUSE=CIBUILDCAUSE,MANUALTRIGGER,DEEPLYNESTEDCAUSES
  if ((echo ${LAUNCH_NAME} | grep -E '^([0-9]{8})-([0-9]{4})$') || \
      (echo ${LAUNCH_NAME} | grep -E '^([0-9]{8})-([0-9]{4})_([0-9]{1,2})$')) && \
      (([[ "${ROOT_BUILD_CAUSE}" == *"MANUALTRIGGER"* ]] && [[ "${ROOT_BUILD_CAUSE}" == *"CIBUILDCAUSE"* ]]) || \
       ([[ "${ROOT_BUILD_CAUSE}" == *"MANUALTRIGGER"* ]] && [[ "${ROOT_BUILD_CAUSE}" == *"TIMERTRIGGER"* ]])) && \
      # [[ "X${BUILD_CAUSE_MANUALTRIGGER}X" != "XX" ]] && [[ "${BUILD_CAUSE_MANUALTRIGGER}" == "true" ]] && \
      [[ "${TIERN_REPO_OWNER}" == "openshift" ]] && [[ "${REPO_OWNER}" == "openshift" ]]; then
    echo "valid launch name with reran pipeline build. Try to find fail case and update SCENARIO"
    failcaseid=`ocgfc ${WORKBUILDDIR} ${WORKSPACE} ${LAUNCH_NAME} "${SCENARIO}" ${BUILD_NUMBER} "null""${FILTERS}" 2>&1 || true`
    echo -e "${failcaseid}"
    result=`echo -e ${failcaseid} | tail -1|xargs`
    if [ "X${result}X" != "XX" ] && [ "X${result}X" != "XNOREPLACEX" ] && [ "X${result}X" != "XNORERUNX" ]; then
      echo -e "Found fail case ID: ${result}"
      SCENARIO="${result}"
    elif [ "X${result}X" == "XNORERUNX" ]; then
      echo "No need to rerun it"
      exit 0
    fi
  else
    echo "no launch name or invalid launch name, or not rerun pipeline build, and keep original ${SCENARIO}"
  fi
}

function put_fake_launch_for_each_profile {
  if [ "${SCENARIO}" == "putfakelaunchforeachprofile" ] ; then
    ocpf ${WORKBUILDDIR} ${WORKSPACE} ${JENKINS_SLAVE}
    exit 0
  fi
}

run
