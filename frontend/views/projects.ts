import { listPage } from '../upstream/views/list-page';

export const projectsPage = {
    filterByRequester: (selector: string) => {
        listPage.filter.clickFilterDropdown();
        cy.get(selector).click();
    },
    filterMyProjects: () => projectsPage.filterByRequester('#me'),
    filterSystemProjects: () => projectsPage.filterByRequester('#system'),
    filterUserProjects: () => projectsPage.filterByRequester('#user'),
    checkProjectExists: (projectname: string) => {
        cy.get(`[data-test-id="${projectname}"]`).should('exist');
    },
    checkProjectNotExists: (projectname: string) => {
        cy.get(`[data-test-id="${projectname}"]`).should('not.exist');
    }
}