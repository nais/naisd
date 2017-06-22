package main

import (
	"k8s.io/client-go/rest"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"flag"
	"k8s.io/client-go/kubernetes"
	"net/http"

	"github.com/navikt/naisd/api"
)

const Port string = ":8081"

func main() {
	fmt.Printf("serving @ %s\n", Port)
	http.ListenAndServe(Port, api.Api{newClientSet()}.NewApi())
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
