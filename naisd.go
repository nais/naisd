package main

import (
	"flag"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"

	"github.com/nais/naisd/api"
)

const Port string = ":8081"

func main() {
	fmt.Printf("serving @ %s\n", Port)
	http.ListenAndServe(Port, api.Api{newClientSet(), fasit()}.NewApi())
}

// returns config using kubeconfig if provided, else from cluster context
func newClientSet() kubernetes.Interface {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	flag.Parse()

	var config *rest.Config
	var err error

	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func fasit() api.Fasit{
	return api.FasitAdapter{"www"}
}
