package api

import (
	"fmt"
	"io/ioutil"
	"github.com/golang/glog"
	"net/http"
	"gopkg.in/yaml.v2"
	"github.com/imdario/mergo"
)

type Probe struct {
	Path string
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
	Port  	string
	Path    string
}

type NaisAppConfig struct {
	Image          string
	Port           int
	InitialDelay   int
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

func createAppConfigUrl(appConfigUrl, application, version string) string {
	if appConfigUrl != "" {
		return appConfigUrl
	} else {
		return fmt.Sprintf("http://nexus.adeo.no/nexus/service/local/repositories/m2internal/content/nais/%s/%s/%s", application, version, fmt.Sprintf("%s-%s.yaml", application, version))
	}
}

func fetchAppConfig(url string, deploymentRequest NaisDeploymentRequest) (naisAppConfig NaisAppConfig, err error) {

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

	return appConfig, nil
}
