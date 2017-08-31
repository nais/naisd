package api

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"strconv"
)

func init() {
	prometheus.MustRegister(httpReqsCounter)
}

type FasitClient struct {
	FasitUrl string
	Username string
	Password string
}

type Properties struct {
	Url      string
	Username string
}

type Password struct {
	Ref string
}

type FasitResource struct {
	Alias        string
	ResourceType string `json:"type"`
	Properties   map[string]string
	Secrets      map[string]map[string]string
}

type ResourceRequest struct {
	Alias        string
	ResourceType string
}

type NaisResource struct {
	name         string
	resourceType string
	properties   map[string]string
	secret       map[string]string
}

func (fasit FasitClient) GetResources(resourcesRequests []ResourceRequest, environment string, application string, zone string) (resources []NaisResource, err error) {
	for _, request := range resourcesRequests {
		resource, err := fasit.getResource(request, environment, application, zone)
		if err != nil {
			return []NaisResource{}, fmt.Errorf("failed to get resource for "+request.Alias, err)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func fetchFasitResources(fasitUrl string, deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig) ([]NaisResource, error) {
	var resourceRequests []ResourceRequest
	for _, resource := range appConfig.FasitResources.Used {
		resourceRequests = append(resourceRequests, ResourceRequest{Alias: resource.Alias, ResourceType: resource.ResourceType})
	}

	fasit := FasitClient{fasitUrl, deploymentRequest.Username, deploymentRequest.Password}

	return fasit.GetResources(resourceRequests, deploymentRequest.Environment, deploymentRequest.Application, deploymentRequest.Zone)
}

func (fasit FasitClient) getResource(resourcesRequest ResourceRequest, environment string, application string, zone string) (resource NaisResource, err error) {
	requestCounter.With(nil).Inc()
	req, err := buildRequest(fasit.FasitUrl, resourcesRequest.Alias, resourcesRequest.ResourceType, environment, application, zone)
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return NaisResource{}, fmt.Errorf("Could not create request: %s", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return NaisResource{}, fmt.Errorf("Error contacting fasit: %s", err)
	}
	httpReqsCounter.WithLabelValues(string(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return NaisResource{}, fmt.Errorf("Fasit gave errormessage: %s" + strconv.Itoa(resp.StatusCode))
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorCounter.WithLabelValues("read_body").Inc()
		return NaisResource{}, fmt.Errorf("Could not read body: %s", err)
	}
	var fasitResource FasitResource

	err = json.Unmarshal(body, &fasitResource)
	if err != nil {
		errorCounter.WithLabelValues("unmarshal_body").Inc()
		return NaisResource{}, fmt.Errorf("Could not unmarshal body: %s", err)
	}

	resource, err = fasit.mapToNaisResource(fasitResource)

	if err != nil {
		return NaisResource{}, err
	}

	return resource, nil
}

func (fasit FasitClient) mapToNaisResource(fasitResource FasitResource) (resource NaisResource, err error) {
	resource.name = fasitResource.Alias
	resource.resourceType = fasitResource.ResourceType
	resource.properties = fasitResource.Properties

	if len(fasitResource.Secrets) > 0 {
		secret, err := resolveSecret(fasitResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return NaisResource{}, fmt.Errorf("Unable to resolve secret: %s", err)
		}
		resource.secret = secret
	}

	return resource, nil
}

func resolveSecret(secrets map[string]map[string]string, username string, password string) (map[string]string, error) {

	req, err := http.NewRequest("GET", secrets[getFirstKey(secrets)]["ref"], nil)

	if err != nil {
		return map[string]string{}, err
	}

	req.SetBasicAuth(username, password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return map[string]string{}, fmt.Errorf("Error contacting fasit when resolving secret: %s", err)
	}

	httpReqsCounter.WithLabelValues(string(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return map[string]string{}, fmt.Errorf("Fasit gave errormessage when resolving secret: %s" + strconv.Itoa(resp.StatusCode))
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	return map[string]string{"password": string(body)}, nil
}

func getFirstKey(m map[string]map[string]string) string {
	if len(m) > 0 {
		for key := range m {
			return key
		}
	}
	return ""
}

func buildRequest(fasit string, alias string, resourceType string, environment string, application string, zone string) (*http.Request, error) {
	req, err := http.NewRequest("GET", fasit+"/api/v2/scopedresource", nil)

	q := req.URL.Query()
	q.Add("alias", alias)
	q.Add("type", resourceType)
	q.Add("environment", environment)
	q.Add("application", application)
	q.Add("zone", zone)
	req.URL.RawQuery = q.Encode()
	return req, err
}

var httpReqsCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasitAdapter",
		Name:      "http_requests_total",
		Help:      "How many HTTP requests processed, partitioned by status code and HTTP method.",
	},
	[]string{"code", "method"})

var requestCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasit",
		Name:      "requests",
		Help:      "Incoming requests to fasitadapter",
	},
	[]string{})

var errorCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasit",
		Name:      "errors",
		Help:      "Errors occurred in fasitadapter",
	},
	[]string{"type"})
