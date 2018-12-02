pipeline {
    agent { label 'upbound-gce' }

    options {
        disableConcurrentBuilds()
        timestamps()
    }

    environment {
        DOCKER = credentials('dockerhub-upboundci')
        AWS = credentials('aws-upbound-bot')
        GITHUB_UPBOUND_BOT = credentials('github-upbound-jenkins')
    }

    stages {

        stage('Prepare') {
            steps {
                script {
                    def pr_number = sh (script: "echo ${env.BRANCH_NAME} | grep -o -E '[0-9]+' ",returnStdout: true)
                    def json = sh (script: "curl -s https://api.github.com/repos/crossplaneio/crossplane/pulls/${pr_number}", returnStdout: true).trim()
                    def body = evaluateJson(json,'${json.body}')
                    if (body.contains("[skip ci]")) {
                        echo ("'[skip ci]' spotted in PR body text.")
                        env.shouldBuild = "false"
                    }
                }
                sh 'git config --global user.name "upbound-bot"'
                sh 'echo "machine github.com login upbound-bot password $GITHUB_UPBOUND_BOT" > ~/.netrc'
            }
        }

        stage('Build validation'){
            when {
                expression {
                    return env.shouldBuild != "false"
                }
            }
            steps {
                sh './build/run make vendor.check'
                sh './build/run make -j\$(nproc) build.all'
            }
        }

        stage('Unit Tests') {
            when {
                expression {
                    return env.shouldBuild != "false"
                }
            }
            steps {
                sh './build/run make -j\$(nproc) test'
            }
            post {
                always {
                    archiveArtifacts "_output/tests/**/*"
                    junit "_output/tests/**/*.xml"
                }
            }
        }

        stage('Publish') {
            when {
                expression {
                    return env.shouldBuild != "false"
                }
            }
            steps {
                sh 'docker login -u="${DOCKER_USR}" -p="${DOCKER_PSW}"'
                sh "./build/run make -j\$(nproc) publish BRANCH_NAME=${BRANCH_NAME} AWS_ACCESS_KEY_ID=${AWS_USR} AWS_SECRET_ACCESS_KEY=${AWS_PSW} GIT_API_TOKEN=${GITHUB_UPBOUND_BOT}"
                script {
                    if (BRANCH_NAME == 'master') {
                        lock('promote-job') {
                            sh "./build/run make -j\$(nproc) promote BRANCH_NAME=master CHANNEL=master AWS_ACCESS_KEY_ID=${AWS_USR} AWS_SECRET_ACCESS_KEY=${AWS_PSW}"
                        }
                    }
                }
            }
        }
    }

    post {
        always {
            script {
                sh 'make -j\$(nproc) clean'
                sh 'make -j\$(nproc) prune PRUNE_HOURS=48 PRUNE_KEEP=48'
                sh 'docker images'
            }
        }
    }
}

@NonCPS
def evaluateJson(String json, String gpath){
    //parse json
    def ojson = new groovy.json.JsonSlurper().parseText(json)
    //evaluate gpath as a gstring template where $json is a parsed json parameter
    return new groovy.text.GStringTemplateEngine().createTemplate(gpath).make(json:ojson).toString()
}
