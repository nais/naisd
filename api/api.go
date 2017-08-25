package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/imdario/mergo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/errors"
	"net/http"
)

type Api struct {
	Clientset        kubernetes.Interface
	FasitUrl         string
	ClusterSubdomain string
}

type NaisDeploymentRequest struct {
	Application  string
	Version      string
	Environment  string
	Zone         string
	AppConfigUrl string
	Username     string
	Password     string
	Namespace    string
}

type Probe struct {
	Path string
	Port string
}

type HealthCheck struct {
	Liveness  Probe
	Readiness Probe
}

type NaisAppConfig struct {
	Name           string
	Image          string
	Port           *Port
	Healthcheks    HealthCheck
	FasitResources FasitResources `yaml:"fasitResources"`
}

type Port struct {
	Name       string
	Port       int
	TargetPort int `yaml:"targetPort"`
	Protocol   string
}

type FasitResources struct {
	Used    []UsedResource
	Exposed []ExposedResource
}

type UsedResource struct {
	Alias        string
	ResourceType string `yaml:"resourceType"`
}

type ExposedResource struct {
	Alias        string
	ResourceType string `yaml:"resourceType"`
	Path         string
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

	mux.HandleFunc(pat.Get("/isalive"), api.isAlive)
	mux.HandleFunc(pat.Post("/deploy"), api.deploy)
	mux.Handle(pat.Get("/metrics"), promhttp.Handler())

	return mux
}

func (api Api) isAlive(w http.ResponseWriter, _ *http.Request) {
	requests.With(prometheus.Labels{"path": "isAlive"}).Inc()
	fmt.Fprint(w, "")
}

func fetchAppConfig(url string) (naisAppConfig NaisAppConfig, err error) {
	glog.Infof("Fetching manifest from URL %s\n", url)
	response, err := http.Get(url)
	if err != nil {
		glog.Errorf("Could not fetch %s", err)
		return NaisAppConfig{}, err
	}

	defer response.Body.Close()

	if response.StatusCode > 299 {
		return NaisAppConfig{}, fmt.Errorf("got http status code %d\n", response.StatusCode)
	}

	var defaultAppConfig = GetDefaultAppConfig()
	var appConfig NaisAppConfig

	if body, err := ioutil.ReadAll(response.Body); err != nil {
		return NaisAppConfig{}, err
	} else {
		if err := yaml.Unmarshal(body, &appConfig); err != nil {
			glog.Errorf("Could not unmarshal yaml %s", err)
			return NaisAppConfig{}, err
		}
		glog.Infof("Got manifest %s", appConfig)
	}

	emptyPort := Port{}

	if *appConfig.Port == emptyPort {
		fmt.Println("port is nil" , )
		defaultAppConfig.Port = nil
		appConfig.Port = nil
	}

	//fmt.Println(appConfig.Port)
	//fmt.Println(defaultAppConfig.Port)

	if err := mergo.Merge(&appConfig, defaultAppConfig); err != nil {
		glog.Errorf("Could not merge appconfig %s", err)
		return NaisAppConfig{}, err
	}

	return appConfig, nil
}

func (api Api) deploy(w http.ResponseWriter, r *http.Request) {

	requests.With(prometheus.Labels{"path": "deploy"}).Inc()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.Errorf("Could not read body %s", err)
		w.WriteHeader(400)
		w.Write([]byte("Could not read body "))
		return
	}

	var deploymentRequest NaisDeploymentRequest
	if err = json.Unmarshal(body, &deploymentRequest); err != nil {
		glog.Errorf("Could not parse body %s", err)
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

	appConfig, err := fetchAppConfig(appConfigUrl)

	if err != nil {
		glog.Errorf("Unable to fetch manifest: %s\n", err)
		w.WriteHeader(500)
		w.Write([]byte("Could not fetch manifest\n"))
		return
	}

	var resourceRequests []ResourceRequest
	for _, resource := range appConfig.FasitResources.Used {
		resourceRequests = append(resourceRequests, ResourceRequest{Alias: resource.Alias, ResourceType: resource.ResourceType})
	}

	fasit := FasitClient{api.FasitUrl, deploymentRequest.Username, deploymentRequest.Password}

	var resources []NaisResource

	if len(resourceRequests) > 0 {
		resources, err = fasit.GetResources(resourceRequests, deploymentRequest.Environment, deploymentRequest.Application, deploymentRequest.Zone)
	}

	if err != nil {
		glog.Errorf("Error getting fasit resources: %s", err)
		w.WriteHeader(500)
		w.Write([]byte("Error getting fasit resources: " + err.Error()))
		return
	}

	glog.Infof("Read app-config.yaml, looks like this:%s\n", appConfig)
	if err := api.createOrUpdateService(deploymentRequest, appConfig); err != nil {
		glog.Errorf("Failed creating or updating service: %s", err)
		w.WriteHeader(500)
		w.Write([]byte("createOrUpdateService failed with: " + err.Error()))
		return
	}

	if err := api.createOrUpdateDeployment(deploymentRequest, appConfig, resources); err != nil {
		glog.Errorf("failed create or update Deployment: %s", err)
		w.WriteHeader(500)
		w.Write([]byte("createOrUpdateDeployment failed with: " + err.Error()))
		return
	}

	if err := api.createOrUpdateSecret(deploymentRequest, appConfig, resources); err != nil {
		glog.Errorf("failed create or update Secret: %s", err)
		w.WriteHeader(500)
		w.Write([]byte("createOrUpdateSecret failed with: " + err.Error()))
		return
	}

	if err := api.createOrUpdateIngress(deploymentRequest, appConfig); err != nil {
		glog.Errorf("failed create or update Ingress: %s", err)
		w.WriteHeader(500)
		w.Write([]byte("createOrUpdateIngress failed with: " + err.Error()))
		return
	}

	deploys.With(prometheus.Labels{"nais_app": deploymentRequest.Application}).Inc()

	w.WriteHeader(200)
	w.Write([]byte("ok\n"))
}

