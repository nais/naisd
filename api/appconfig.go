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
	Path         string
	InitialDelay int `yaml:"initialDelay"`
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
	Image          string
	Port           int
	Healthcheck    Healthcheck
	Prometheus     PrometheusConfig
	Replicas       Replicas
	Ingress		   Ingress
	Resources      ResourceRequirements
	FasitResources FasitResources `yaml:"fasitResources"`
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
	ResourceType string `yaml:"resourceType"`
}

type ExposedResource struct {
	Alias        string
	ResourceType string `yaml:"resourceType"`
	Path         string
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

	var appConfig NaisAppConfig

	if !deploymentRequest.NoAppConfig {
		if len(deploymentRequest.AppConfigUrl) > 0 {
			appConfig, err = fetchAppConfig(deploymentRequest.AppConfigUrl, &appConfig)
			if err != nil {
				return NaisAppConfig{}, err
			}
		} else {
			urls := createAppConfigUrls(deploymentRequest.Application, deploymentRequest.Version)

			appConfig, err = fetchAppConfig(urls[0], &appConfig)
			if err != nil {
				glog.Infof("No manifest found on URL %s\n", urls[0])
				appConfig, err = fetchAppConfig(urls[1], &appConfig)
				if err != nil {
					return NaisAppConfig{}, err
				}

			}
		}
	}
	return appConfig, nil
}

func createAppConfigUrls(application, version string) [2]string {
	var urls = [2]string{}
	baseUrl := "http://nexus.adeo.no/nexus/service/local/repositories/m2internal/content/nais"
	urls[0] = fmt.Sprintf("%s/%s/%s/nais.yaml", baseUrl, application, version)
	urls[1] = fmt.Sprintf("%s/%s/%s/%s.yaml", baseUrl, application, version, application+"-"+version)
	return urls
}

func AddDefaultAppconfigValues(config *NaisAppConfig, application string) error {
	return mergo.Merge(config, GetDefaultAppConfig(application))
}
func fetchAppConfig(url string, appConfig *NaisAppConfig) (NaisAppConfig, error) {

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

	if body, err := ioutil.ReadAll(response.Body); err != nil {
		return NaisAppConfig{}, err
	} else {
		if err := yaml.Unmarshal(body, &appConfig); err != nil {
			glog.Errorf("Could not unmarshal yaml %s", err)
			return NaisAppConfig{}, err
		}
		glog.Infof("Got manifest %s", appConfig)
	}
	return *appConfig, err
}

func ValidateAppConfig(appConfig NaisAppConfig) ValidationErrors {
	validations := []func(NaisAppConfig) *ValidationError{
		validateImage,
		validateReplicasMax,
		validateReplicasMin,
		validateMinIsSmallerThanMax,
		validateCpuThreshold,
	}

	var validationErrors ValidationErrors
	for _, valfunc := range validations {
		if valError := valfunc(appConfig); valError != nil {
			validationErrors.Errors = append(validationErrors.Errors, *valError)
		}
	}

	return validationErrors
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
