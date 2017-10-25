package api

import (
	"encoding/json"
	"errors"
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
	Clientset              kubernetes.Interface
	FasitUrl               string
	ClusterSubdomain       string
	ClusterName			   string
	DeploymentStatusViewer DeploymentStatusViewer
}

type NaisDeploymentRequest struct {
	Application  string `json:"application"`
	Version      string `json:"version"`
	Environment  string `json:"environment"`
	Zone         string `json:"zone"`
	AppConfigUrl string `json:"appconfigurl,omitempty"`
	NoAppConfig  bool   `json:"-"`
	NoFasit		 bool	`json:"-"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Namespace    string `json:"namespace"`
}

type AppError interface {
	error
	Code() int
}

type appError struct {
	OriginalError error
	Message       string
	StatusCode    int
}

func (e appError) Code() int {
	return e.StatusCode
}
func (e appError) Error() string {
	if e.OriginalError != nil {
		return fmt.Sprintf("%s: %s, (%d)", e.Message, e.OriginalError.Error(), e.StatusCode)
	}
	return fmt.Sprintf("%s: (%d)", e.Message, e.StatusCode)

}

type appHandler func(w http.ResponseWriter, r *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil { // e is *appError, not os.Error.
		glog.Errorf(e.Message+": %s\n", e.Error)
		http.Error(w, e.Message, e.StatusCode)
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

func (api Api) Handler() http.Handler {
	mux := goji.NewMux()

	mux.Handle(pat.Get("/isalive"), appHandler(api.isAlive))
	mux.Handle(pat.Post("/deploy"), appHandler(api.deploy))
	mux.Handle(pat.Get("/metrics"), promhttp.Handler())
	mux.Handle(pat.Get("/deploystatus/:namespace/:deployName"), appHandler(api.deploymentStatusHandler))
	return mux
}

func NewApi(clientset kubernetes.Interface, fasitUrl, clusterDomain, clusterName string, d DeploymentStatusViewer) Api {
	return Api{
		Clientset:              clientset,
		FasitUrl:               fasitUrl,
		ClusterSubdomain:       clusterDomain,
		ClusterName:			clusterName,
		DeploymentStatusViewer: d,
	}
}

func (api Api) deploymentStatusHandler(w http.ResponseWriter, r *http.Request) *appError {
	namespace := pat.Param(r, "namespace")
	deployName := pat.Param(r, "deployName")

	status, view, err := api.DeploymentStatusViewer.DeploymentStatusView(namespace, deployName)

	if err != nil {
		return &appError{err, "Deployment not found ", http.StatusNotFound}
	}

	switch status {
	case InProgress:
		w.WriteHeader(http.StatusAccepted)
	case Failed:
		w.WriteHeader(http.StatusInternalServerError)
	case Success:
		w.WriteHeader(http.StatusOK)
	}

	if b, err := json.Marshal(view); err == nil {
		w.Write(b)
	} else {
		glog.Errorf("Unable to marshal deploy status view: %+v", view)
	}

	return nil
}

func (api Api) isAlive(w http.ResponseWriter, _ *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "isAlive"}).Inc()
	fmt.Fprint(w, "")
	return nil
}

func validateFasitRequirements(fasit FasitClientAdapter, application, fasitEnvironment string)error{
	if err := fasit.GetFasitEnvironment(fasitEnvironment); err != nil {
		glog.Errorf("Environment '%s' does not exist in Fasit", fasitEnvironment)
		return err
	}
	if err := fasit.GetFasitApplication(application); err != nil {
		glog.Errorf("Application '%s' does not exist in Fasit", application)
		return err
	}

	return nil
}
func (api Api) deploy(w http.ResponseWriter, r *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "deploy"}).Inc()


	deploymentRequest, err := unmarshalDeploymentRequest(r.Body)
	if err != nil {
		return &appError{err, "Unable to unmarshal deployment request", http.StatusBadRequest}
	}

	fasit := FasitClient{api.FasitUrl, deploymentRequest.Username, deploymentRequest.Password}

	glog.Infof("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.Environment)

	appConfig, err := GenerateAppConfig(deploymentRequest)
	if err != nil {
		return &appError{err, "Unable to fetch manifest", http.StatusInternalServerError}
	}

	fasitEnvironment := fasit.environmentNameFromNamespaceBuilder(deploymentRequest.Namespace, api.ClusterName)

	if !deploymentRequest.NoFasit {
		if err := validateFasitRequirements(fasit, deploymentRequest.Application, fasitEnvironment); err != nil {
			return &appError{err, "Validating requirements for deployment failed", http.StatusInternalServerError}
		}
	}

	glog.Infof("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.Environment)

	naisResources, err := fetchFasitResources(fasit, deploymentRequest, appConfig)
	if err != nil {
		return &appError{err, "Unable to fetch fasit resources", http.StatusInternalServerError}
	}

	deploymentResult, err := createOrUpdateK8sResources(deploymentRequest, appConfig, naisResources, api.ClusterSubdomain, api.Clientset)
	if err != nil {
		return &appError{err, "Failed while creating or updating k8s-resources", http.StatusInternalServerError}
	}

	deploys.With(prometheus.Labels{"nais_app": deploymentRequest.Application}).Inc()

	ingressHostnames := deploymentResult.Ingress.Spec.Rules
	ingressHostname := ingressHostnames[len(ingressHostnames)-1].Host

	if !deploymentRequest.NoFasit{
		if err := updateFasit(fasit, deploymentRequest, naisResources, appConfig, ingressHostname, api.ClusterName); err != nil {
			return &appError{err, "Failed while updating Fasit", http.StatusInternalServerError}
		}
	}

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

	var errs []error
	for key, pointer := range required {
		if len(*pointer) == 0 {
			errs = append(errs, fmt.Errorf("%s is required and is empty", key))
		}
	}

	if r.Zone != "fss" && r.Zone != "sbs" && r.Zone != "iapp" {
		errs = append(errs, errors.New("Zone can only be fss, sbs or iapp"))
	}

	return errs
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
