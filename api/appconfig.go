package api

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type Probe struct {
	Path             string
	InitialDelay     int `yaml:"initialDelay"`
	PeriodSeconds    int `yaml:"periodSeconds"`
	FailureThreshold int `yaml:"failureThreshold"`
}

type Healthcheck struct {
	Liveness  Probe
	Readiness Probe
}

type ResourceList struct {
	Cpu    string
	Memory string
}

type ResourceRequirements struct {
	Limits   ResourceList
	Requests ResourceList
}

type PrometheusConfig struct {
	Enabled bool
	Port    string
	Path    string
}

type NaisAppConfig struct {
	Image           string
	Port            int
	Healthcheck     Healthcheck
	PreStopHookPath string `yaml:"preStopHookPath"`
	Prometheus      PrometheusConfig
	Replicas        Replicas
	Ingress         Ingress
	Resources       ResourceRequirements
	FasitResources  FasitResources `yaml:"fasitResources"`
}

type Ingress struct {
	Enabled bool
}

type Replicas struct {
	Min                    int
	Max                    int
	CpuThresholdPercentage int `yaml:"cpuThresholdPercentage"`
}

type FasitResources struct {
	Used    []UsedResource
	Exposed []ExposedResource
}

type UsedResource struct {
	Alias        string
	ResourceType string            `yaml:"resourceType"`
	PropertyMap  map[string]string `yaml:"propertyMap"`
}

type ExposedResource struct {
	Alias          string
	ResourceType   string `yaml:"resourceType"`
	Path           string
	Description    string
	WsdlGroupId    string `yaml:"wsdlGroupId"`
	WsdlArtifactId string `yaml:"wsdlArtifactId"`
	WsdlVersion    string `yaml:"wsdlVersion"`
	SecurityToken  string `yaml:"securityToken"`
	AllZones       bool   `yaml:"allZones"`
}

type ValidationErrors struct {
	Errors []ValidationError
}

type ValidationError struct {
	ErrorMessage string
	Fields       map[string]string
}

type Field struct {
	Name  string
	Value string
}

func GenerateAppConfig(deploymentRequest NaisDeploymentRequest) (naisAppConfig NaisAppConfig, err error) {

	appConfig, err := downloadAppConfig(deploymentRequest)
	if err != nil {
		glog.Errorf("could not download appconfig", err)
		return NaisAppConfig{}, err
	}

	if err := AddDefaultAppconfigValues(&appConfig, deploymentRequest.Application); err != nil {
		glog.Errorf("Could not merge appconfig %s", err)
		return NaisAppConfig{}, err
	}

	validationErrors := ValidateAppConfig(appConfig)
	if len(validationErrors.Errors) != 0 {
		glog.Error("Invalid appconfig: ", validationErrors.Error())
		return NaisAppConfig{}, validationErrors
	}

	return appConfig, nil
}

func downloadAppConfig(deploymentRequest NaisDeploymentRequest) (naisAppConfig NaisAppConfig, err error) {
	// manifest url is provided in deployment request
	if len(deploymentRequest.AppConfigUrl) > 0 {
		appConfig, err := fetchAppConfig(deploymentRequest.AppConfigUrl)
		if err != nil {
			return NaisAppConfig{}, err
		} else {
			return appConfig, nil
		}
	}

	// not provided, using defaults
	urls := createAppConfigUrls(deploymentRequest.Application, deploymentRequest.Version)
	for _, url := range urls {
		appConfig, err := fetchAppConfig(url)
		if err == nil {
			return appConfig, nil
		}
	}

	glog.Infof("No manifest found on URLs %s\n", urls)
	return NaisAppConfig{}, err
}

func createAppConfigUrls(application, version string) []string {
	return []string{
		fmt.Sprintf("https://repo.adeo.no/repository/raw/nais/%s/%s/nais.yaml", application, version),
		fmt.Sprintf("http://nexus.adeo.no/nexus/service/local/repositories/m2internal/content/nais/%s/%s/nais.yaml", application, version),
		fmt.Sprintf("http://nexus.adeo.no/nexus/service/local/repositories/m2internal/content/nais/%s/%s/%s.yaml", application, version, application+"-"+version),
	}
}

