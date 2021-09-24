export const nwpolicyPage = {
    goToNetworkPolicy: () => cy.visit('/k8s/all-namespaces/networkpolicies'),
    creatPolicyForm: () => cy.get(nwpolicyPageSelectors.createFormButton).should('exist').click().then($form => {
        cy.wrap($form).get('form').should('be.visible')
    })

};

export namespace nwpolicyPageSelectors {
    export const createFormButton = '#yaml-create';
    export const nwPolicyName = 'input[id="name"]';
    export const savePolicy = '#save-changes';
    export const cancelButton = '#cancel';
    export const srcDestOptions = ['#sameNS-link', '#anyNS-link', '#ipblock-link'];
}