func (api Api) createOrUpdateService(req NaisDeploymentRequest, appConfig NaisAppConfig) error {

	serviceClient := api.Clientset.CoreV1().Services(req.Namespace)
	existingService, err := serviceClient.Get(req.Application)

	resourceCreator := K8sResourceCreator{appConfig, req}
	switch {
	case err == nil:
		updatedService, err := serviceClient.Update(resourceCreator.UpdateService(*existingService))
		if err != nil {
			return fmt.Errorf("failed to update service: %s", err)
		}
		glog.Infof("serviceClient updated: %s", updatedService)
	case errors.IsNotFound(err):
		newService, err := serviceClient.Create(K8sResourceCreator{AppConfig: appConfig, DeploymentRequest: req}.CreateService())
		if err != nil {
			return fmt.Errorf("failed to create service: %s", err)
		}
		glog.Infof("service created %s", newService)

	default:
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}

func (api Api) createOrUpdateDeployment(req NaisDeploymentRequest, appConfig NaisAppConfig, resource []NaisResource) error {

	deploymentClient := api.Clientset.ExtensionsV1beta1().Deployments(req.Namespace)
	deployment, err := deploymentClient.Get(req.Application)

	resourceCreator := K8sResourceCreator{appConfig, req}
	switch {
	case err == nil:
		updatedDeployment, err := deploymentClient.Update(resourceCreator.UpdateDeployment(deployment, resource))
		if err != nil {
			return fmt.Errorf("failed to update deployment: %s", err)
		}
		glog.Infof("deployment updated %s", updatedDeployment)
	case errors.IsNotFound(err):
		newDeployment, err := deploymentClient.Create(resourceCreator.CreateDeployment(resource))
		if err != nil {
			return fmt.Errorf("could not create deployment %s", err)
		}
		glog.Infof("deployment created %s", newDeployment)
	default:
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}

func (api Api) createOrUpdateSecret(req NaisDeploymentRequest, appConfig NaisAppConfig, resource []NaisResource) error {

	secretClient := api.Clientset.CoreV1().Secrets(req.Namespace)
	existingSecret, err := secretClient.Get(req.Application)
	resourceCreator := K8sResourceCreator{appConfig, req}
	switch {
	case err == nil:
		updatedSecret, err := secretClient.Update(resourceCreator.updateSecret(existingSecret, resource))
		if err != nil {
			return fmt.Errorf("failed to update secret: %s", updatedSecret)
		}
		glog.Infof("secretClient updated %s", updatedSecret)
	case errors.IsNotFound(err):
		newSecret, err := secretClient.Create(resourceCreator.CreateSecret(resource))
		if err != nil {
			glog.Infof("No secrets associated with deployment")
		}
		glog.Infof("secretClient created %s", newSecret)
	default:
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}

func (api Api) createOrUpdateIngress(req NaisDeploymentRequest, appConfig NaisAppConfig) error {

	ingressClient := api.Clientset.ExtensionsV1beta1().Ingresses(req.Namespace)
	existingIngress, err := ingressClient.Get(req.Application)
	resourceCreator := K8sResourceCreator{appConfig, req}
	switch {
	case err == nil:
		updatedIngress, err := ingressClient.Update(resourceCreator.updateIngress(existingIngress, api.ClusterSubdomain))
		if err != nil {
			return fmt.Errorf("failed to update ingress: %s", updatedIngress)
		}
		glog.Infof("ingressClient updated %s", updatedIngress)
	case errors.IsNotFound(err):
		newIngress, err := ingressClient.Create(resourceCreator.CreateIngress(api.ClusterSubdomain))
		if err != nil {
			return fmt.Errorf("failed to create ingress: %s", newIngress)
		}
		glog.Infof("ingressClient created %s", newIngress)
	default:
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}
