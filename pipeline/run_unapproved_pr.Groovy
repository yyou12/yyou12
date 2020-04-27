prdb_output_file = ""
prs_content = ""
jobs_success = [:]
jobs_fail = [:]

pipeline {
    agent any

    stages {
        stage('generate unapproved pr file') {
            steps {
                sh 'cp -fr /root/prdb/generateoutputfilename.sh .'
                script {
                    // sh(returnStdout: false, script: 'cp -fr /root/prdb/generateoutputfilename.sh .')
                    return_output = sh(returnStdout: true, script: './generateoutputfilename.sh').trim()
                    prdb_output_file = return_output
                    sh(returnStdout: false, script: 'cp -fr /root/prdb/prdbhandler.py .')
                    sh(returnStdout: false, script: "python3 prdbhandler.py -a get -o ${prdb_output_file}")
                }
            }
        }
        stage('get unapprove pr list') {
            steps {
                script {
                    prinfos = readFile "${prdb_output_file}"
                    prs_content = prinfos
                    sh(returnStdout: false, script: "rm -fr ${prdb_output_file}")
                    }
                }
        }
        stage('execute unapprove prs') {
            steps {
                script {
                    prs = prs_content.split("\n")
                    Boolean failedJobs = false
                    for( String pr : prs ) {
                        def probj = readJSON text: pr
                        job_build = build job:'olm-test' , parameters:[
                            string(name: 'REPO',value: probj.repo),
                            string(name: 'BRANCH',value: probj.branch),
                            string(name: 'SCENARIO',value: probj.scenario),
                            string(name: 'KUBECONFIG_FILE',value: KUBECONFIG_FILE),
                            string(name: 'AZURE_AUTH_LOCATION_FILE',value: AZURE_AUTH_LOCATION_FILE)
                            ], wait: true, propagate: false
                        result = job_build.getResult()
                        buildurl = job_build.getAbsoluteUrl()
                        if (result =="SUCCESS" ) {
                            jobs_success[probj.prurl] = buildurl
                        } else {
                            jobs_fail[probj.prurl] = buildurl
                            failedJobs = true
                        }
                    }
                    if (failedJobs) {
                        error("One or more PRs have failed")
                    } 
                }
            }
        }
    }
    post {
        always {
            echo "PR success results: ${jobs_success.toString()}"
            echo "PR fail results: ${jobs_fail.toString()}"
        }
        success {
            echo "All PRs completed OK"
        }
        failure {
            echo "failure"
            script {
                jobs_fail.each{ k, v -> slackSend (channel: '#olm-standup',
                    color: 'danger',
                    message: "The build of unapproved PR ${k} failed. and the URL is ${v}")
                }
            }
        }
        unstable {
            echo "unstable"
        }
        
    }
}

