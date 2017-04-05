# NAIS API

k8s in-cluster API for performing NAIS-deploys.

Basic outline

1. HTTP POST to API with name of application, version and environment
2. Fetches app-config from internal artifact repository
3. Extract info from yaml
4. Creates appropriate k8s resources 


## dev notes

Uses [dep](https://github.com/golang/dep) for managing dependencies.

For local development, use minikube. You can run api.go with -kubeconfig=<path to kube config> for testing without deploying to cluster. 

```dep ensure```

...to fetch dependecies



To reduce build time, do

```go build -i api.go```

initially.

