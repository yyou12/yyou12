import { checkErrors } from '../../upstream/support';
import { nwpolicyPage, nwpolicyPageSelectors } from '../../views/nwpolicy-page';
import { OCCreds, OCCli } from '../../views/cluster-cliops';
import { podsPageUtils } from '../../views/pods';
import testFixture from '../../fixtures/network_policy_form_test.json'
import * as utils from '../../views/utils'
import { helperfuncs, VerifyPolicyForm } from '../../views/nwpolicy-utils'

const projects: string[] = ['test0', 'test1', 'test2']
const podLabels: string[] = ["test-pods", 'test-pods2']
const pod_label_key = 'name';
const ns_label_key = 'kubernetes.io/metadata.name'
const fixtureFile = 'network_policy_form_test'

before('root-level: any test suite', function () {
    let tmpFile = `/tmp/${utils.getRandomName()}`
    cy.writeFile(tmpFile, JSON.stringify(testFixture))

    let creds: OCCreds = { idp: Cypress.env('LOGIN_IDP'), user: Cypress.env('LOGIN_USERNAME'), password: Cypress.env('LOGIN_PASSWORD') }
    cy.login(creds.idp, creds.user, creds.password);
    cy.switchPerspective('Administrator');
    this.cli = new OCCli(creds)
    this.creds = creds

    helperfuncs.setNetworkProviderAlias()

    // create projects and deploy pods.
    for (let i = 0; i < projects.length; i++) {
        cy.createProject(projects[i])
        this.cli.createPods(tmpFile, projects[i])
    }

    cy.fixture(fixtureFile).as('testData')
    cy.exec(`rm ${tmpFile}`);

});

