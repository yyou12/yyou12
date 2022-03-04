#!/bin/bash

set -euo pipefail

ARTIFACT_DIR=${ARTIFACT_DIR:=/tmp/artifacts}
cp -L $KUBECONFIG /tmp/kubeconfig && export KUBECONFIG_PATH=/tmp/kubeconfig
mkdir -p $ARTIFACT_DIR
SCREENSHOTS_DIR=gui_test_screenshots

function copyArtifacts {
  if [ -d "$ARTIFACT_DIR" ] && [ -d "$SCREENSHOTS_DIR" ]; then
    echo "Copying artifacts from $(pwd)..."
    cp -r "$SCREENSHOTS_DIR" "${ARTIFACT_DIR}/gui_test_screenshots"
  fi
}

## Add IDP for testing
# prepare users
users=""
htpass_file=/tmp/users.htpasswd

for i in $(seq 1 5);
do
    username="uiauto-test-${i}"
    password=$(< /dev/urandom tr -dc 'a-z0-9' | fold -w 12 | head -n 1 || true)
    users+="${username}:${password},"
    if [ -f "${htpass_file}" ]; then
        htpasswd -B -b ${htpass_file} "${username}" "${password}"
    else
        htpasswd -c -B -b ${htpass_file} "${username}" "${password}"
    fi
done

# current generation
gen=$(oc get deployment oauth-openshift -n openshift-authentication -o jsonpath='{.metadata.generation}')

# add users to cluster
oc create secret generic uiauto-htpass-secret --from-file=htpasswd=${htpass_file} -n openshift-config
oc patch oauth cluster --type='json'  -p='[{"op": "add", "path": "/spec/identityProviders", "value": [{"type": "HTPasswd", "name": "uiauto-htpasswd-idp", "mappingMethod": "claim", "htpasswd":{"fileData":{"name": "uiauto-htpass-secret"}}}]}]'

## wait for oauth-openshift to rollout
wait_auth=true
expected_replicas=$(oc get deployment oauth-openshift -n openshift-authentication -o jsonpath='{.spec.replicas}')
while $wait_auth;
do
    available_replicas=$(oc get deployment oauth-openshift -n openshift-authentication -o jsonpath='{.status.availableReplicas}')
    new_gen=$(oc get deployment oauth-openshift -n openshift-authentication -o jsonpath='{.metadata.generation}')
    if [[ $expected_replicas == "$available_replicas" && $((new_gen)) -gt $((gen)) ]]; then
        wait_auth=false
    else
        sleep 10
    fi
done
echo "authentication operator finished updating"
trap copyArtifacts EXIT

# clone upstream console repo and create soft link
set -x
git clone -b master https://github.com/openshift/console.git upstream_console && cd upstream_console/frontend && yarn install 
cd ../../
ln -s ./upstream_console/frontend/packages/integration-tests-cypress upstream

# in frontend dir, install deps and trigger tests
yarn install

# trigger tests
set +x
console_route=$(oc get route console -n openshift-console -o jsonpath='{.spec.host}')
export BRIDGE_BASE_ADDRESS=https://$console_route
export LOGIN_IDP=uiauto-htpasswd-idp
export LOGIN_USERNAME=uiauto-test-1
export LOGIN_PASSWORD=$(echo $users | awk -F ',' '{print $1}' | awk -F ':' '{print $2}')
export LOGIN_UP_PAIR=$users
ls -ltr
echo "triggering tests"
set -x
yarn run test-cypress-console-headless
