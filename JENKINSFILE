node{
  try{
    stage("test"){
        sh("make test")
    }

    stage("release_docker "){
        sh("make dockerhub-release")
    }
  }catch(e){
    currentBuild.result = "FAILED"
    throw e
  }
}