# OpenShift Console tests

## Directory Structure
```bash
$ ls frontend
README.md
tests/    -> Cypress auto tests scenario
plugins/  -> webpack-preprocessor, enviornment variables, baseUrl, custom tasks
upstream@ -> dynamically created soft link to upstream helpers 
```

## Local Development
### Clone upstream 'console' repo and create a soft link
```bash
git clone git@github.com:openshift/console.git
ln -s /path/to/upstream/console/frontend/packages/integration-tests-cypress upstream
```
### Install all dependencies
```bash
yarn install
ls -ltr
node_modules/     -> dependencies will be installed at runtime here
```
### Export necessary variables
```bash
export BRIDGE_BASE_ADDRESS=https://console-route
export LOGIN_IDP=kubeadmin
export LOGIN_USERNAME=testuser
export LOGIN_PASSWORD=testpassword
```
### Start Cypress and add/run/debug your tests
```bash
./node_modules/cypress/bin/cypress open

```
