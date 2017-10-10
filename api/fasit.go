package api

import (
	"encoding/json"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
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
	ResourceType string                 `json:"type"`
	Properties   map[string]string
	Secrets      map[string]map[string]string
	Certificates map[string]interface{} `json:"files""`
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
	certificates map[string][]byte
}


func (fasit FasitClient) GetScopedResources(resourcesRequests []ResourceRequest, environment string, application string, zone string) (resources []NaisResource, err error) {
	for _, request := range resourcesRequests {
		resource, err := fasit.getScopedResource(request, environment, application, zone)
		if err != nil {
			return []NaisResource{}, fmt.Errorf("Failed to get resource: %s. %s", request.Alias, err)
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

	return fasit.GetScopedResources(resourceRequests, deploymentRequest.Environment, deploymentRequest.Application, deploymentRequest.Zone)
}

func (fasit FasitClient) doRequest(r *http.Request) (resource NaisResource, err error) {
	requestCounter.With(nil).Inc()

	client := &http.Client{}
	resp, err := client.Do(r)

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorCounter.WithLabelValues("read_body").Inc()
		return NaisResource{}, fmt.Errorf("Could not read body: %s", err)
	}

	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return NaisResource{}, fmt.Errorf("Error contacting fasit: %s", err)
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return NaisResource{}, fmt.Errorf("Fasit returned: %s (%s)", body, strconv.Itoa(resp.StatusCode))
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
func (fasit FasitClient) getScopedResource(resourcesRequest ResourceRequest, environment string, application string, zone string) (resource NaisResource, err error) {

	req, err := fasit.buildRequest("GET", "/api/v2/scopedresource", map[string]string{
		"alias":       resourcesRequest.Alias,
		"type":        resourcesRequest.ResourceType,
		"environment": environment,
		"application": application,
		"zone":        zone,
	})

	if err != nil {
		return NaisResource{}, err
	}

	return fasit.doRequest(req)

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

	if fasitResource.ResourceType == "certificate" && len(fasitResource.Certificates) > 0 {
		files, err := resolveCertificates(fasitResource.Certificates, fasitResource.Alias)

		if err != nil {
			errorCounter.WithLabelValues("resolve_file").Inc()
			return NaisResource{}, fmt.Errorf("unable to resolve Certificates: %s", err)
		}

		resource.certificates = files

	} else if fasitResource.ResourceType == "applicationproperties" {
		for _, prop := range strings.Split(fasitResource.Properties["applicationProperties"], "\r\n") {
			parts := strings.SplitN(prop, "=", 2)
			resource.properties[parts[0]] = parts[1]
		}
		delete(resource.properties, "applicationProperties")
	}

	return resource, nil
}
func resolveCertificates(files map[string]interface{}, resourceName string) (map[string][]byte, error) {
	fileContent := make(map[string][]byte)

	fileName, fileUrl, err := parseFilesObject(files)
	if err != nil {
		return fileContent, err
	}

	response, err := http.Get(fileUrl)
	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return fileContent, fmt.Errorf("error contacting fasit when resolving file: %s", err)
	}
	defer response.Body.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return fileContent, fmt.Errorf("error downloading file: %s", err)
	}

	fileContent[resourceName+"_"+fileName] = bodyBytes
	return fileContent, nil

}

func parseFilesObject(files map[string]interface{}) (fileName string, fileUrl string, e error) {
	json, err := gabs.Consume(files)
	if err != nil {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return "", "", fmt.Errorf("Error parsing fasit json: %s ", files)
	}

	fileName, fileNameFound := json.Path("keystore.filename").Data().(string)
	if !fileNameFound {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return "", "", fmt.Errorf("Error parsing fasit json. Filename not found: %s ", files)
	}

	fileUrl, fileUrlfound := json.Path("keystore.ref").Data().(string)
	if !fileUrlfound {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return "", "", fmt.Errorf("Error parsing fasit json. Fileurl not found: %s ", files)
	}

	return fileName, fileUrl, nil
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

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
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

func (fasit FasitClient) buildRequest(method, path string, queryParams map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(method, fasit.FasitUrl+path, nil)

	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return nil, fmt.Errorf("could not create request: %s", err)
	}

	q := req.URL.Query()

	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()
	return req, nil
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
