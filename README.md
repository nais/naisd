#  naisd

k8s in-cluster daemon with API for performing NAIS-operations

Basic outline

1. HTTP POST to API with name of application, version and environment
2. Fetches app-config from internal artifact repository
3. Extract info from yaml
4. Get and inject environment specific variables from Fasit
5. Creates appropriate k8s resources

## dev notes

For local development, use minikube. You can run naisd.go with -kubeconfig=<path to kube config> for testing without deploying to cluster. 

```glide install```

...to fetch dependecies

To reduce build time, do

```go build -i .```

initially. 

