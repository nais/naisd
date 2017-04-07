package main

import (
	"k8s.io/client-go/rest"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"flag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
	"net/http"
	"io/ioutil"
)

func main() {
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

	pods, err := clientset.CoreV1().Pods("").List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	for _, pod := range pods.Items {
		fmt.Println(pod.Name)
	}

	goji.Post("/deploy", deploy)
	goji.Serve()
}

func deploy(c web.C, w http.ResponseWriter, r *http.Request) {
	fmt.Println("Hello there!")
	body, err := ioutil.ReadAll(r.Body)

	if err != nil{
		panic(err)
	}

	fmt.Println(string(body))
}

// returns config using kubeconfig if provided, else from cluster context
// useful for testing locally w/minikube
func getClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}