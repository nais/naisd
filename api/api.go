package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
	ver "github.com/nais/naisd/api/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goji.io"
	"goji.io/pat"
	"io"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"net/url"
	"os"
)

type Api struct {
	Clientset              kubernetes.Interface
	FasitURL               string
	ClusterSubdomain       string
	ClusterName            string
	IstioEnabled           bool
	AuthenticationEnabled  bool
	DeploymentStatusViewer DeploymentStatusViewer
}

var (
	clientID     = os.Getenv("AZURE_AD_SERVICE_PRINCIPAL_APP_ID")
	clientSecret = os.Getenv("AZURE_AD_SERVICE_PRINCIPAL_PASSWORD")
	tenantID     = os.Getenv("AZURE_AD_SERVICE_PRINCIPAL_TENANT")
)

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
		return fmt.Sprintf("%s: %s (%d)", e.Message, e.OriginalError.Error(), e.StatusCode)
	}

	return fmt.Sprintf("%s (%d)", e.Message, e.StatusCode)
}

type appHandler func(w http.ResponseWriter, r *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil { // e is *appError, not os.Error.
		glog.Errorf(e.Error())
		http.Error(w, e.Error(), e.StatusCode)
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
	mux.Handle(pat.Get("/version"), appHandler(api.version))
	mux.Handle(pat.Get("/deploystatus/:namespace/:deployName"), appHandler(api.deploymentStatusHandler))
	mux.Handle(pat.Delete("/app/:namespace/:deployName"), appHandler(api.deleteApplication))
	return mux
}

func NewApi(clientset kubernetes.Interface, fasitUrl, clusterDomain, clusterName string, istioEnabled bool, authenticationEnabled bool, d DeploymentStatusViewer) Api {
	return Api{
		Clientset:              clientset,
		FasitURL:               fasitUrl,
		ClusterSubdomain:       clusterDomain,
		ClusterName:            clusterName,
		IstioEnabled:           istioEnabled,
		AuthenticationEnabled:  authenticationEnabled,
		DeploymentStatusViewer: d,
	}
}
func authenticate(username, password string) *appError {
	authURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/token", tenantID)
	content := url.Values{}
	content.Add("grant_type", "password")
	content.Add("username", username)
	content.Add("password", password)
	content.Add("client_id", clientID)
	content.Add("client_secret", clientSecret)
	content.Add("resource", clientID)

	res, err := http.PostForm(authURL, content)
	if err != nil {
		return &appError{err, "Failed during authentication", 500}
	}
	if res.StatusCode != 200 {
		return &appError{err, "Authentication failure", 401}
	}
	return nil
}

func (api Api) deploy(w http.ResponseWriter, r *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "deploy"}).Inc()
	deploymentRequest, err := unmarshalDeploymentRequest(r.Body)

	if api.AuthenticationEnabled {
		username, password, _ := r.BasicAuth()
		if clientID == "" || tenantID == "" || clientSecret == "" {
			return &appError{fmt.Errorf("missing Azure AD configuration"), "", 500}
		}
		appErr := authenticate(username, password)
		if appErr != nil {
			return appErr
		}
	}

	// Warn about deprecated fields in deploymentRequest and set default env if not set
	warnings := ensurePropertyCompatibility(&deploymentRequest)
	if len(deploymentRequest.Namespace) == 0 {
		deploymentRequest.Namespace = "app"
	}

	if len(deploymentRequest.Namespace) == 0 {
		deploymentRequest.Namespace = "default"
	}

	if err != nil {
		return &appError{err, "unable to unmarshal deployment request", http.StatusBadRequest}
	}
	glog.Infof("Received deployment request: %s", deploymentRequest)

	fasit := FasitClient{api.FasitURL, deploymentRequest.FasitUsername, deploymentRequest.FasitPassword}

	glog.Infof("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.FasitEnvironment)

	manifest, err := GenerateManifest(deploymentRequest)
	if err != nil {
		return &appError{err, "unable to generate manifest/nais.yaml", http.StatusInternalServerError}
	}

	var fasitEnvironmentClass string
	var naisResources []NaisResource

	if !deploymentRequest.SkipFasit {
		glog.Infof("Starting deployment. Deploying %s:%s to %s\n", deploymentRequest.Application, deploymentRequest.Version, deploymentRequest.FasitEnvironment)
	} else {
		glog.Infof("Starting deployment. Deploying %s:%s. Fasit will be skipped\n", deploymentRequest.Application, deploymentRequest.Version)
	}

	if !deploymentRequest.SkipFasit {
		if hasResources(manifest) {
			if deploymentRequest.FasitEnvironment == "" {
				return &appError{err, "no fasit environment provided, but contains resources to be consumed or exposed", http.StatusInternalServerError}
			}
			if err := validateFasitRequirements(fasit, deploymentRequest.Application, deploymentRequest.FasitEnvironment); err != nil {
				return &appError{err, "validating requirements for deployment failed", http.StatusInternalServerError}
			}
			fasitEnvironmentClass, err = fasit.GetFasitEnvironmentClass(deploymentRequest.FasitEnvironment)
		}

		naisResources, err = FetchFasitResources(fasit, deploymentRequest.Application, deploymentRequest.FasitEnvironment, deploymentRequest.Zone, manifest.FasitResources.Used)
		if err != nil {
			return &appError{err, "unable to fetch fasit resources", http.StatusBadRequest}
		}
	}

	spec := app.Spec{
		Application: deploymentRequest.Application,
		Namespace:   deploymentRequest.Namespace,
		Team:        manifest.Team,
	}
	deploymentResult, err := createOrUpdateK8sResources(spec, deploymentRequest, manifest, naisResources, api.ClusterSubdomain, api.IstioEnabled, api.Clientset)
	if err != nil {
		return &appError{err, "failed while creating or updating k8s-resources", http.StatusInternalServerError}
	}

	deploys.With(prometheus.Labels{"nais_app": deploymentRequest.Application}).Inc()

	if !deploymentRequest.SkipFasit && hasResources(manifest) {
		if err := updateFasit(fasit, deploymentRequest, naisResources, manifest, createIngressHostname(deploymentRequest.Application, deploymentRequest.Namespace, api.ClusterSubdomain), fasitEnvironmentClass, deploymentRequest.FasitEnvironment, api.ClusterSubdomain); err != nil {
			return &appError{err, "failed while updating Fasit", http.StatusInternalServerError}
		}
	}

	NotifySensuAboutDeploy(spec, &deploymentRequest, &api.ClusterName)

	w.WriteHeader(200)
	w.Write(createResponse(deploymentResult, warnings))
	return nil
}

