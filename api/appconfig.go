package api

import (
	"fmt"
	"io/ioutil"
	"github.com/golang/glog"
	"net/http"
	"gopkg.in/yaml.v2"
	"github.com/imdario/mergo"
	"strconv"
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
	Resources      ResourceRequirements
	FasitResources FasitResources `yaml:"fasitResources"`
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
	Fields       []Field
}

type Field struct {
	Name  string
	Value string
}

func createAppConfigUrl(appConfigUrl, application, version string) string {
	if appConfigUrl != "" {
		return appConfigUrl
	} else {
		return fmt.Sprintf("http://nexus.adeo.no/nexus/service/local/repositories/m2internal/content/nais/%s/%s/%s", application, version, fmt.Sprintf("%s-%s.yaml", application, version))
	}
}

func fetchAppConfig(deploymentRequest NaisDeploymentRequest) (naisAppConfig NaisAppConfig, err error) {

	url := createAppConfigUrl(deploymentRequest.AppConfigUrl, deploymentRequest.Application, deploymentRequest.Version)

	var defaultAppConfig = GetDefaultAppConfig(deploymentRequest)
	var appConfig NaisAppConfig

	if !deploymentRequest.NoAppConfig {

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
	}

	if err := mergo.Merge(&appConfig, defaultAppConfig); err != nil {
		glog.Errorf("Could not merge appconfig %s", err)
		return NaisAppConfig{}, err
	}

	validationErrors := validateAppConfig(appConfig);
	if len(validationErrors.Errors) != 0 {
		glog.Error("Invalid appconfig: ", validationErrors.Error())
		return NaisAppConfig{}, &validationErrors
	}

	return appConfig, nil
}

func validateAppConfig(appConfig NaisAppConfig) ValidationErrors {

	var validationErrors ValidationErrors

	if appConfig.Replicas.Max == 0 {
		validationErrors.Errors = append(validationErrors.Errors, ValidationError{
			ErrorMessage: "Replicas.Max is not set.",
			Fields: []Field{
				{Name: "Replicas.Max", Value: strconv.Itoa(appConfig.Replicas.Max)},
			}})
	}

	if appConfig.Replicas.Min == 0 {
		validationErrors.Errors = append(validationErrors.Errors, ValidationError{
			ErrorMessage: "Replicas.Min is not set.",
			Fields: []Field{
				{Name: "Replicas.Min", Value: strconv.Itoa(appConfig.Replicas.Min)},
			}})
	}

	if appConfig.Replicas.Min > appConfig.Replicas.Max {
		validationErrors.Errors = append(validationErrors.Errors, ValidationError{
			ErrorMessage: "Replicas.Min is larger than Replicas.Max.",
			Fields: []Field{
				{Name: "Replicas.Min", Value: strconv.Itoa(appConfig.Replicas.Min)},
				{Name: "Replicas.Max", Value: strconv.Itoa(appConfig.Replicas.Max)},
			}})
	}

	if appConfig.Replicas.CpuThresholdPercentage < 10 || appConfig.Replicas.CpuThresholdPercentage > 90 {
		validationErrors.Errors = append(validationErrors.Errors, ValidationError{
			ErrorMessage: "CpuThreshold must be between 10 and 90",
			Fields: []Field{
				{Name: "Replicas.CpuThreshold", Value: strconv.Itoa(appConfig.Replicas.CpuThresholdPercentage)},
			}})
	}

	return validationErrors
}

func (errors *ValidationErrors) Error() (s string) {
	for _, error := range errors.Errors {
		s += error.ErrorMessage + "\n"
		for _, field := range error.Fields {
			s+= " - " +field.Name + ": " + field.Value  + ".\n"
		}
	}
	return s
}
