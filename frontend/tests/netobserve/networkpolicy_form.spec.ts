import { checkErrors } from '../../upstream/support';
import { podsList } from '../../data/networkpolicy_form_pods';
import { nwpolicyPage, nwpolicyPageSelectors } from '../../views/nwpolicy-page';

const nProjects = 3;

const createPods = (obj, project) => {
    const filename = [
        Cypress.config('screenshotsFolder')
            .toString()
            .replace('/cypress/screenshots', ''),
        `${obj.metadata.name}.${obj.kind.toLowerCase()}.json`,
    ].join('/');
    cy.writeFile(filename, JSON.stringify(obj));
    cy.log(Cypress.env('KUBECONFIG'))
    cy.exec(`oc create -f ${filename} -n ${project} --kubeconfig=${Cypress.env('KUBECONFIG')}`);
    cy.exec(`rm ${filename}`);
};

// generate 10 character random string for policy name
function getRandomPolicyName() {
    return [...Array(10)].map(i => (~~(Math.random() * 36)).toString(36)).join('')
};

// alias names will be set as "pod0Name", "pod0IP" (corresponding to first pod in list)
// where 0 is the index of pod shown in UI.
// currently this is not used anywhere but created generic function for future use.
function setPodNamesIPAsCypressAlias(project, podsCount) {
    for (let i = 0; i < podsCount; i++) {
        cy.visit(`./k8s/ns/${project}/pods`).get('[data-test-id="resource-title"]').should('exist')

        //get Pod names and IP of pods to be used as an alias.
        cy.get(`tr[data-index="${i}"] > td[id=name] > span > a`).invoke('text').as(`pod${i}Name`)
        cy.get(`tr[data-index="${i}"] > td[id=name] > span > a`).click()
        cy.byTestSelector('details-item-value__Pod IP').should('exist').invoke('text').as(`pod${i}IP`)

    }
}

const verifyConnection = (project, fromPodName, toPodIP, expectedResult) => {
    cy.exec(`oc login -u kubeadmin -p ${Cypress.env('LOGIN_PASSWORD')}`)
    cy.exec(`oc project ${project}`)
    cy.exec(`oc rsh ${fromPodName} curl ${toPodIP}:8080 -m 5`, { failOnNonZeroExit: expectedResult }).then(result => {
        if (expectedResult) {
            cy.log("expecting stdout")
            expect(result.stdout).to.contain('Hello OpenShift')
        }
        else {
            cy.log("expecting stderr")
            expect(result.stderr).to.contain('timed out')
        }

    })
}

describe('Console Network Policies (OCP-41858, NETOBSERV)', () => {
    before(() => {
        cy.login(Cypress.env('LOGIN_IDP'), Cypress.env('LOGIN_USERNAME'), Cypress.env('LOGIN_PASSWORD'));

        cy.switchPerspective('Administrator');

        // create projects and deploy pods.
        for (let i = 0; i < nProjects; i++) {
            cy.createProject('test' + i);
            createPods(podsList, 'test' + i)
        }
        cy.visit('/k8s/all-namespaces/networkpolicies')
    });

    beforeEach(() => {
        const projectName = 'test0'
        cy.visit('/k8s/all-namespaces/networkpolicies')
        cy.get('span.pf-c-menu-toggle__text').should('have.text', 'Project: All Projects').click()
        cy.get('span.pf-c-menu__item-text').contains(projectName).click()

        // get podName and pod IP for project test0 and create aliases to be used later in each test
        const nPods = 2
        const podsUrl = '/k8s/ns/' + projectName + '/pods'
        for (let i = 0; i < nPods; i++) {
            cy.visit(podsUrl).get('[data-test-id="resource-title"]').should('exist')
            cy.get(`tr[data-index="${i}"] > td[id=name] > span > a`).invoke('text').as(`pod${i}Name`)
            cy.get(`tr[data-index="${i}"] > td[id=name] > span > a`).click()
            cy.byTestSelector('details-item-value__Pod IP').should('exist').invoke('text').as(`pod${i}IP`)
        }
        cy.visit(`/k8s/ns/${projectName}/networkpolicies`)
    });


    afterEach(() => {
        checkErrors();
        cy.visit('/k8s/all-namespaces/networkpolicies').its('yaml-create').then(content => {
            expect(content).to.be.visible
        })

        // delete all network policies after each test
        const deleteAllPolicies = () => {
            cy.get('body').then($body => {
                if ($body.find('tr[data-id]').length > 0) {
                    cy.get('tr[data-id="0-0"]').find('button[data-test-id]').should('exist').then($rrow => {
                        cy.wrap($rrow).click()
                        cy.wrap($rrow).get('button[data-test-action="Delete NetworkPolicy"]').click().then($conf => {
                            cy.wrap($conf).get('button[data-test="confirm-action"]').should('exist').click()
                        })
                    })
                    cy.reload()
                    deleteAllPolicies()
                }
            })
        }
        deleteAllPolicies()
    });

    after(() => {
        for (let i = 0; i < nProjects; i++) {
            cy.deleteProject('test' + i);
        }
        cy.logout();
    });

    it('should validate network policy form', () => {
        nwpolicyPage.creatPolicyForm()

        cy.get('input').should('have.id', 'name')
        cy.byTestID('Deny all ingress traffic__checkbox').should('not.be.checked')
        cy.byTestID('Deny all egress traffic__checkbox').should('not.be.checked')
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

        // not verifying egress fields since it depends on network provider.
        // currently there is no way to figure out from UI who's the network provider.

        // verify create and cancel button.
        cy.get(nwpolicyPageSelectors.savePolicy).should('exist')
        cy.get(nwpolicyPageSelectors.cancelButton).should('exist')
    })

    it('should verify deny all ingress policy in same ns', () => {
        // verify ingress traffic from one pod to another pod is not allowed in same project (test0) 

        nwpolicyPage.creatPolicyForm()
        cy.get(nwpolicyPageSelectors.nwPolicyName).type(getRandomPolicyName())
        cy.byTestID('Deny all ingress traffic__checkbox').should('not.be.checked').click()
        cy.get(nwpolicyPageSelectors.savePolicy).click()

        // verify all incoming traffic is rejected to pod 2 from pod 1
        cy.get('@pod0Name').then(pod0Name => {
            cy.get('@pod1IP').then(pod1IP => {
                verifyConnection('test0', pod0Name, pod1IP, false)
            })
        })

    })

    it('should verify deny all egress policy in same ns', () => {
        // verify egress traffic from one pod to another pod is allowed in same project (test0)
        // despite having egress policy.
        nwpolicyPage.creatPolicyForm()
        cy.get(nwpolicyPageSelectors.nwPolicyName).type(getRandomPolicyName())
        cy.byTestID('Deny all egress traffic__checkbox').should('not.be.checked').click()
        cy.get(nwpolicyPageSelectors.savePolicy).click()

        // verify all incoming traffic is rejected to pod 2 from pod 1
        cy.get('@pod0Name').then(pod0Name => {
            cy.get('@pod1IP').then(pod1IP => {
                // if the project is set with just deny egress policy
                // it doesn't take effect by design therefore
                // curl must request suceed here.
                verifyConnection('test0', pod0Name, pod1IP, true)
            })
        })
    })
})
