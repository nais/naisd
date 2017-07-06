node{
  try{

    stage("checkout") {
        git url: "https://github.com/nais/naisd.git"
    }

    stage("install"){
     sh("make install")
    }

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