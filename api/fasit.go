package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"github.com/prometheus/client_golang/prometheus"
)

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
	[]string{"type"}, )

func init() {
	prometheus.MustRegister(httpReqsCounter)
}

type Properties struct{
	Url      string
	Username string
}

type Password struct {
	Ref string
}
type Secrets struct {
	Password Password
	Actual   string
}

type FasitResource struct {
	Alias        string
	ResourceType string `json:"type"`
	Properties   Properties
	Secrets      Secrets
}

type ResourceRequest struct{
	name string
	resourceType string
}

type Resource struct {
	name string
	resourceType string
	properties map[string]string
	secret string
}


type Fasit interface {
	getResource(resourcesRequest ResourceRequest, environment string, application string, zone string) (resource Resource, err error)
	getResources(resourceRequests []ResourceRequest, environment string, application string, zone string) (resources []Resource, err error)
}

type FasitAdapter struct {
	FasitUrl string
}

func (fasit FasitAdapter) getResources(resourcesRequests []ResourceRequest, environment string, application string, zone string) (resources []Resource, err error) {
	for _, request := range resourcesRequests {
		resource, err := fasit.getResource(request, environment, application, zone)
		if err != nil {
			return []Resource{}, fmt.Errorf("failed to get resource for "+request.name, err)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (fasit FasitAdapter) getResource(resourcesRequest ResourceRequest, environment string, application string, zone string) (resource Resource, err error) {
	requestCounter.With(nil).Inc()
	req, err := buildRequest(fasit.FasitUrl, resourcesRequest.name, resourcesRequest.resourceType, environment, application, zone)
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return Resource{}, fmt.Errorf("could not create request: ", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return Resource{}, fmt.Errorf("error contacting fasit: ", err)
	}
	httpReqsCounter.WithLabelValues(string(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return Resource{}, fmt.Errorf("Fasit gave errormessage: " + strconv.Itoa(resp.StatusCode))
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorCounter.WithLabelValues("read_body").Inc()
		return Resource{}, fmt.Errorf("Could not read body: ", err)
	}
	var fasitResource FasitResource

	err = json.Unmarshal(body, &fasitResource)
	if err != nil {
		errorCounter.WithLabelValues("unmarshal_bpdy").Inc()
		return Resource{}, fmt.Errorf("Could not unmarshal body: ", err)
	}

	resource = mapToNaisResource(fasitResource)

	return resource, nil
}

func mapToNaisResource(fasitResource FasitResource) (resource Resource){
	resource.name = fasitResource.Alias
	resource.resourceType = fasitResource.ResourceType
	resource.properties = make(map[string]string)
	resource.properties["url"] = fasitResource.Properties.Url
	resource.properties["username"] = fasitResource.Properties.Username
	resource.secret = fasitResource.Secrets.Password.Ref
	return
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
