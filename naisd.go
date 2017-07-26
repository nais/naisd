package main

import (
	"flag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"

	"github.com/nais/naisd/api"
	"github.com/golang/glog"
)

const Port string = ":8081"

func main() {

	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	fasitUrl := flag.String("fasiturl", "https://fasit.adeo.no", "URL to fasit instance")

	flag.Parse()

	glog.Infof("using fasit instance %s", *fasitUrl)

	glog.Infof("running on port %s", Port)
	err := http.ListenAndServe(Port, api.Api{newClientSet(*kubeconfig), *fasitUrl}.NewApi())
	if err != nil {
		panic(err)
	}
}

// returns config using kubeconfig if provided, else from cluster context
func newClientSet(kubeconfig string) kubernetes.Interface {

	var config *rest.Config
	var err error

	if kubeconfig != "" {
		glog.Infof("using provided kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		glog.Infof("no kubeconfig provided, assuming we are running inside a cluster")
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