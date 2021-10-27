import { OCCli, OCCreds } from "./cluster-cliops"
import { nwpolicyPageSelectors } from "./nwpolicy-page"

export namespace helperfuncs {
    export function setNetworkProviderAlias() {
        cy.exec("oc get Network.config.openshift.io cluster -o json").then(result => {
            expect(result.stdout).to.be.not.empty
            const networkprovider = JSON.parse(result.stdout)
            cy.wrap(networkprovider.spec.networkType).as('networkprovider')
        })
    }
}

export class VerifyPolicyForm {
    creds: OCCreds
    oc: OCCli

    constructor(oc?: OCCli, creds?: OCCreds) {
        this.creds = creds || undefined
        if (creds) {
            this.oc = new OCCli(creds)
        }
        else if (!oc) {
            throw 'must pass creds: OCCreds property or oc: OCli'
        }
        else {
            this.oc = oc

        }
    }

    connection(project: string, fromPodName: string, toPodIP: string, match: string, expectedResult: boolean = true, port = "8080") {
        let cmd = `curl ${toPodIP}:${port} -m 5`
        this.oc.runPodCmd(project, fromPodName, cmd, match, expectedResult)
    }

    podslist(project, label) {
        let sed_cmd = "s/pod\\///g"
        cy.exec(`oc get pods -o name -n ${project} -l ${label} | sed '${sed_cmd}'`).then(result => {
            cy.log(result.stdout)
            cy.get(nwpolicyPageSelectors.podsList).each(($el, $index) => {
                //individual Pod
                cy.wrap($el).find(nwpolicyPageSelectors.treeNode).then($node => {
                    expect(result.stdout).to.contain($node.text())
                })
            })
        })
    }
}
