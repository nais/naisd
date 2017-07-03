package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"github.com/prometheus/client_golang/prometheus"
)

var httpReqs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasitAdapter",
		Name:      "http_requests_total",
		Help:      "How many HTTP requests processed, partitioned by status code and HTTP method.",
	},
	[]string{"code", "method"})

var reqs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasit",
		Name:      "requests",
		Help:      "Incoming requests to fasitadapter",
	},
	[]string{})

var errs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasit",
		Name:      "errors",
		Help:      "Errors occurred in fasitadapter",
	},
	[]string{"type"}, )

func init() {
	prometheus.MustRegister(httpReqs)
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

type NaisResourceRequest struct{
	name string
	resourceType string
}

type NaisResource struct {
	name string
	resourceType string
	properties map[string]string
	secret string
}


type Fasit interface {
	getResource(alias string, resourceType string, environment string, application string, zone string) (resource NaisResource, err error)
	getResources(resourcesSpec []NaisResourceRequest, environment string, application string, zone string) (resources []NaisResource, err error)
}

type FasitAdapter struct {
	FasitUrl string
}

func (fasit FasitAdapter) getResources(resourcesSpec []NaisResourceRequest, environment string, application string, zone string) (resources []NaisResource, err error) {
	for _, res := range resourcesSpec {
		resource, err := fasit.getResource(res.name, res.resourceType, environment, application, zone)
		if err != nil {
			return []NaisResource{}, fmt.Errorf("failed to get resource for "+res.name, err)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (fasit FasitAdapter) getResource(alias string, resourceType string, environment string, application string, zone string) (resource NaisResource, err error) {
	reqs.With(nil).Inc()
	req, err := buildRequest(fasit.FasitUrl, alias, resourceType, environment, application, zone)
	if err != nil {
		errs.WithLabelValues("create_request").Inc()
		return NaisResource{}, fmt.Errorf("could not create request: ", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errs.WithLabelValues("contact_fasit").Inc()
		return NaisResource{}, fmt.Errorf("error contacting fasit: ", err)
	}
	httpReqs.WithLabelValues(string(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errs.WithLabelValues("error_fasit").Inc()
		return NaisResource{}, fmt.Errorf("Fasit gave errormessage: " + strconv.Itoa(resp.StatusCode))
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errs.WithLabelValues("read_body").Inc()
		return NaisResource{}, fmt.Errorf("Could not read body: ", err)
	}
	var fasitResource FasitResource

	err = json.Unmarshal(body, &fasitResource)
	if err != nil {
		errs.WithLabelValues("unmarshal_bpdy").Inc()
		return NaisResource{}, fmt.Errorf("Could not unmarshal body: ", err)
	}

	resource = mapToNaisResource(fasitResource)

	return resource, nil
}

func mapToNaisResource(fasitResource FasitResource) (resource NaisResource ){
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
