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
                sh 'make build'
                sh 'GO111MODULE=on CGO_ENABLED=0 go build -o grafeas-server go/v1beta1/main/main.go'
            }
        }
        stage('Release') {
            steps {
                archiveArtifacts artifacts: 'grafeas-server', fingerprint: true
            }
        }
    }
}