func AddDefaultAppconfigValues(config *NaisAppConfig, application string) error {
	return mergo.Merge(config, GetDefaultAppConfig(application))
}
func fetchAppConfig(url string) (NaisAppConfig, error) {
	glog.Infof("Fetching manifest from URL %s\n", url)
	response, err := http.Get(url)
	if err != nil {
		glog.Errorf("Could not fetch %s", err)
		return NaisAppConfig{}, fmt.Errorf("HTTP GET failed for url: %s. %s", url, err.Error())
	}

	defer response.Body.Close()

	if response.StatusCode > 299 {
		return NaisAppConfig{}, fmt.Errorf("Got HTTP status code %d fetching manifest from URL: %s", response.StatusCode, url)
	}

	if body, err := ioutil.ReadAll(response.Body); err != nil {
		return NaisAppConfig{}, err
	} else {
		var appConfig NaisAppConfig
		if err := yaml.Unmarshal(body, &appConfig); err != nil {
			glog.Errorf("Could not unmarshal yaml %s", err)
			return NaisAppConfig{}, fmt.Errorf("Unable to unmarshal yaml: %s", err.Error())
		}
		glog.Infof("Got manifest %s", appConfig)
		return appConfig, err
	}
}

func ValidateAppConfig(appConfig NaisAppConfig) ValidationErrors {
	validations := []func(NaisAppConfig) *ValidationError{
		validateImage,
		validateReplicasMax,
		validateReplicasMin,
		validateMinIsSmallerThanMax,
		validateCpuThreshold,
		validateResources,
	}

	var validationErrors ValidationErrors
	for _, valfunc := range validations {
		if valError := valfunc(appConfig); valError != nil {
			validationErrors.Errors = append(validationErrors.Errors, *valError)
		}
	}

	return validationErrors
}

func validateResources(appConfig NaisAppConfig) *ValidationError {
	for _, resource := range appConfig.FasitResources.Exposed {
		if resource.ResourceType == "" || resource.Alias == "" {
			return &ValidationError{
				"Alias and ResourceType must be specified",
				map[string]string{"Alias": resource.Alias},
			}
		}
	}
	for _, resource := range appConfig.FasitResources.Used {
		if resource.ResourceType == "" || resource.Alias == "" {
			return &ValidationError{
				"Alias and ResourceType must be specified",
				map[string]string{"Alias": resource.Alias},
			}
		}
	}
	return nil
}
func validateImage(appConfig NaisAppConfig) *ValidationError {
	if strings.LastIndex(appConfig.Image, ":") > strings.LastIndex(appConfig.Image, "/") {
		return &ValidationError{
			"Image cannot contain tag",
			map[string]string{"Image": appConfig.Image},
		}
	}
	return nil
}

func validateCpuThreshold(appConfig NaisAppConfig) *ValidationError {
	if appConfig.Replicas.CpuThresholdPercentage < 10 || appConfig.Replicas.CpuThresholdPercentage > 90 {
		error := new(ValidationError)
		error.ErrorMessage = "CpuThreshold must be between 10 and 90."
		error.Fields = make(map[string]string)
		error.Fields["Replicas.CpuThreshold"] = strconv.Itoa(appConfig.Replicas.CpuThresholdPercentage)
		return error

	}
	return nil

}
func validateMinIsSmallerThanMax(appConfig NaisAppConfig) *ValidationError {
	if appConfig.Replicas.Min > appConfig.Replicas.Max {
		validationError := new(ValidationError)
		validationError.ErrorMessage = "Replicas.Min is larger than Replicas.Max."
		validationError.Fields = make(map[string]string)
		validationError.Fields["Replicas.Max"] = strconv.Itoa(appConfig.Replicas.Max)
		validationError.Fields["Replicas.Min"] = strconv.Itoa(appConfig.Replicas.Min)
		return validationError
	}
	return nil

}
func validateReplicasMin(appConfig NaisAppConfig) *ValidationError {
	if appConfig.Replicas.Min == 0 {
		validationError := new(ValidationError)
		validationError.ErrorMessage = "Replicas.Min is not set"
		validationError.Fields = make(map[string]string)
		validationError.Fields["Replicas.Min"] = strconv.Itoa(appConfig.Replicas.Min)
		return validationError

	}
	return nil

}
func validateReplicasMax(appConfig NaisAppConfig) *ValidationError {
	if appConfig.Replicas.Max == 0 {
		validationError := new(ValidationError)
		validationError.ErrorMessage = "Replicas.Max is not set"
		validationError.Fields = make(map[string]string)
		validationError.Fields["Replicas.Max"] = strconv.Itoa(appConfig.Replicas.Max)
		return validationError
	}
	return nil

}

func (errors ValidationErrors) Error() (s string) {
	for _, validationError := range errors.Errors {
		s += validationError.ErrorMessage + "\n"
		for k, v := range validationError.Fields {
			s += " - " + k + ": " + v + ".\n"
		}
	}
	return s
}
