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
		Help:      "Errors occured in fasitadapter",
	},
	[]string{"type"}, )

func init() {
	prometheus.MustRegister(httpReqs)
}

type Properties struct {
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

type Fasit interface {
	getResource(alias string, resourceType string, environment string, application string, zone string) (resource FasitResource, err error)
	getResources(resourcesSpec []FasitResource, environment string, application string, zone string) (resources []FasitResource, err error)
}

type FasitAdapter struct {
	FasitUrl string
}

func (fasit FasitAdapter) getResources(resourcesSpec []FasitResource, environment string, application string, zone string) (resources []FasitResource, err error) {
	for _, res := range resourcesSpec {
		resource, err := fasit.getResource(res.Alias, res.ResourceType, environment, application, zone)
		if err != nil {
			return []FasitResource{}, fmt.Errorf("failed to get resource for "+res.Alias, err)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (fasit FasitAdapter) getResource(alias string, resourceType string, environment string, application string, zone string) (resource FasitResource, err error) {
	reqs.With(nil).Inc()
	req, err := buildRequest(err, fasit, alias, resourceType, environment, application, zone)
	if err != nil {
		errs.WithLabelValues("create_request").Inc()
		return FasitResource{}, fmt.Errorf("could not create request: ", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errs.WithLabelValues("contact_fasit").Inc()
		return FasitResource{}, fmt.Errorf("error contacting fasit: ", err)
	}
	httpReqs.WithLabelValues(string(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errs.WithLabelValues("error_fasit").Inc()
		return FasitResource{}, fmt.Errorf("Fasit gave errormessage: " + strconv.Itoa(resp.StatusCode))
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errs.WithLabelValues("read_body").Inc()
		return FasitResource{}, fmt.Errorf("Could not read body: ", err)
	}

	err = json.Unmarshal(body, &resource)
	if err != nil {
		errs.WithLabelValues("unmarshal_bpdy").Inc()
		return FasitResource{}, fmt.Errorf("Could not unmarshal body: ", err)
	}
	return resource, nil
}
func buildRequest(err error, fasit FasitAdapter, alias string, resourceType string, environment string, application string, zone string) (*http.Request, error) {
	req, err := http.NewRequest("GET", fasit.FasitUrl+"/api/v2/scopedresource", nil)
	q := req.URL.Query()
	q.Add("alias", alias)
	q.Add("type", resourceType)
	q.Add("environment", environment)
	q.Add("application", application)
	q.Add("zone", zone)
	req.URL.RawQuery = q.Encode()
	return req, err
}
