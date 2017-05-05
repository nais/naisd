package api

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
	"net/http"
)

type Api struct {
	Clientset kubernetes.Clientset
}

var (
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "requests", Help:"requests pr path"}, []string{"path"},
	)
	deploys = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "deployment", Help:"deployments done by NaisD"}, []string{"app"},
	)
)

func init() {
	prometheus.MustRegister(requests)
	prometheus.MustRegister(deploys)
}

func (api Api) NewApi() http.Handler {
	mux := goji.NewMux()

	mux.HandleFunc(pat.Get("/pods"), api.listPods)
	mux.HandleFunc(pat.Get("/hello"), api.hello)
	mux.HandleFunc(pat.Post("/deploy"), api.deploy)
	mux.Handle(pat.Get("/metrics"), promhttp.Handler())

	return mux
}

func (api Api) listPods(w http.ResponseWriter, _ *http.Request) {

	requests.With(prometheus.Labels{"path": "pods"}).Inc()
	pods, err := api.Clientset.CoreV1().Pods("").List(v1.ListOptions{})
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

func (api Api) hello(w http.ResponseWriter, _ *http.Request) {
	requests.With(prometheus.Labels{"path": "hello"}).Inc()

	fmt.Fprint(w, "banan")
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
	Name       string
	TargetPort int
	Port       int
	Protocol   string
}

type Container struct {
	Name  string
	Image string
	Ports []Port
}

func (api Api) deploy(w http.ResponseWriter, r *http.Request) {
	requests.With(prometheus.Labels{"path": "deploy"}).Inc()

	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		panic(err)
	}

	var deploymentRequest DeploymentRequest

	if err = json.Unmarshal(body, &deploymentRequest); err != nil {
		panic(err)
	}

	fmt.Printf("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.Environment)

	body, err = ioutil.ReadFile("./app-config.yaml")
	if err != nil {
		panic(err)
	}

	var appConfig AppConfig

	yaml.Unmarshal(body, &appConfig)

	output, _ := yaml.Marshal(appConfig)
	fmt.Printf("Read app-config.yaml, looks like this:\n%s", string(output))

	w.Write([]byte("ok\n"))

	if err := api.createOrUpdateService(deploymentRequest, appConfig); err != nil {
		fmt.Println(err)
	}

	if err := api.createOrUpdateDeployment(deploymentRequest, appConfig); err != nil {
		fmt.Println(err)
	}

	if err := api.createOrUpdateIngress(deploymentRequest, appConfig); err != nil {
		fmt.Println(err)
	}

	deploys.With(prometheus.Labels{"app":deploymentRequest.Application}).Inc()
}

func (api Api) createOrUpdateService(req DeploymentRequest, appConfig AppConfig) error {
	appName := req.Application

	serviceSpec := ResourceCreator{appConfig, req}.CreateService()

	service := api.Clientset.Core().Services("default")

	svc, err := service.Get(appName)

	switch {
	case err == nil:
		serviceSpec.ObjectMeta.ResourceVersion = svc.ObjectMeta.ResourceVersion
		serviceSpec.Spec.ClusterIP = svc.Spec.ClusterIP
		_, err = service.Update(serviceSpec)
		if err != nil {
			return fmt.Errorf("failed to update service: %s", err)
		}
		fmt.Println("service updated")
	case errors.IsNotFound(err):
		_, err = service.Create(serviceSpec)
		if err != nil {
			return fmt.Errorf("failed to create service: %s", err)
		}
		fmt.Println("service created")
	default:
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}

func (api Api) createOrUpdateDeployment(req DeploymentRequest, appConfig AppConfig) error {
	deploymentSpec := ResourceCreator{appConfig, req}.CreateDeployment()

	// Implement deployment update-or-create semantics.
	deploy := api.Clientset.Extensions().Deployments("default")
	_, err := deploy.Update(deploymentSpec)
	switch {
	case err == nil:
		fmt.Println("deployment controller updated")
	case !errors.IsNotFound(err):
		return fmt.Errorf("could not update deployment controller: %s", err)
	default:
		_, err = deploy.Create(deploymentSpec)
		if err != nil {
			return fmt.Errorf("could not create deployment controller: %s", err)
		}
		fmt.Println("deployment controller created")
	}

	return nil
}

func (api Api) createOrUpdateIngress(req DeploymentRequest, appConfig AppConfig) error {
	ingressSpec := ResourceCreator{appConfig, req}.CreateIngress()

	// Implement deployment update-or-create semantics.
	ingress := api.Clientset.Extensions().Ingresses("default")
	_, err := ingress.Update(ingressSpec)
	switch {
	case err == nil:
		fmt.Println("ingress updated")
	case !errors.IsNotFound(err):
		return fmt.Errorf("could not update ingress: %s", err)
	default:
		_, err = ingress.Create(ingressSpec)
		if err != nil {
			return fmt.Errorf("could not create ingress: %s", err)
		}
		fmt.Println("ingress created")
	}

	return nil
}
