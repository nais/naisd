package main

import (
	"k8s.io/client-go/rest"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"flag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"net/http"
	"goji.io"
	"goji.io/pat"
	"io/ioutil"
	"encoding/json"

	"gopkg.in/yaml.v2"
)

var clientset = initKubeConfiguration()

func initKubeConfiguration() *kubernetes.Clientset {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	flag.Parse()

	config, err := getClientConfig(*kubeconfig)

	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func main() {
	mux := goji.NewMux()
	mux.HandleFunc(pat.Get("/pods"), listPods)
	mux.HandleFunc(pat.Post("/deploy"), deploy)

	serveLocation := "localhost:6969"

	fmt.Printf("serving @ %s\n", serveLocation)
	http.ListenAndServe(serveLocation, mux)
}

func listPods(w http.ResponseWriter, _ *http.Request) {
	pods, err := clientset.CoreV1().Pods("").List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	for _, pod := range pods.Items {
		fmt.Println(pod.Name)
	}

	output, _ := json.MarshalIndent(pods.Items, "", "    ")

	fmt.Fprint(w, string(output))
}

type DeploymentRequest struct {
	Application string
	Version     string
	Environment string
}

type AppConfig struct {
	Containers []Container
}

type Port struct {
	Name string
	TargetPort int
	Port int
	Protocol string
}

type Container struct {
	Name  string
	Image string
	Ports []Port
}

func deploy(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		panic(err)
	}

	var deploymentRequest DeploymentRequest

	if err = json.Unmarshal(body, &deploymentRequest); err != nil {
		panic(err)
	}

	fmt.Printf("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.Environment)

	body, err = ioutil.ReadFile("./app-config.yaml");

	if err != nil {
		panic(err)
	}

	var appConfig AppConfig

	yaml.Unmarshal(body, &appConfig)

	output,_ := yaml.Marshal(appConfig)
	fmt.Printf("Read app-config.yaml, looks like this:\n%s", string(output))

	w.Write([]byte("ok\n"))
}

// returns config using kubeconfig if provided, else from cluster context
// useful for testing locally w/minikube
func getClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}