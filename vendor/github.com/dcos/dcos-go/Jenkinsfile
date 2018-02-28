node('mesos') {
  stage('Run tests') {
    dir('dcos-go') {
      checkout scm

      sh 'docker build -t dcos/dcos-go .'

      sh '''
         docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
           -v $(which docker):/usr/bin/docker -v "$PWD":/go/src/github.com/dcos/dcos-go \
           dcos/dcos-go make test'''
    }
  }
}
