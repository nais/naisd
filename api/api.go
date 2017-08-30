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
	autoscalingv1 "k8s.io/client-go/pkg/apis/autoscaling/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/api/v1"
	"io"
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
}

type Healthcheck struct {
	Liveness  Probe
	Readiness Probe
}

type NaisAppConfig struct {
	Name           string
	Image          string
	Port           Port
	Healthcheck    Healthcheck
	Replicas       Replicas
	FasitResources FasitResources `yaml:"fasitResources"`
}

type Replicas struct {
	Min                    int
	Max                    int
	CpuThresholdPercentage int `yaml:"cpuThresholdPercentage"`
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

func (api Api) deploy(w http.ResponseWriter, r *http.Request) {

	requests.With(prometheus.Labels{"path": "deploy"}).Inc()

	deploymentRequest, err := unmarshalDeploymentRequest(r.Body)

	if err != nil {
		glog.Errorf("Unable to unmarshal deployment request %s", err)
		w.WriteHeader(400)
		w.Write([]byte("Unable to understand deployment request: " + err.Error()))
		return
	}

	glog.Infof("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.Environment)

	appConfigUrl := createAppConfigUrl(deploymentRequest.AppConfigUrl, deploymentRequest.Application, deploymentRequest.Version)
	appConfig, err := fetchAppConfig(appConfigUrl)

	if err != nil {
		glog.Errorf("Unable to fetch manifest: %s\n", err)
		w.WriteHeader(500)
		w.Write([]byte("Could not fetch manifest\n"))
		return
	}

	naisResources, err := fetchFasitResources(api.FasitUrl, deploymentRequest, appConfig)

	if err != nil {
		glog.Errorf("Error getting fasit resources: %s", err)
		w.WriteHeader(500)
		w.Write([]byte("Error getting fasit resources: " + err.Error()))
		return
	}

	deploymentResult, err := api.createOrUpdateK8sResources(deploymentRequest, appConfig, naisResources)

	if err != nil {
		glog.Errorf("Failed while creating or updating resources: %s\n Created this %s", err, deploymentResult)
		w.WriteHeader(500)
		w.Write([]byte("Failed while creating or updating resources: " + err.Error()))
		return
	}

	deploys.With(prometheus.Labels{"nais_app": deploymentRequest.Application}).Inc()

	w.WriteHeader(200)
	w.Write([]byte("ok\n"))
}
func fetchFasitResources(fasitUrl string, deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig) ([]NaisResource, error) {
	var resourceRequests []ResourceRequest
	for _, resource := range appConfig.FasitResources.Used {
		resourceRequests = append(resourceRequests, ResourceRequest{Alias: resource.Alias, ResourceType: resource.ResourceType})
	}

	fasit := FasitClient{fasitUrl, deploymentRequest.Username, deploymentRequest.Password}

	return fasit.GetResources(resourceRequests, deploymentRequest.Environment, deploymentRequest.Application, deploymentRequest.Zone)
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

	if err := mergo.Merge(&appConfig, defaultAppConfig); err != nil {
		glog.Errorf("Could not merge appconfig %s", err)
		return NaisAppConfig{}, err
	}

	return appConfig, nil
}

func unmarshalDeploymentRequest(body io.ReadCloser) (NaisDeploymentRequest, error) {
	requestBody, err := ioutil.ReadAll(body)
	if err != nil {
		return NaisDeploymentRequest{}, fmt.Errorf("Could not read deployment request body %s", err)
	}

	var deploymentRequest NaisDeploymentRequest
	if err = json.Unmarshal(requestBody, &deploymentRequest); err != nil {
		return NaisDeploymentRequest{}, fmt.Errorf("Could not parse body %s", err)
	}

	return deploymentRequest, nil
}

func createAppConfigUrl(appConfigUrl, application, version string) string {
	if appConfigUrl != "" {
		return appConfigUrl
	} else {
		return fmt.Sprintf("http://nexus.adeo.no/nexus/service/local/repositories/m2internal/content/nais/%s/%s/%s", application, version, fmt.Sprintf("%s-%s.yaml", application, version))
	}
}

type DeploymentResult struct {
	Autoscaler *autoscalingv1.HorizontalPodAutoscaler
	Ingress    *v1beta1.Ingress
	Deployment *v1beta1.Deployment
	Secret     *v1.Secret
	Service    *v1.Service
}

func (api Api) createOrUpdateK8sResources(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, resources []NaisResource) (DeploymentResult, error) {

	var deploymentResult DeploymentResult

	service, err := api.createOrUpdateService(deploymentRequest, appConfig)

	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating service: %s", err)
	}

	deploymentResult.Service = service

	deployment, err := api.createOrUpdateDeployment(deploymentRequest, appConfig, resources)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating deployment: %s", err)
	}
	deploymentResult.Deployment = deployment

	secret, err := api.createOrUpdateSecret(deploymentRequest, appConfig, resources)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating secret: %s", err)
	}
	deploymentResult.Secret = secret

	ingress, err := api.createOrUpdateIngress(deploymentRequest, api.ClusterSubdomain)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating ingress: %s", err)
	}
	deploymentResult.Ingress = ingress

	autoscaler, err := api.createOrUpdateAutoscaler(deploymentRequest, appConfig)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating autoscaler: %s", err)
	}
	deploymentResult.Autoscaler = autoscaler

	return deploymentResult, err
}

func (api Api) createOrUpdateAutoscaler(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	autoscalerId, err := api.getExistingAutoscalerId(deploymentRequest.Application, deploymentRequest.Namespace)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing autoscaler id: %s", err)
	}

	autoscalerDef := createAutoscalerDef(appConfig.Replicas.Min, appConfig.Replicas.Max, appConfig.Replicas.CpuThresholdPercentage, autoscalerId, deploymentRequest.Application, deploymentRequest.Namespace)
	return api.createOrUpdateAutoscalerResource(autoscalerDef, deploymentRequest.Namespace)
}

