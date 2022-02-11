
export interface OCCreds {
    idp: string;
    user: string;
    password: string;
}

export class OCCli {
    creds: OCCreds
    loggedin;
    networkprovider: string

    constructor(creds: OCCreds) {
        this.creds = creds
        this.loggedin = this.login()
    }

    login(): void {
        cy.exec(`oc login -u ${this.creds.user} -p ${this.creds.password}`).then(result => {
            expect(result.stderr).to.be.empty
        })
    }

    createPods(specFilePath, project): void {
        cy.exec(`oc create -f ${specFilePath} -n ${project}`);
    }

    switchProject(project: string): void {
        cy.exec(`oc project ${project}`).then(result => {
            expect(result.stderr).to.be.empty
        })
    }

    runPodCmd(project: string, podName: string, cmd: string, exOut: string, exResult: boolean = true) {
        cy.exec(`oc rsh -n ${project} ${podName} ${cmd}`, { failOnNonZeroExit: exResult }).then(result => {
            if (exResult) {
                this.matchOutput(result.stdout, exOut)
            }
            else {
                this.matchOutput(result.stderr, exOut)
            }
        })
    }

    private matchOutput(text: string, match: string = null) {
        if (match) {
            cy.wrap(text).then(text => {
                expect(text).to.contain(match)
            })
        }
        else {
            cy.wrap(text).then(text => {
                expect(text).to.be.not.empty
            })
        }
    }
}
