import { OCCli, OCCreds } from "./cluster-cliops"
import { nwpolicyPageSelectors } from "./nwpolicy-page"

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

    podslist(project: string, label: string = "''") {
        const sed_cmd = "s/pod\\///g"
        const cmd = `oc get pods -o name -n ${project} -l ${label} | sed '${sed_cmd}'`
        cy.exec(cmd).then(result => {
            expect(result.stderr).to.be.empty
            cy.get(nwpolicyPageSelectors.podsList).each(($el, $index) => {
                //individual Pod
                cy.wrap($el).find(nwpolicyPageSelectors.treeNode).then($node => {
                    expect(result.stdout).to.contain($node.text())
                })
            })
        })
    }
}