func (api Api) createOrUpdateIngress(deploymentRequest NaisDeploymentRequest, clusterSubdomain string) (*v1beta1.Ingress, error) {
	existingIngressId, err := api.getExistingIngressId(deploymentRequest.Application, deploymentRequest.Namespace)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing ingress id: %s", err)
	}

	ingressDef := createIngressDef(clusterSubdomain, existingIngressId, deploymentRequest.Application, deploymentRequest.Namespace)
	return api.createOrUpdateIngressResource(ingressDef, deploymentRequest.Namespace)
}

func (api Api) createOrUpdateService(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig) (*v1.Service, error) {
	existingServiceId, err := api.getExistingServiceId(deploymentRequest.Application, deploymentRequest.Namespace)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing service id: %s", err)
	}

	autoscalerDef := createServiceDef(appConfig.Port.TargetPort, existingServiceId, deploymentRequest.Application, deploymentRequest.Namespace)
	return api.createOrUpdateServiceResource(autoscalerDef, deploymentRequest.Namespace)
}

func (api Api) createOrUpdateDeployment(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, naisResources []NaisResource) (*v1beta1.Deployment, error) {
	existingDeploymentId, err := api.getExistingDeploymentId(deploymentRequest.Application, deploymentRequest.Namespace)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing deployment id: %s", err)
	}

	deploymentDef := createDeploymentDef(naisResources, appConfig.Image, deploymentRequest.Version, appConfig.Port.Port, appConfig.Healthcheck.Liveness.Path, appConfig.Healthcheck.Readiness.Path, existingDeploymentId, deploymentRequest.Application, deploymentRequest.Namespace)
	return api.createOrUpdateDeploymentResource(deploymentDef, deploymentRequest.Namespace)
}

func (api Api) createOrUpdateSecret(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, naisResources []NaisResource) (*v1.Secret, error) {
	existingSecretId, err := api.getExistingSecretId(deploymentRequest.Application, deploymentRequest.Namespace)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing autoscaler id: %s", err)
	}

	secretDef := createSecretDef(naisResources, existingSecretId, deploymentRequest.Application, deploymentRequest.Namespace)
	return api.createOrUpdateSecretResource(secretDef, deploymentRequest.Namespace)
}

func (api Api) getExistingServiceId(application string, namespace string) (string, error) {
	serviceClient := api.Clientset.CoreV1().Services(namespace)
	service, err := serviceClient.Get(application)

	switch {
	case err == nil:
		return service.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func (api Api) getExistingSecretId(application string, namespace string) (string, error) {
	secretClient := api.Clientset.CoreV1().Secrets(namespace)
	secret, err := secretClient.Get(application)

	switch {
	case err == nil:
		return secret.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func (api Api) getExistingDeploymentId(application string, namespace string) (string, error) {
	deploymentClient := api.Clientset.ExtensionsV1beta1().Deployments(namespace)
	deployment, err := deploymentClient.Get(application)

	switch {
	case err == nil:
		return deployment.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func (api Api) getExistingIngressId(application string, namespace string) (string, error) {
	ingressClient := api.Clientset.ExtensionsV1beta1().Ingresses(namespace)
	ingress, err := ingressClient.Get(application)

	switch {
	case err == nil:
		return ingress.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func (api Api) getExistingAutoscalerId(application string, namespace string) (string, error) {
	autoscalerClient := api.Clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace)
	autoscaler, err := autoscalerClient.Get(application)

	switch {
	case err == nil:
		return autoscaler.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func (api Api) createOrUpdateAutoscalerResource(autoscalerSpec *autoscalingv1.HorizontalPodAutoscaler, namespace string) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	if autoscalerSpec.ObjectMeta.ResourceVersion != "" {
		return api.Clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(autoscalerSpec)
	} else {
		return api.Clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(autoscalerSpec)
	}
}

func (api Api) createOrUpdateIngressResource(ingressSpec *v1beta1.Ingress, namespace string) (*v1beta1.Ingress, error) {
	if ingressSpec.ObjectMeta.ResourceVersion != "" {
		return api.Clientset.ExtensionsV1beta1().Ingresses(namespace).Update(ingressSpec)
	} else {
		return api.Clientset.ExtensionsV1beta1().Ingresses(namespace).Create(ingressSpec)
	}
}

func (api Api) createOrUpdateDeploymentResource(deploymentSpec *v1beta1.Deployment, namespace string) (*v1beta1.Deployment, error) {
	if deploymentSpec.ObjectMeta.ResourceVersion != "" {
		return api.Clientset.ExtensionsV1beta1().Deployments(namespace).Update(deploymentSpec)
	} else {
		return api.Clientset.ExtensionsV1beta1().Deployments(namespace).Create(deploymentSpec)
	}
}

func (api Api) createOrUpdateServiceResource(serviceSpec *v1.Service, namespace string) (*v1.Service, error) {
	if serviceSpec.ObjectMeta.ResourceVersion != "" {
		fmt.Println("updating..")
		return api.Clientset.CoreV1().Services(namespace).Update(serviceSpec)
	} else {
		fmt.Println("creating..")
		return api.Clientset.CoreV1().Services(namespace).Create(serviceSpec)
	}
}

func (api Api) createOrUpdateSecretResource(secretSpec *v1.Secret, namespace string) (*v1.Secret, error) {
	if secretSpec.ObjectMeta.ResourceVersion != "" {
		return api.Clientset.CoreV1().Secrets(namespace).Update(secretSpec)
	} else {
		return api.Clientset.CoreV1().Secrets(namespace).Create(secretSpec)
	}
}
