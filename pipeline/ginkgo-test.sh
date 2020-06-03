#!/bin/bash
set -e

function run {
  if [ "${SCENARIO}" == "" ] ; then
    echo "please input value for SCENARIO"
    exit 1
  fi
  if echo ${JENKINS_SLAVE} | grep -E '^ginkgo-slave-oc([0-9]{2})$'; then
    source ~/.bash_profile
  fi
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
  if echo ${JENKINS_SLAVE} | grep -E '^ginkgo-slave-oc([0-9]{2})$'; then
    echo "get oc client"
    getoc ${JENKINS_SLAVE} ${PIPELINESCRIPT_DIR}
  else
    mkdir -p /home/jenkins/kubeconf && mkdir -p /home/jenkins/azureauth && \
    echo "export KUBECONFIG=/home/jenkins/kubeconf/kubeconfig" >> ~/.bash_profile && \
    echo "export AZURE_AUTH_LOCATION=/home/jenkins/azureauth/azure_auth.json" >> ~/.bash_profile && \
    echo 'export GOROOT=/usr/local/go' >> ~/.bash_profile && \
    echo 'export GOPATH=/goproject' >> ~/.bash_profile && \
    echo 'export GOCACHE=/gocache' >> ~/.bash_profile && \
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bash_profile && \
    source ~/.bash_profile
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

run
