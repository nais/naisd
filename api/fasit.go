package api

import (
	"encoding/json"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"strconv"
	"github.com/golang/glog"
	"bytes"
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
	Certificates map[string]interface{} `json:"files""`
}

type ResourceRequest struct {
	Alias        string
	ResourceType string
}

type NaisResource struct {
	id			 int
	name         string
	resourceType string
	properties   map[string]string
	secret       map[string]string
	certificates map[string][]byte
}



func (fasit FasitClient) GetResources(resourcesRequests []ResourceRequest, environment string, application string, zone string) (resources []NaisResource, err error) {
	for _, request := range resourcesRequests {
		resource, err := fasit.getResource(request, environment, application, zone)
		if err != nil {
			return []NaisResource{}, fmt.Errorf("Failed to get resource: %s. %s", request.Alias, err)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (fasit FasitClient) createOrUpdateFasitResources(resources []ExposedResource, hostname, environment, application, zone string)([]int, error){
	var exposedResourceIds []int
	for _, resource := range resources {
		var request = ResourceRequest{Alias: resource.Alias, ResourceType: resource.ResourceType}
		existingResource, err :=  fasit.getResource(request, environment, application, zone)
		if err != nil {
			createdResource, err := fasit.PostResource(resource, environment, application, zone)
			if err != nil {
				return exposedResourceIds, fmt.Errorf("Failed creating resource: %s, %s", resource.Alias, err)
			}
			exposedResourceIds = append(exposedResourceIds, createdResource.id)
			continue
		} else if resourceHasChanged(existingResource, resource) {
			// updateResource
			if err != nil {
				return exposedResourceIds, fmt.Errorf("Failed updating resource: %s, %s", resource.Alias, err)
			}
			exposedResourceIds = append(exposedResourceIds, updatedResource.id)
			continue
		}
		// UnchangedResource
		exposedResourceIds = append(exposedResourceIds, existingResource.id)
	}


	return exposedResourceIds, nil

}

func environmentExistsInFasit(fasitUrl string, deploymentRequest NaisDeploymentRequest)(error) {
	fasit := FasitClient{fasitUrl, deploymentRequest.Username, deploymentRequest.Password}
	return fasit.getFasitEnvironment(deploymentRequest.Namespace)
}

func applicationExistsInFasit(fasitUrl string, deploymentRequest NaisDeploymentRequest)(error) {
	fasit := FasitClient{fasitUrl, deploymentRequest.Username, deploymentRequest.Password}
	return fasit.getFasitApplication(deploymentRequest.Application)
}

func fetchFasitResources(fasitUrl string, deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig) ([]NaisResource, error) {
	var resourceRequests []ResourceRequest
	for _, resource := range appConfig.FasitResources.Used {
		resourceRequests = append(resourceRequests, ResourceRequest{Alias: resource.Alias, ResourceType: resource.ResourceType})
	}

	fasit := FasitClient{fasitUrl, deploymentRequest.Username, deploymentRequest.Password}

	return fasit.GetResources(resourceRequests, deploymentRequest.Environment, deploymentRequest.Application, deploymentRequest.Zone)
}

// Updates Fasit with information
func updateFasit(fasitUrl string, deploymentRequest NaisDeploymentRequest, resources []NaisResource, appConfig NaisAppConfig, hostname string) error {
	fasit := FasitClient{fasitUrl, deploymentRequest.Username, deploymentRequest.Password}
	exposedResourceIds, err := fasit.createOrUpdateFasitResources(appConfig.FasitResources.Exposed, hostname, deploymentRequest.Environment, deploymentRequest.Application, deploymentRequest.Zone)

	if err != nil {
		panic(err)
	}

	fmt.Println(exposedResourceIds)

	// createApplicationInstance(resources, exposedResourceIds)

	return nil
}

func createOrUpdateResources(resources []ExposedResource, hostname string) ([]int, error) {
	var resourceIds []int
	for i,res := range resources {
		fmt.Println("alias", res.Alias)
		fmt.Println("path", res.Path)
		fmt.Println("type", res.ResourceType)
		fmt.Println("hostname", hostname)
		//create resource
		resourceIds = append(resourceIds, i)
	}

	return resourceIds, nil
}

func (fasit FasitClient) getResource(resourcesRequest ResourceRequest, environment string, application string, zone string) (resource NaisResource, err error) {
	requestCounter.With(nil).Inc()
	req, err := buildResourceRequest(fasit.FasitUrl, resourcesRequest.Alias, resourcesRequest.ResourceType, environment, application, zone)
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return NaisResource{}, fmt.Errorf("Could not create request: %s", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)

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

func (fasit FasitClient) postResource(resource ExposedResource, environment, application, zone string)(error){
	requestCounter.With(nil).Inc()

	req, err := http.NewRequest("POST", fasit.FasitUrl+"/api/v2/resources/", bytes.NewBuffer(buildResourcePayload(resource ExposedResource, environment, application, zone)))

	return nil
}

func (fasit FasitClient) getFasitEnvironment(environmentName string) (error){
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitUrl+"/api/v2/environments/"+environmentName, nil)
	if err != nil {
		return fmt.Errorf("Could not create request: %s", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)

	defer resp.Body.Close()

	if err != nil {
		return fmt.Errorf("Error contacting Fasit: %s", err)
	}

	if resp.StatusCode == 200 {
		return nil
	}
	return fmt.Errorf("Could not find environment %s in Fasit", environmentName)
}

func (fasit FasitClient) getFasitApplication(application string) (error){
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitUrl+"/api/v2/applications/"+application, nil)
	if err != nil {
		return fmt.Errorf("Could not create request: %s", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)

	defer resp.Body.Close()

	if err != nil {
		return fmt.Errorf("Error contacting Fasit: %s", err)
	}

	if resp.StatusCode == 200 {
		return nil
	}
	return fmt.Errorf("Could not find application %s in Fasit", application)
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

func buildResourceRequest(fasit string, alias string, resourceType string, environment string, application string, zone string) (*http.Request, error) {
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

func buildResourcePayload(resource ExposedResource, environment, application, zone string)(string){

	payload := `{
					"type":"`+resource.ResourceType+`",
					"environment":"`+environment+`",
					"application":"`+application+`",
					"scope:{
							"environmentClass":"`+environmentclass+`",
							"zone":"`+zone+`"
						}"
					}`

	return payload
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
