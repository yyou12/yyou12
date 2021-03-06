export const nwpolicyPage = {
    getProjectPolicyURL: (projectName?: string) => {
        if (!projectName) {
            return '/k8s/all-namespaces/networkpolicies'
        }
        else {
            return `/k8s/ns/${projectName}/networkpolicies`
        }

    },
    goToNetworkPolicy: (projectName?: string) => cy.visit(nwpolicyPage.getProjectPolicyURL(projectName)),
    creatPolicyForm: () => cy.get(nwpolicyPageSelectors.createFormButton).should('exist').click().then($form => {
        cy.wrap($form).get('form').should('be.visible')
    }),
    deleteAllPolicies: (projectName?: string) => {
        cy.visit(nwpolicyPage.getProjectPolicyURL(projectName)).its('yaml-create').then(content => {
            deletePolicies(projectName)
        })

        const deletePolicies = (projectName: string) => {
            cy.get('body').then($body => {
                if ($body.find('tr[data-id="0-0"]').length > 0) {
                    cy.get('tr[data-id="0-0"]').find('button[data-test-id]').should('exist').then($rrow => {
                        cy.wrap($rrow).click()
                        cy.wrap($rrow).get(nwpolicyPageSelectors.deletePolicyBtn).click().then($conf => {
                            cy.wrap($conf).get('button[data-test="confirm-action"]').should('exist').click()
                        })
                    })

                    nwpolicyPage.goToNetworkPolicy(projectName).its('yaml-create').then(content => {
                        deletePolicies(projectName)
                    })
                }
            })
        }
    },
    addPodOrNamespace: (groupSelector, key, value) => {
        let button;
        if (groupSelector == nwpolicyPageSelectors.addNamespace) {
            button = nwpolicyPageSelectors.addNSBtn

        }
        else if (groupSelector == nwpolicyPageSelectors.addPod) {
            button = nwpolicyPageSelectors.peerPodBtn
        }
        else {
            throw "pass nwpolicyPageSelectors.addNSBtn or nwpolicyPageSelectors.addPod as group selector"
        }

        cy.get(groupSelector).within(() => {
            cy.byTestID(button).click()
            cy.get(nwpolicyPageSelectors.label).type(key)
            cy.get(nwpolicyPageSelectors.selector).type(value)
        })
    },
    verifyIngressEgressOpts: (text) => {
        let buttonText;
        let index;
        if (text == "Ingress") {
            buttonText = 'Add allowed source'
            index = 1
        }
        else if (text == 'Egress') {
            buttonText = 'Add allowed destination'
            index = 2
        }
        const buttonCount = 3;
        const src_dest_options = ['Allow pods from the same namespace', 'Allow pods from inside the cluster', 'Allow peers by IP block']
        for (let i = 0; i < buttonCount; i++) {
            cy.get('.pf-c-dropdown__toggle.pf-m-secondary').eq(index).should('have.text', buttonText).click()
            cy.get(nwpolicyPageSelectors.srcDestOptions[i]).then(($elem) => {
                cy.wrap($elem).should('have.text', src_dest_options[i]).click()
            })
        }
    }
};

export namespace nwpolicyPageSelectors {
    export const createFormButton = '#yaml-create';
    export const nwPolicyName = 'input[id="name"]';
    export const savePolicy = '#save-changes';
    export const cancelButton = '#cancel';
    export const srcDestOptions = ['#sameNS-link', '#anyNS-link', '#ipblock-link'];
    export const podsList = 'ul.pf-c-tree-view__list[role="group"] > li'
    export const treeNode = 'span.pf-c-tree-view__node-text'
    export const label = 'input[placeholder="Label"]'
    export const selector = 'input[placeholder="Selector"]'
    export const addNamespace = 'div.form-group.co-create-networkpolicy__namespaceselector'
    export const addNSBtn = 'add-peer-namespace-selector'
    export const addPod = 'div.form-group.co-create-networkpolicy__podselector'
    export const peerPodBtn = 'add-peer-pod-selector'
    export const addPort = 'add-port'
    export const deletePolicyBtn = 'button[data-test-action="Delete NetworkPolicy"]'
    export const mainPodBtn = 'add-main-pod-selector'
    export const labelName = 'pairs-list-name'
    export const labelValue = 'pairs-list-value'
    export const addIngress = 'add-ingress'
    export const addEgress = 'add-egress'
    export const showIngressPods = 'show-affected-pods-ingress'
    export const podsTreeViewBtn = '.pf-c-tree-view__node-toggle-icon > svg'
    export const dropdownBtn = 'button[data-test-id="dropdown-button"]'
    export const podsPreviewTree = 'pods-preview-tree'
    export const podsPreviewTitle = 'pods-preview-title'
    export const peerHeader = '#peer-header-0'
    export const portField = '#port-0-port'
    export const cidrField = '#cidr'
}
