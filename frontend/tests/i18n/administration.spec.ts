import { checkErrors } from '../../upstream/support';
import { DetailsPageSelector } from '../../upstream/views/details-page';

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

  it('pseudolocalizes cluster settings (admin)', () => {
  	cy.log('test Cluster Settings page pesudo translation');
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
})
