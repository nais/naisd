package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
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
	Clientset kubernetes.Interface
}

var (
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "requests", Help: "requests pr path"}, []string{"path"},
	)
	deploys = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "deployments", Help: "deployments done by NaisD"}, []string{"nais_app"},
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
	mux.HandleFunc(pat.Get("/test"), api.testing)
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
		glog.Info(pod.Name)
	}

	output, _ := json.MarshalIndent(pods.Items, "", "    ")

	fmt.Fprint(w, string(output))
}

func (api Api) hello(w http.ResponseWriter, _ *http.Request) {
	requests.With(prometheus.Labels{"path": "hello"}).Inc()

	fmt.Fprint(w, "banan")
}

type DeploymentRequest struct {
	Application  string
	Version      string
	Environment  string
	AppConfigUrl string
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

func (api Api) testing(w http.ResponseWriter, r *http.Request) {
	appConfig, _ := fetchManifest("http://localhost:8080/app-config.yaml")
	fmt.Println(appConfig.Containers[0].Name)
}

func fetchManifest(url string) (AppConfig, error) {

	glog.Info("Fetching manifest from URL %s\n", url)

	if response, err := http.Get(url); err != nil {
		return AppConfig{}, err
	} else if response.StatusCode > 299 {
		return AppConfig{}, fmt.Errorf("could fetching manifests gave status " + string(response.StatusCode))
	} else {
		defer response.Body.Close()
		var appConfig AppConfig
		if body, err := ioutil.ReadAll(response.Body); err != nil {
			return AppConfig{}, err
		} else {
			yaml.Unmarshal(body, &appConfig)
			return appConfig, nil
		}
	}
}

func (api Api) deploy(w http.ResponseWriter, r *http.Request) {
	requests.With(prometheus.Labels{"path": "deploy"}).Inc()

	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		glog.Error("Could not read body %s", err)
		w.WriteHeader(400)
		w.Write([]byte("Could not read body "))
		return
	}

	var deploymentRequest DeploymentRequest

	if err = json.Unmarshal(body, &deploymentRequest); err != nil {
		glog.Error("Could not parse body %s", err)
		w.WriteHeader(400)
		w.Write([]byte("Could not parse body "))
		return
	}

	glog.Infof("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.Environment)

	var appConfigUrl string

	if deploymentRequest.AppConfigUrl != "" {
		appConfigUrl = deploymentRequest.AppConfigUrl
	} else {
		appConfigUrl = fmt.Sprintf("http://nexus.adeo.no/nexus/service/local/repositories/m2internal/content/nais/%s/%s/%s", deploymentRequest.Application, deploymentRequest.Version, fmt.Sprintf("%s-%s.yaml", deploymentRequest.Application, deploymentRequest.Version))
	}

	appConfig, err := fetchManifest(appConfigUrl)
	if err != nil {
		fmt.Printf("Unable to fetch manifest %s", err)
		w.WriteHeader(500)
		w.Write([]byte("Could not fetch manifest"))
		return
	}

	glog.Infof("Read app-config.yaml, looks like this:%s\n", appConfig)

	if err := api.createOrUpdateService(deploymentRequest, appConfig); err != nil {
		glog.Error("Failed creating or updring serivce", err)
		w.WriteHeader(500)
		w.Write([]byte("CreateUpdate of Service failed with errror: " + err.Error()))
		return
	}

	if err := api.createOrUpdateDeployment(deploymentRequest, appConfig); err != nil {
		glog.Error("failed create or update Deployment", err)
		w.WriteHeader(500)
		w.Write([]byte("createOrUpdateDeployment failed with: " + err.Error()))
		return
	}

	if err := api.createOrUpdateIngress(deploymentRequest, appConfig); err != nil {
		glog.Error("failed create or update Ingress", err)
		w.WriteHeader(500)
		w.Write([]byte("createOrUpdateIngress failed with: " + err.Error()))
		return
	}

	deploys.With(prometheus.Labels{"nais_app": deploymentRequest.Application}).Inc()

	w.WriteHeader(200)
	w.Write([]byte("ok\n"))
}

func (api Api) createOrUpdateService(req DeploymentRequest, appConfig AppConfig) error {

	serviceClient := api.Clientset.CoreV1().Services(req.Environment)
	svc, err := serviceClient.Get(req.Application)

	switch {
	case err == nil:
		newService, err := serviceClient.Update(ResourceCreator{appConfig, req}.UpdateService(*svc))
		if err != nil {
			return fmt.Errorf("failed to update service: %s", err)
		}
		glog.Info("serviceClient updated: %s", newService)
	case errors.IsNotFound(err):
		newService, err := serviceClient.Create(ResourceCreator{AppConfig: appConfig, DeploymentRequest: req}.CreateService())
		if err != nil {
			return fmt.Errorf("failed to create service: %s", err)
		}
		glog.Info("service created %s", newService)

	default:
		return fmt.Errorf("unexpected error: %s", err)
	}
	return nil
}

func (api Api) createOrUpdateDeployment(req DeploymentRequest, appConfig AppConfig) error {

	deploymentClient := api.Clientset.ExtensionsV1beta1().Deployments(req.Environment)
	deployment, err := deploymentClient.Get(req.Application)

	switch {
	case err == nil:
		updatedDeployment, err := deploymentClient.Update(ResourceCreator{appConfig, req}.UpdateDeployment(deployment))
		if err != nil {
			return fmt.Errorf("failed to update deployment", err)
		}
		glog.Info("deployment updated %s", updatedDeployment)
	case !errors.IsNotFound(err):
		newDeployment, err := deploymentClient.Create(ResourceCreator{appConfig, req}.CreateDeployment())
		if err != nil {
			return fmt.Errorf("could not create deployment %s", err)
		}
		glog.Info("deployment created %s", newDeployment)
	default:
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}

func (api Api) createOrUpdateIngress(req DeploymentRequest, appConfig AppConfig) error {

	ingressClient := api.Clientset.Extensions().Ingresses(req.Environment)
	existingIngress, err := ingressClient.Get(req.Application)
	switch {
	case err == nil:
		updatedIngress, err := ingressClient.Update(ResourceCreator{appConfig, req}.updateIngress(existingIngress))
		if err != nil {
			return fmt.Errorf("failed to update ingress", updatedIngress)
		}
		glog.Info("ingressClient updated %s", updatedIngress)
	case !errors.IsNotFound(err):
		newIngress, err := ingressClient.Create(ResourceCreator{appConfig, req}.CreateIngress())
		if err != nil {
			return fmt.Errorf("failed to create ingress", newIngress)
		}
		glog.Info("ingressClient created %s", newIngress)
	default:
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}
