#!/bin/bash
set -e

function run {
  if [ "${SCENARIO}" == "" ] ; then
    echo "please input value for SCENARIO"
    exit 1
  fi
  source ~/.bash_profile
  PIPELINESCRIPT_DIR=${WORKSPACE}"/private/pipeline" && export PATH=${PIPELINESCRIPT_DIR}:$PATH
  if [ ${REPO_OWNER} == "openshift" ]; then
    WORKBUILDDIR=${WORKSPACE}"/private"
  else
    WORKBUILDDIR=${WORKSPACE}"/public"
  fi
  cd ${WORKBUILDDIR}

  config_env_for_cluster
  execute
  result_report
}

function config_env_for_cluster {
  echo "get oc client"
  if echo ${JENKINS_SLAVE} | grep -E '^ginkgo-slave-oc([0-9]{2})$'; then
    getoc ${JENKINS_SLAVE} ${PIPELINESCRIPT_DIR}
  fi
  echo "configure kubeconfig, azure authentication or client proxy for the cluster"
  source ${PIPELINESCRIPT_DIR}"/occe4c" ${WORKSPACE} "null"${FLEXY_BUILD} "${CONFIG}"
}
function result_report {
  echo "get result and parse it"
  ocgr ${WORKBUILDDIR} ${WORKSPACE}
}

#execute cases
function execute {
  echo "the scenario is \"${SCENARIO}\", and the importance is \"${IMPORTANCE}\""
  if [ ${IMPORTANCE} == "all" ]; then
    IMPORTANCE=""
  fi
  eval rm -fr ${WORKSPACE}"/private/junit_e2e_*.xml" ${WORKSPACE}"/public/junit_e2e_*.xml"
  cd ${WORKBUILDDIR}

  case "$REPO_OWNER" in
    openshift)
      echo "run case with oropenshift-tests-private under openshift or your account"
      echo "ocrd ${TIERN_REPO_OWNER} ${SCENARIO} ${IMPORTANCE}"
      ocrd ${TIERN_REPO_OWNER} "${SCENARIO}" ${IMPORTANCE}  || true
      ;;
    *)
      echo "run case with oropenshift-tests under your account"
      echo "ocru ${SCENARIO} ${IMPORTANCE}"
      ocru "${SCENARIO}" ${IMPORTANCE} || true
      ;;
  esac
}

run
