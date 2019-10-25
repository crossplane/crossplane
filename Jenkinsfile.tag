pipeline {
    agent { label 'upbound-gce' }

    parameters {
        string(name: 'version', defaultValue: '', description: 'The version you are releasing, for example v0.4.0')
        string(name: 'commit', defaultValue: '', description: 'Optional commit hash for this release, for example 56b65dba917e50132b0a540ae6ff4c5bbfda2db6. If empty the latest commit hash will be used.')
    }

    options {
        disableConcurrentBuilds()
        timestamps()
    }

    environment {
        GITHUB_UPBOUND_BOT = credentials('github-upbound-jenkins')
    }

    stages {

        stage('Prepare') {
            steps {
                 // github credentials are not setup to push over https in jenkins. add the github token to the url
                sh "git config remote.origin.url https://${GITHUB_UPBOUND_BOT_USR}:${GITHUB_UPBOUND_BOT_PSW}@\$(git config --get remote.origin.url | sed -e 's/https:\\/\\///')"
                sh 'git config user.name "upbound-bot"'
                sh 'git config user.email "info@crossplane.io"'
                sh 'echo "machine github.com login upbound-bot password $GITHUB_UPBOUND_BOT" > ~/.netrc'
            }
        }

        stage('Tag Release') {
            steps {
                sh "./build/run make -j\$(nproc) tag VERSION=${params.version} COMMIT_HASH=${params.commit}"
            }
        }
    }

    post {
        always {
            deleteDir()
        }
    }
}
