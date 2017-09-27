package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goji.io"
	"goji.io/pat"
	"io"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"net/http"
)

type Api struct {
	Clientset        kubernetes.Interface
	FasitUrl         string
	ClusterSubdomain string
}

type NaisDeploymentRequest struct {
	Application  string `json:"application"`
	Version      string `json:"version"`
	Environment  string `json:"environment"`
	Zone         string `json:"zone"`
	AppConfigUrl string `json:"appconfigurl,omitempty"`
	NoAppConfig  bool   `json:"-"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Namespace    string `json:"namespace"`
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

	appConfig, err := GenerateAppConfig(deploymentRequest)

	if err != nil {
		glog.Errorf("Unable to fetch manifest: %s\n", err)
		w.WriteHeader(500)
		w.Write([]byte("Could not fetch manifest: " + err.Error()))
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

	response := createResponse(deploymentResult)
	w.Write(response)
}
func createResponse(deploymentResult DeploymentResult) []byte {

	response := "result: \n"

	if deploymentResult.Deployment != nil {
		response += "- created deployment\n"
	}
	if deploymentResult.Secret != nil {
		response += "- created secret\n"
	}
	if deploymentResult.Service != nil {
		response += "- created service\n"
	}
	if deploymentResult.Ingress != nil {
		response += "- created ingress\n"
	}
	if deploymentResult.Autoscaler != nil {
		response += "- created autoscaler\n"
	}

	return []byte(response)
}

func (r NaisDeploymentRequest) Validate() []error {
	required := map[string]*string{
		"Application": &r.Application,
		"Version":     &r.Version,
		"Environment": &r.Environment,
		"Zone":        &r.Zone,
		"Username":    &r.Username,
		"Password":    &r.Password,
		"Namespace":   &r.Namespace,
	}

	var errors []error
	for key, pointer := range required {
		if len(*pointer) == 0 {
			errors = append(errors, fmt.Errorf("%s is required and is empty", key))
		}
	}

	return errors
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