describe('Console Network Policies (OCP-41858, NETOBSERV)', function () {
    before('any test', function () {

        /* set pod names aliases */
        projects.forEach((project) => {
            podLabels.forEach(label => {
                let modLabel = label.replace('-', '')
                let aliasPrefix = `${project}_${modLabel}`
                podsPageUtils.setProjectPodNamesAlias(project, label, aliasPrefix)
            })
        })

    })

    beforeEach('test', function () {
        cy.visit('/k8s/all-namespaces/networkpolicies')
        cy.get('span.pf-c-menu-toggle__text').should('have.text', 'Project: All Projects').click()
        cy.get('span.pf-c-menu__item-text').contains('test0').click()

    })

    afterEach(() => {
        checkErrors();
    });

    after(function () {
        projects.forEach((project) => {
            cy.deleteProject(project);
        })
        cy.logout();
    })

    describe("UI validating tests", function () {

        it('should validate network policy form', function () {
            nwpolicyPage.goToNetworkPolicy()
            nwpolicyPage.creatPolicyForm()

            cy.get('input[id="name"]').should('have.attr', 'name')
            cy.byTestID('Deny all ingress traffic__checkbox').should('not.be.checked')
            cy.get('button').contains('Add ingress rule').click()

            // verify ingress sources.
            var src_dest_options = ['Allow pods from the same namespace', 'Allow pods from inside the cluster', 'Allow peers by IP block']

            const buttonCount = 3;
            for (let i = 0; i < buttonCount; i++) {
                cy.get('button.pf-c-dropdown__toggle.pf-c-button.pf-m-secondary').should('have.text', 'Add allowed source').click()
                cy.get(nwpolicyPageSelectors.srcDestOptions[i]).then(($elem) => {
                    cy.wrap($elem).should('have.text', src_dest_options[i]).click()
                })
            }
            cy.get('button').contains('Remove')

            // verify create and cancel button.
            cy.get(nwpolicyPageSelectors.savePolicy).should('exist')
            cy.get(nwpolicyPageSelectors.cancelButton).should('exist')
        })
    })

    describe("network policy end-to-end tests", function () {
        before('any end-to-end test', function () {
            /* map labels to number replicas for pods to those labels */
            var labels_npods = new Map()
            this.testData.items.filter(item => (item.kind == 'ReplicationController')).forEach((item) => { labels_npods.set(item.spec.template.metadata.labels.name, item.spec.replicas) })

            /* set aliases for pod IP */
            for (let p = 0; p < projects.length - 1; p++) {
                let project = projects[p]
                podLabels.forEach(label => {
                    let modLabel = label.replace('-', '')
                    let aliasPrefix = `${project}_${modLabel}`
                    for (let np = 0; np < labels_npods.get(label); np++) {
                        let podNameAlias = `${aliasPrefix}_pod${np}Name`
                        podsPageUtils.setPodIPAlias(project, this[podNameAlias])
                    }
                })
            }
        })

        beforeEach('end-to-end test', function () {
            if (this.currentTest.parent.fullTitle().includes('OVN') && this.networkprovider.includes('OpenShiftSDN')) {
                this.skip()
            }

            cy.visit('/k8s/all-namespaces/networkpolicies')
            cy.get('span.pf-c-menu-toggle__text').should('have.text', 'Project: All Projects').click()
            cy.get('span.pf-c-menu__item-text').contains('test0').click()
        })

        describe("ingress policies tests", function () {
            beforeEach('ingress tests', function () {
                nwpolicyPage.creatPolicyForm()
                cy.get(nwpolicyPageSelectors.nwPolicyName).type(utils.getRandomName())

                if (this.currentTest.title.includes('deny all')) {
                    return
                }
                cy.get('button').contains('Add ingress rule').click()
                cy.get('button.pf-c-dropdown__toggle.pf-c-button.pf-m-secondary').should('have.text', 'Add allowed source').click()
            })

            it('ingress: should verify deny all', function () {
                /* verify ingress traffic from one pod to another pod is not allowed in same project (test0)  */
                cy.byTestID('Deny all ingress traffic__checkbox').should('not.be.checked').click()
                cy.get(nwpolicyPageSelectors.savePolicy).click()

                /* verify all incoming traffic is rejected to pod 2 from pod 1 in NS test0 */
                let mod_label1 = podLabels[0].replace('-', '')
                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project0_label1_pod1Name = this[`${project0_label1_prefix}_pod1Name`]
                let verify = new VerifyPolicyForm(this.cli)
                verify.connection(projects[0], this[`${project0_label1_prefix}_pod0Name`], this[`${project0_label1_pod1Name}IP`], 'timed out', false)
            })

            it('ingress: should allow pods from the same NS', function () {
                /*  apply policy to all pods in namespace only allow traffic from pods with label name=test-pods2 */
                cy.get(nwpolicyPageSelectors.srcDestOptions[0]).click()
                cy.get('#peer-header-0').should('have.text', 'Allow traffic from pods in the same namespace')

                cy.get(nwpolicyPageSelectors.addPod).eq(1).within(() => {
                    cy.get('button').should('have.text', "Add pod selector").click()
                    cy.get(nwpolicyPageSelectors.label).type(pod_label_key)
                    cy.get(nwpolicyPageSelectors.selector).type(podLabels[1])
                })

                cy.get(nwpolicyPageSelectors.savePolicy).click()
                let mod_label1 = podLabels[0].replace('-', '')
                let mod_label2 = podLabels[1].replace('-', '')
                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project0_label2_prefix = `${projects[0]}_${mod_label2}`
                let project1_label1_prefix = `${projects[1]}_${mod_label1}`

                let verify = new VerifyPolicyForm(this.cli)

                /* connection from NS test0 with label:name=test-pods  to other pod with same label in same NS - should fail */

                let project0_label1_pod1Name = this[`${project0_label1_prefix}_pod1Name`]
                verify.connection(projects[0], this[`${project0_label1_prefix}_pod0Name`], this[`${project0_label1_pod1Name}IP`], 'timed out', false)

                /* connection from NS test0 from pod with label:name=test-pods2 to pod with label:name=test-pods in same NS - should pass */
                verify.connection(projects[0], this[`${project0_label2_prefix}_pod0Name`], this[`${project0_label1_pod1Name}IP`], "Hello OpenShift")

                /* connection from NS test0 with label:name=test-pods to pod with label:name=test-pods2 - should fail */
                let project0_label2_pod0Name = this[`${project0_label2_prefix}_pod0Name`]
                verify.connection(projects[0], this[`${project0_label1_prefix}_pod0Name`], this[`${project0_label2_pod0Name}IP`], 'timed out', false)

                /* connection from NS test1 from pod with label:name=test-pods2 to pod with label:name=test-pods in NS test0 - should fail */
                verify.connection(projects[1], this[`${project1_label1_prefix}_pod1Name`], this[`${project0_label1_pod1Name}IP`], 'timed out', false)
            })

            it('ingress: should allow pods from different NS', function () {
                cy.get(nwpolicyPageSelectors.srcDestOptions[1]).click()

                cy.get('#peer-header-0').should('have.text', 'Allow traffic from pods inside the cluster')

                cy.get(nwpolicyPageSelectors.addNamespace).parent().within(() => {

                    cy.get(nwpolicyPageSelectors.addNamespace).within(() => {
                        cy.get('button').should('have.text', "Add namespace selector").click()
                        cy.get(nwpolicyPageSelectors.label).type(ns_label_key)
                        cy.get(nwpolicyPageSelectors.selector).type(projects[1])
                    })

                    cy.get(nwpolicyPageSelectors.addPod).within(() => {
                        cy.get('button').should('have.text', "Add pod selector").click()
                        cy.get(nwpolicyPageSelectors.label).type(pod_label_key)
                        cy.get(nwpolicyPageSelectors.selector).type(podLabels[1])
                    })
                })

                cy.get(nwpolicyPageSelectors.savePolicy).click()
                let mod_label1 = podLabels[0].replace('-', '')
                let mod_label2 = podLabels[1].replace('-', '')
                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project0_label2_prefix = `${projects[0]}_${mod_label2}`
                let project1_label2_prefix = `${projects[1]}_${mod_label2}`
                let project2_label2_prefix = `${projects[2]}_${mod_label2}`

                let project0label1pod1Name = this[`${project0_label1_prefix}_pod1Name`]
                let verify = new VerifyPolicyForm(this.cli)

                /* connection from pods with l:name=test-pods2 from NS test1 to pods in NS test0 should succeed. */
                verify.connection(projects[1], this[`${project1_label2_prefix}_pod1Name`], this[`${project0label1pod1Name}IP`], "Hello OpenShift")

                /* connection from pods with l:name=test-pods2 from NS test2 to pods in NS test0 should fail. */
                verify.connection(projects[2], this[`${project2_label2_prefix}_pod0Name`], this[`${project0label1pod1Name}IP`], 'timed out', false)

                /* connection from pods with l:name=test-pods2 in same NS test0 should fail */
                verify.connection(projects[0], this[`${project0_label2_prefix}_pod0Name`], this[`${project0label1pod1Name}IP`], 'timed out', false)

            })

            it("ingress: should validate peers by IP block", function () {
                cy.get(nwpolicyPageSelectors.srcDestOptions[2]).click()
                cy.get('#peer-header-0').should('have.text', 'Allow traffic from peers by IP block')

                let [mod_label1, mod_label2] = [podLabels[0].replace('-', ''), podLabels[1].replace('-', '')]

                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project1_label1_prefix = `${projects[1]}_${mod_label1}`
                let project2_label2_prefix = `${projects[2]}_${mod_label2}`

                let project0label1pod0Name = this[`${project0_label1_prefix}_pod0Name`]
                let project1label1pod0Name = this[`${project1_label1_prefix}_pod0Name`]
                let project2label2pod0Name = this[`${project2_label2_prefix}_pod0Name`]

                let project1label1pod0IP = this[`${project1label1pod0Name}IP`]
                cy.get('#cidr').type(`${project1label1pod0IP}/32`)
                cy.get(nwpolicyPageSelectors.addPort).click()
                cy.get('#port-0-port').type('8080')

                cy.get(nwpolicyPageSelectors.savePolicy).click()

                let project0label1pod0IP = this[`${project0label1pod0Name}IP`]
                let verify = new VerifyPolicyForm(this.cli)

                /* connection to pod in NS: test0 from NS: test1 pod0 IP should pass on port 8080 */
                verify.connection(projects[1], project1label1pod0Name, project0label1pod0IP, "Hello OpenShift")

                /* connection to pod in NS: test0 from NS: test1 pod0 IP should fail on port 8888 */
                verify.connection(projects[1], project1label1pod0Name, project0label1pod0IP, "timed out", false, "8888")

                /* connection to pod in NS: test0 from NS: test2 pod0 IP should fail on port 8080 */
                verify.connection(projects[2], project2label2pod0Name, project0label1pod0IP, "timed out", false)
            })
        })

        describe('egress policies tests (OVN)', function () {
            beforeEach('egress test', function () {
                nwpolicyPage.creatPolicyForm()
                cy.get(nwpolicyPageSelectors.nwPolicyName).type(utils.getRandomName())

                if (this.currentTest.title.includes('deny all')) {
                    return
                }

                cy.get('button').contains('Add egress rule').click()
                cy.get('button.pf-c-dropdown__toggle.pf-c-button.pf-m-secondary').should('have.text', 'Add allowed destination').click()
            })

            it('egress: should verify deny all', function () {
                cy.byTestID('Deny all egress traffic__checkbox').should('not.be.checked').click()
                cy.get(nwpolicyPageSelectors.savePolicy).click()

                let mod_label1 = podLabels[0].replace('-', '')
                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project0_label1_pod1Name = this[`${project0_label1_prefix}_pod1Name`]

                let verify = new VerifyPolicyForm(this.cli)

                // verify connection can't be made from pod1 to pod0 in same NS.
                verify.connection(projects[0], this[`${project0_label1_prefix}_pod0Name`], this[`${project0_label1_pod1Name}IP`], 'timed out', false)
            })

            it('egress: should allow traffic to pods in the same namespace', function () {
                cy.get(nwpolicyPageSelectors.srcDestOptions[0]).click()
                cy.get('#peer-header-0').should('have.text', 'Allow traffic to pods in the same namespace')

                cy.get(nwpolicyPageSelectors.addPod).eq(1).within(() => {
                    cy.get('button').should('have.text', "Add pod selector").click()
                    cy.get(nwpolicyPageSelectors.label).type(pod_label_key)
                    cy.get(nwpolicyPageSelectors.selector).type(podLabels[1])
                })

                cy.get(nwpolicyPageSelectors.savePolicy).click()
                let mod_label1 = podLabels[0].replace('-', '')
                let mod_label2 = podLabels[1].replace('-', '')
                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project0_label2_prefix = `${projects[0]}_${mod_label2}`
                let project1_label2_prefix = `${projects[1]}_${mod_label2}`

                let verify = new VerifyPolicyForm(this.cli)


                let project0_label1_pod1Name = this[`${project0_label1_prefix}_pod1Name`]
                let project0_label2_pod0Name = this[`${project0_label2_prefix}_pod0Name`]

                /* connection from NS test0 with label:name=test-pods  to other pod with same label in same NS - should fail */

                verify.connection(projects[0], this[`${project0_label1_prefix}_pod0Name`], this[`${project0_label1_pod1Name}IP`], 'timed out', false)

                /* connection from NS test0 from pod with label:name=test-pods2 to other pod with same label and in same NS - should pass */
                verify.connection(projects[0], this[`${project0_label2_prefix}_pod1Name`], this[`${project0_label2_pod0Name}IP`], "Hello OpenShift")

                /* connection from NS test0 with label:name=test-pods2 to pod with label:name=test-pods - should fail */
                let project0_label1_pod0Name = this[`${project0_label1_prefix}_pod0Name`]
                verify.connection(projects[0], this[`${project0_label2_prefix}_pod0Name`], this[`${project0_label1_pod0Name}IP`], 'timed out', false)

                /* connection from NS test1 from pod with label:name=test-pods to pod with label:name=test-pods in NS test1 - should pass */
                verify.connection(projects[1], this[`${project1_label2_prefix}_pod1Name`], this[`${project0_label1_pod0Name}IP`], 'Hello OpenShift')

            })

            it('egress: should allow traffic to different NS', function () {
                cy.get(nwpolicyPageSelectors.srcDestOptions[1]).click()

                cy.get('#peer-header-0').should('have.text', 'Allow traffic to pods inside the cluster')

                cy.get(nwpolicyPageSelectors.addNamespace).parent().within(() => {

                    cy.get(nwpolicyPageSelectors.addNamespace).within(() => {
                        cy.get('button').should('have.text', "Add namespace selector").click()
                        cy.get(nwpolicyPageSelectors.label).type(ns_label_key)
                        cy.get(nwpolicyPageSelectors.selector).type(projects[1])
                    })

                    cy.get(nwpolicyPageSelectors.addPod).within(() => {
                        cy.get('button').should('have.text', "Add pod selector").click()
                        cy.get(nwpolicyPageSelectors.label).type(pod_label_key)
                        cy.get(nwpolicyPageSelectors.selector).type(podLabels[1])
                    })
                })

                cy.get(nwpolicyPageSelectors.savePolicy).click()
                let mod_label1 = podLabels[0].replace('-', '')
                let mod_label2 = podLabels[1].replace('-', '')
                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project0_label2_prefix = `${projects[0]}_${mod_label2}`
                let project1_label1_prefix = `${projects[1]}_${mod_label1}`
                let project1_label2_prefix = `${projects[1]}_${mod_label2}`

                let project0_label2_pod1Name = this[`${project0_label2_prefix}_pod1Name`]
                let project1_label1_pod1Name = this[`${project1_label1_prefix}_pod1Name`]
                let project1_label2_pod1Name = this[`${project1_label2_prefix}_pod1Name`]

                let verify = new VerifyPolicyForm(this.cli)

                /* connection from NS test0 pods should only succeed to pods in NS test1 pods with label name=test-pods2.
                   connections to all other pods should fail.
                */
                verify.connection(projects[0], this[`${project0_label1_prefix}_pod1Name`], this[`${project1_label1_pod1Name}IP`], "timed out", false)

                verify.connection(projects[0], this[`${project0_label1_prefix}_pod1Name`], this[`${project1_label2_pod1Name}IP`], "Hello OpenShift")

                verify.connection(projects[0], this[`${project0_label1_prefix}_pod0Name`], this[`${project0_label2_pod1Name}IP`], 'timed out', false)
            })

            it('egress: should allow traffic to peers by CIDR', function () {
                cy.get(nwpolicyPageSelectors.srcDestOptions[2]).click()
                cy.get('#peer-header-0').should('have.text', 'Allow traffic to peers by IP block')

                let [mod_label1, mod_label2] = [podLabels[0].replace('-', ''), podLabels[1].replace('-', '')]

                let project0_label1_prefix = `${projects[0]}_${mod_label1}`
                let project0_label2_prefix = `${projects[0]}_${mod_label2}`
                let project1_label1_prefix = `${projects[1]}_${mod_label1}`

                let project0_label1_pod0Name = this[`${project0_label1_prefix}_pod0Name`]
                let project0_label2_pod0Name = this[`${project0_label2_prefix}_pod0Name`]

                let project0_label1_pod0IP = this[`${project0_label1_pod0Name}IP`]

                cy.get('#cidr').type(`${project0_label1_pod0IP}/32`)
                cy.get(nwpolicyPageSelectors.addPort).click()
                cy.get('#port-0-port').type('8080')

                cy.get(nwpolicyPageSelectors.savePolicy).click()


                let verify = new VerifyPolicyForm(this.cli)

                /* pods in same NS test0 can only reach to pod whose CIDR and port matches; no other pod can be reached in NS test0 by other pods in same NS */
                verify.connection(projects[0], this[`${project0_label2_prefix}_pod0Name`], project0_label1_pod0IP, "Hello OpenShift")

                verify.connection(projects[0], this[`${project0_label2_prefix}_pod0Name`], project0_label1_pod0IP, "timed out", false, '8888')

                verify.connection(projects[0], this[`${project0_label1_prefix}_pod1Name`], this[`${project0_label2_pod0Name}IP`], "timed out", false)

                /* however pods from different NS can reach to any pods of NS test0 */
                verify.connection(projects[1], this[`${project1_label1_prefix}_pod1Name`], this[`${project0_label2_pod0Name}IP`], "Hello OpenShift")
            })
        })


        afterEach(() => {
            nwpolicyPage.deleteAllPolicies()
        });
    })
})
