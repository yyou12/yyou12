import { checkErrors } from '../../upstream/support';
import { DetailsPageSelector } from '../../upstream/views/details-page';
import { listPage, ListPageSelector } from '../../upstream/views/list-page';

describe('Administration pages pesudo translation', () => {
  before(() => {
  	cy.login(Cypress.env('LOGIN_IDP'), Cypress.env('LOGIN_USERNAME'), Cypress.env('LOGIN_PASSWORD'));
  });

  afterEach(() => {
  	checkErrors();
  });

  after(() => {
  	cy.logout;
  });

  it('cluster settings details (OCP-35766,admin)', () => {
  	cy.visit('/settings/cluster?pseudolocalization=true&lng=en');
  	cy.get('.co-cluster-settings__section', {timeout: 10000});
  	cy.get(DetailsPageSelector.horizontalNavTabs).isPseudoLocalized();
  	cy.get('.pf-c-alert__title').isPseudoLocalized();
  	cy.get('.co-cluster-settings').isPseudoLocalized();
  	cy.get(DetailsPageSelector.horizontalNavTabs).isPseudoLocalized();
  	cy.get(DetailsPageSelector.itemLabels).isPseudoLocalized();
  	cy.get(DetailsPageSelector.sectionHeadings).isPseudoLocalized();
  	cy.get('th').isPseudoLocalized();
  });

  it('cluster settings cluster operators', () => {
    cy.visit('/settings/cluster/clusteroperators?pseudolocalization=true&lng=en');
    listPage.rows.shouldBeLoaded();
    cy.get(ListPageSelector.tableColumnHeaders).isPseudoLocalized();
  });

  it('cluster settings configurations (OCP-35766,admin)', () => {
    cy.visit('/settings/cluster/globalconfig?pseudolocalization=true&lng=en');
    listPage.rows.shouldBeLoaded();
    cy.byLegacyTestID('item-filter').isPseudoLocalized();
    cy.get('.co-help-text').isPseudoLocalized();
  });
})
