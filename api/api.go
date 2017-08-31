package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goji.io"
	"goji.io/pat"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
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

type DeploymentResult struct {
	Autoscaler *autoscalingv1.HorizontalPodAutoscaler
	Ingress    *v1beta1.Ingress
	Deployment *v1beta1.Deployment
	Secret     *v1.Secret
	Service    *v1.Service
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
		glog.Errorf("Unable to unmarshal deployment request: %s", err)
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

	deploymentResult, err := createOrUpdateK8sResources(deploymentRequest, appConfig, naisResources, api.ClusterSubdomain, api.Clientset)

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



func unmarshalDeploymentRequest(body io.ReadCloser) (NaisDeploymentRequest, error) {
	requestBody, err := ioutil.ReadAll(body)
	if err != nil {
		return NaisDeploymentRequest{}, fmt.Errorf("Could not read deployment request body %s", err)
	}

	var deploymentRequest NaisDeploymentRequest
	if err = json.Unmarshal(requestBody, &deploymentRequest); err != nil {
		return NaisDeploymentRequest{}, fmt.Errorf("Could not unmarshal body %s", err)
	}

	return deploymentRequest, nil
}

