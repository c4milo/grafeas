pipeline {
    agent any

    stages {
        stage('Build') {
            steps {
                sh 'make build'
            }
        }
        stage('Test') {
            steps {
                sh 'make test'
            }
        }
        stage('Release') {
            steps {
                sh 'GO111MODULE=on CGO_ENABLED=0 go build -o grafeas-server go/v1beta1/main/main.go'
            }
        }
    }

    post {
        success {
            archiveArtifacts artifacts: 'grafeas-server', fingerprint: true
            grafeas address: 'http://127.0.0.1:8090', projectId: 'grafeas'
        }
    }
}
