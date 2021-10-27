
export namespace podsPageUtils {
    export function setProjectPodNamesAlias(project, label, aliasPrefix, pod_label_key = "name") {
        cy.visit(`/k8s/ns/${project}/pods`).get('[data-test-id="resource-title"]').should('be.visible')
        cy.get('#content-scrollable').within(() => {
            cy.get('button.pf-c-dropdown__toggle').should('have.text', 'Name').click().get('#LABEL-link').click()
            cy.byLegacyTestID('item-filter').type(`${pod_label_key}=${label}`).get('span.co-text-node').contains(label).should('be.visible').click()

            cy.get('tr > td[id=name]').find('a').each(($el, $index) => {
                cy.wrap($el)
                    .invoke('text').as(`${aliasPrefix}_pod${$index}Name`)
            })
        })
    }
    export function setPodIPAlias(project, podName) {
        cy.visit(`./k8s/ns/${project}/pods/${podName}`).byTestSelector('details-item-value__Pod IP').should('be.visible').invoke('text').as(`${podName}IP`)
    }
}
