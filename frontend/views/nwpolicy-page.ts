export const nwpolicyPage = {
    goToNetworkPolicy: () => cy.visit('/k8s/all-namespaces/networkpolicies'),
    creatPolicyForm: () => cy.get(nwpolicyPageSelectors.createFormButton).should('exist').click().then($form => {
        cy.wrap($form).get('form').should('be.visible')
    }),
    deleteAllPolicies: () => {
        cy.visit('/k8s/all-namespaces/networkpolicies').its('yaml-create').then(content => {
            expect(content).to.be.visible
        })

        const deletePolicies = () => {
            cy.get('body').then($body => {
                if ($body.find('tr[data-id="0-0"]').length > 0) {
                    cy.get('tr[data-id="0-0"]').find('button[data-test-id]').should('exist').then($rrow => {
                        cy.wrap($rrow).click()
                        cy.wrap($rrow).get(nwpolicyPageSelectors.deletePolicyBtn).click().then($conf => {
                            cy.wrap($conf).get('button[data-test="confirm-action"]').should('exist').click()
                        })
                    })

                    nwpolicyPage.goToNetworkPolicy().its('yaml-create').then(content => {
                        deletePolicies()
                    })
                }
            })
        }
        deletePolicies()
    }
};

export namespace nwpolicyPageSelectors {
    export const createFormButton = '#yaml-create';
    export const nwPolicyName = 'input[id="name"]';
    export const savePolicy = '#save-changes';
    export const cancelButton = '#cancel';
    export const srcDestOptions = ['#sameNS-link', '#anyNS-link', '#ipblock-link'];
    export const podsList = 'ul.pf-c-tree-view__list[role="group"] > li'
    export const showPodsList = 'div.pf-c-popover__body'
    export const treeNode = 'span.pf-c-tree-view__node-text'
    export const label = 'input[placeholder="Label"]'
    export const selector = 'input[placeholder="Selector"]'
    export const podSelectorBtn = 'svg.co-icon-space-r'
    export const addNamespace = 'div.form-group.co-create-networkpolicy__namespaceselector'
    export const addPod = 'div.form-group.co-create-networkpolicy__podselector'
    export const addPort = '.co-create-networkpolicy__ports-list > .co-toolbar__group > .pf-c-button'
    export const deletePolicyBtn = 'button[data-test-action="Delete NetworkPolicy"]'
}
