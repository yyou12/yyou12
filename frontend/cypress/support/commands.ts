import { nav } from '../../upstream/views/nav';

declare global {
    namespace Cypress {
        interface Chainable<Subject> {
            switchPerspective(
                perspective: string,
            );
        }
    }
}

Cypress.Commands.add("switchPerspective", (perspective: string) => {
    cy.get('#nav-toggle').click()
    nav.sidenav.switcher.changePerspectiveTo(perspective);
    nav.sidenav.switcher.shouldHaveText(perspective);
});
