export const Overview = {
    closeGuidedTour: () => cy.get('#tour-step-footer-secondary').click(),
    isLoaded: () => cy.get('[data-test-id="dashboard"]', { timeout: 60000 }).should('exist')
}

export namespace OverviewSelectors {
    export const skipTour = '[data-test=tour-step-footer-secondary]'
} 