pipeline {
    agent any

    stages {
        stage('Test') {
            steps {
                sh 'make test' 
            }
        }
        stage('Build') {
            steps {
                withEnv(['MYTOOL_HOME=/usr/local/mytool']) {
                    sh 'make build'
                    sh 'GO111MODULE=on CGO_ENABLED=0 go build -o grafeas-server go/v1beta1/main/main.go'
                }
            }
        }
    }
    
    post {
        success {
            archiveArtifacts artifacts: 'grafeas-server', fingerprint: true
            grafeas address: 'http://127.0.0.1:8090'
        }
    }
}