func (api Api) deploymentStatusHandler(w http.ResponseWriter, r *http.Request) *appError {
	namespace := pat.Param(r, "namespace")
	deployName := pat.Param(r, "deployName")

	status, view, err := api.DeploymentStatusViewer.DeploymentStatusView(namespace, deployName)

	if err != nil {
		return &appError{err, "deployment not found ", http.StatusNotFound}
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

func (api Api) version(w http.ResponseWriter, _ *http.Request) *appError {
	response := map[string]string{"version": ver.Version, "revision": ver.Revision}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return &appError{err, "unable to encode JSON", 500}
	}

	return nil
}

func (api Api) deleteApplication(w http.ResponseWriter, r *http.Request) *appError {
	namespace := pat.Param(r, "namespace")
	application := pat.Param(r, "deployName")

	spec := app.Spec{Application: application, Namespace: namespace}
	result, err := deleteK8sResouces(spec, api.Clientset)

	response := ""
	if len(result) > 0 {
		response = "result: \n"
		for _, res := range result {
			response += res + "\n"
		}
	}

	if err != nil {
		return &appError{err, fmt.Sprintf("there were errors when trying to delete app: %+v", response), http.StatusInternalServerError}
	}

	glog.Infof("Deleted application %s in %s\n", application, namespace)

	w.Write([]byte(response))
	w.WriteHeader(http.StatusOK)
	return nil
}

func validateFasitRequirements(fasit FasitClientAdapter, application, fasitEnvironment string) error {
	if _, err := fasit.GetFasitEnvironmentClass(fasitEnvironment); err != nil {
		glog.Errorf("Environment '%s' does not exist in Fasit", fasitEnvironment)
		return fmt.Errorf("unable to get fasit environment: %s. %s", fasitEnvironment, err)
	}
	if err := fasit.GetFasitApplication(application); err != nil {
		glog.Errorf("Application '%s' does not exist in Fasit", application)
		return fmt.Errorf("unable to get fasit application: %s. %s", application, err)
	}

	return nil
}

func hasResources(manifest NaisManifest) bool {
	if len(manifest.FasitResources.Used) == 0 && len(manifest.FasitResources.Exposed) == 0 {
		return false
	}
	return true
}

func createResponse(deploymentResult DeploymentResult, warnings []string) []byte {
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
	if deploymentResult.AlertsConfigMap != nil {
		response += "- updated alerts configmap (app-rules)\n"
	}
	if deploymentResult.Redis != nil {
		response += "- created redis\n"
	}
	if deploymentResult.ServiceAccount != nil {
		response += "- created serviceaccount\n"
	}
	if deploymentResult.RoleBinding != nil {
		response += "- created rolebinding\n"
	}

	if len(warnings) > 0 {
		response += "\nWarnings:\n"
		for _, warning := range warnings {
			response += fmt.Sprintf("- %s\n", warning)
		}
	}

	return []byte(response)
}

func unmarshalDeploymentRequest(body io.ReadCloser) (naisrequest.Deploy, error) {
	requestBody, err := ioutil.ReadAll(body)
	if err != nil {
		return naisrequest.Deploy{}, fmt.Errorf("could not read deployment request body %s", err)
	}

	deploymentRequest := naisrequest.Deploy{}

	if err = json.Unmarshal(requestBody, &deploymentRequest); err != nil {
		return naisrequest.Deploy{}, fmt.Errorf("could not unmarshal body %s", err)
	}

	return deploymentRequest, nil
}

func ensurePropertyCompatibility(deploy *naisrequest.Deploy) []string {
	var warnings []string

	if len(deploy.Environment) > 0 {
		warnings = append(warnings, "Specifying environment is deprecated, specify namespace instead.")
	}

	return warnings
}
