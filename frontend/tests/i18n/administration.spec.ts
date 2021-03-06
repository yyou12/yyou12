import { checkErrors } from '../../upstream/support';
import { DetailsPageSelector } from '../../upstream/views/details-page';
import { listPage, ListPageSelector } from '../../upstream/views/list-page';

describe('Administration pages pesudo translation (OCP-35766,admin)', () => {
  before(() => {
    cy.exec(`oc adm policy add-cluster-role-to-user cluster-admin ${Cypress.env('LOGIN_USERNAME')} --kubeconfig ${Cypress.env('KUBECONFIG_PATH')}`);
  	cy.login(Cypress.env('LOGIN_IDP'), Cypress.env('LOGIN_USERNAME'), Cypress.env('LOGIN_PASSWORD'));
  });

  afterEach(() => {
  	checkErrors();
  });

  after(() => {
    cy.exec(`oc adm policy remove-cluster-role-from-user cluster-admin ${Cypress.env('LOGIN_USERNAME')} --kubeconfig ${Cypress.env('KUBECONFIG_PATH')}`);
  	cy.logout;
  });

  it('cluster settings details', () => {
    cy.visit('/settings/cluster?pseudolocalization=true&lng=en');
    cy.get('.co-cluster-settings__section', {timeout: 10000});
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

 it('cluster settings configurations', () => {
    cy.visit('/settings/cluster/globalconfig?pseudolocalization=true&lng=en');
    cy.get('.co-m-table-grid', {timeout: 10000});
    cy.get('.co-help-text').isPseudoLocalized();
    cy.byLegacyTestID('item-filter').isPseudoLocalized();
    cy.get('.co-m-table-grid__head').isPseudoLocalized();
  });

  it('Namespaces list and other pages pesudo translation', () => {
    // list page
    const test_ns = 'openshift-apiserver'
    cy.visit('/k8s/cluster/namespaces?pseudolocalization=true&lng=en');
    listPage.rows.shouldBeLoaded();
    cy.testI18n([ListPageSelector.tableColumnHeaders], ['item-create']);
    cy.byLegacyTestID('kebab-button').first().click();
    cy.get('.pf-c-dropdown__menu-item').isPseudoLocalized();

    //details page
    cy.get('a.co-resource-item__resource-name').first().click();
    cy.get(DetailsPageSelector.horizontalNavTabs).isPseudoLocalized();
    cy.get(DetailsPageSelector.itemLabels).isPseudoLocalized();
    cy.get(DetailsPageSelector.sectionHeadings).isPseudoLocalized();
    cy.byLegacyTestID('actions-menu-button').click();
    cy.get('.pf-c-dropdown__menu-item').isPseudoLocalized();

    // RoleBindings tab
    cy.visit(`/k8s/cluster/namespaces/${test_ns}/roles?pseudolocalization=true&lng=en`);
    listPage.rows.shouldBeLoaded();
    cy.testI18n([ListPageSelector.tableColumnHeaders], ['item-create']);    
  });

    // ResourceQuota and LimitRange has been covered in resource-crud.spec

  it('CustomResourceDefinitions list and details pesudo translation', () => {
    const CRD_kind_group = 'consolequickstarts.console.openshift.io';
    cy.visit('/k8s/cluster/customresourcedefinitions?pseudolocalization=true&lng=en');
    listPage.rows.shouldBeLoaded();
    cy.testI18n([ListPageSelector.tableColumnHeaders], ['item-create']);
    // CRD details
    cy.byLegacyTestID('item-filter').type('consolequickstart');
    cy.byLegacyTestID('ConsoleQuickStart').click();
    cy.get(DetailsPageSelector.horizontalNavTabs).isPseudoLocalized();
    cy.get(DetailsPageSelector.itemLabels).isPseudoLocalized();
    cy.get(DetailsPageSelector.sectionHeadings).isPseudoLocalized();
    cy.get('.co-m-table-grid__head').isPseudoLocalized();
    cy.byLegacyTestID('actions-menu-button').click();
    cy.get('.pf-c-dropdown__menu-item').isPseudoLocalized();
    // Instances page
    cy.visit(`/k8s/cluster/customresourcedefinitions/${CRD_kind_group}/instances?pseudolocalization=true&lng=en`);
    listPage.rows.shouldBeLoaded();
    cy.testI18n([ListPageSelector.tableColumnHeaders], ['item-create']);        
  });
})
