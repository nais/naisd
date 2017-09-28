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
	NoAppConfig  bool
	Username     string
	Password     string
	Namespace    string
}
type appError struct {
	Error   error
	Message string
	Code    int
}

type appHandler func(w http.ResponseWriter, r *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil { // e is *appError, not os.Error.
		glog.Errorf(e.Message+": %s\n", e.Error)
		http.Error(w, e.Message, e.Code)
	}
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

	mux.Handle(pat.Get("/isalive"), appHandler(api.isAlive))
	mux.Handle(pat.Post("/deploy"), appHandler(api.deploy))
	mux.Handle(pat.Get("/metrics"), promhttp.Handler())

	return mux
}

func (api Api) isAlive(w http.ResponseWriter, _ *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "isAlive"}).Inc()
	fmt.Fprint(w, "")
	return nil
}

func (api Api) deploy(w http.ResponseWriter, r *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "deploy"}).Inc()

	deploymentRequest, err := unmarshalDeploymentRequest(r.Body)
	if err != nil {
		return &appError{err, "Unable to unmarshal deployment request", http.StatusBadRequest}
	}

	glog.Infof("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.Environment)

	appConfig, err := GenerateAppConfig(deploymentRequest)
	if err != nil {
		return &appError{err, "Unable to fetch manifest", http.StatusInternalServerError}
	}

	naisResources, err := fetchFasitResources(api.FasitUrl, deploymentRequest, appConfig)
	if err != nil {
		return &appError{err, "Unable to fetch fasit resources", http.StatusInternalServerError}
	}

	deploymentResult, err := createOrUpdateK8sResources(deploymentRequest, appConfig, naisResources, api.ClusterSubdomain, api.Clientset)
	if err != nil {
		return &appError{err, "Failed while creating or updating k8s-resources", http.StatusInternalServerError}
	}

	deploys.With(prometheus.Labels{"nais_app": deploymentRequest.Application}).Inc()

	w.WriteHeader(200)
	w.Write(createResponse(deploymentResult))
	return nil
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
