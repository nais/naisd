package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prometheus.MustRegister(httpReqsCounter)
}

type ResourcePayload interface{}

type RestResourcePayload struct {
	Alias      string         `json:"alias"`
	Scope      Scope          `json:"scope"`
	Type       string         `json:"type"`
	Properties RestProperties `json:"properties"`
}
type WebserviceResourcePayload struct {
	Alias      string               `json:"alias"`
	Scope      Scope                `json:"scope"`
	Type       string               `json:"type"`
	Properties WebserviceProperties `json:"properties"`
}
type WebserviceProperties struct {
	EndpointUrl   string `json:"endpointUrl"`
	WsdlUrl       string `json:"wsdlUrl"`
	SecurityToken string `json:"securityToken"`
	Description   string `json:"description,omitempty"`
}
type RestProperties struct {
	Url         string `json:"url"`
	Description string `json:"description,omitempty"`
}
type Scope struct {
	EnvironmentClass string `json:"environmentclass"`
	Environment      string `json:"environment,omitempty"`
	Zone             string `json:"zone,omitempty"`
}

type Password struct {
	Ref string `json:"ref"`
}
type ApplicationInstancePayload struct {
	Application      string     `json:"application"`
	Environment      string     `json:"environment"`
	Version          string     `json:"version"`
	ExposedResources []Resource `json:"exposedresources"`
	UsedResources    []Resource `json:"usedresources"`
	ClusterName      string     `json:"clustername"`
	Domain           string     `json:"domain"`
}

type Resource struct {
	Id int `json:"id"`
}

type FasitClient struct {
	FasitUrl string
	Username string
	Password string
}
type FasitClientAdapter interface {
	getScopedResource(resourcesRequest ResourceRequest, environment, application, zone string) (NaisResource, AppError)
	createResource(resource ExposedResource, fasitEnvironmentClass, environment, hostname string, deploymentRequest NaisDeploymentRequest) (int, error)
	updateResource(existingResource NaisResource, resource ExposedResource, fasitEnvironmentClass, environment, hostname string, deploymentRequest NaisDeploymentRequest) (int, error)
	GetFasitEnvironment(environmentName string) (string, error)
	GetFasitApplication(application string) error
	GetScopedResources(resourcesRequests []ResourceRequest, environment string, application string, zone string) (resources []NaisResource, err error)
	getLoadBalancerConfig(application string, environment string) (*NaisResource, error)
	createApplicationInstance(deploymentRequest NaisDeploymentRequest, fasitEnvironment, subDomain string, exposedResourceIds, usedResourceIds []int) error
}

type FasitResource struct {
	Id           int
	Alias        string
	ResourceType string                 `json:"type"`
	Scope        Scope                  `json:"scope"`
	Properties   map[string]string
	Secrets      map[string]map[string]string
	Certificates map[string]interface{} `json:"files"`
}

type ResourceRequest struct {
	Alias        string
	ResourceType string
	PropertyMap  map[string]string
}

type NaisResource struct {
	id           int
	name         string
	resourceType string
	scope        Scope
	properties   map[string]string
	propertyMap  map[string]string
	secret       map[string]string
	certificates map[string][]byte
	ingresses    map[string]string
}

func (nr NaisResource) Properties() map[string]string {
	return nr.properties
}

func (nr NaisResource) Secret() map[string]string {
	return nr.secret
}

func (nr NaisResource) ToEnvironmentVariable(property string) string {
	return strings.ToUpper(nr.ToResourceVariable(property))
}

func (nr NaisResource) ToResourceVariable(property string) string {
	if value, ok := nr.propertyMap[property]; ok {
		property = value
	} else if nr.resourceType != "applicationproperties" {
		property = nr.name + "_" + property
	}

	return strings.ToLower(nr.normalizePropertyName(property))
}

func (nr NaisResource) normalizePropertyName(name string) string {
	if strings.Contains(name, ".") {
		name = strings.Replace(name, ".", "_", -1)
	}

	if strings.Contains(name, ":") {
		name = strings.Replace(name, ":", "_", -1)
	}

	if strings.Contains(name, "-") {
		name = strings.Replace(name, "-", "_", -1)
	}

	return name
}

func (fasit FasitClient) GetScopedResources(resourcesRequests []ResourceRequest, environment string, application string, zone string) (resources []NaisResource, err error) {
	for _, request := range resourcesRequests {
		resource, appErr := fasit.getScopedResource(request, environment, application, zone)
		if appErr != nil {
			return []NaisResource{}, fmt.Errorf("unable to get resource %s (%s). %s", request.Alias, request.ResourceType, appErr)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (fasit FasitClient) createApplicationInstance(deploymentRequest NaisDeploymentRequest, fasitEnvironment, subDomain string, exposedResourceIds, usedResourceIds []int) error {
	fasitPath := fasit.FasitUrl + "/api/v2/applicationinstances/"

	payload, err := json.Marshal(buildApplicationInstancePayload(deploymentRequest, fasitEnvironment, subDomain, exposedResourceIds, usedResourceIds))
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return fmt.Errorf("unable to create payload (%s)", err)
	}

	glog.Infof("ApplicationInstancePayload: %s", payload)
	req, err := http.NewRequest("POST", fasitPath, bytes.NewBuffer(payload))
	req.SetBasicAuth(deploymentRequest.Username, deploymentRequest.Password)
	req.Header.Set("Content-Type", "application/json")
	if deploymentRequest.OnBehalfOf != "" {
		glog.Infof("I am setting onbehalfofheader to: %s", deploymentRequest.OnBehalfOf)
		req.Header.Set("x-onbehalfof", deploymentRequest.OnBehalfOf)
	}

	_, appErr := fasit.doRequest(req)
	if appErr != nil {
		return appErr
	}
	return nil
}

func (fasit FasitClient) getLoadBalancerConfig(application string, environment string) (*NaisResource, error) {
	req, err := fasit.buildRequest("GET", "/api/v2/resources", map[string]string{
		"environment": environment,
		"application": application,
		"type":        "LoadBalancerConfig",
	})

	body, appErr := fasit.doRequest(req)
	if appErr != nil {
		return nil, err
	}

	ingresses, err := parseLoadBalancerConfig(body)
	if err != nil {
		return nil, err
	}

	if len(ingresses) == 0 {
		return nil, nil
	}

	//todo UGh...
	return &NaisResource{
		name:         "",
		properties:   nil,
		resourceType: "LoadBalancerConfig",
		certificates: nil,
		secret:       nil,
		ingresses:    ingresses,
	}, nil

}

func CreateOrUpdateFasitResources(fasit FasitClientAdapter, resources []ExposedResource, hostname, fasitEnvironmentClass, fasitEnvironment string, deploymentRequest NaisDeploymentRequest) ([]int, error) {
	var exposedResourceIds []int

	for _, resource := range resources {
		var request = ResourceRequest{Alias: resource.Alias, ResourceType: resource.ResourceType}
		existingResource, appError := fasit.getScopedResource(request, fasitEnvironment, deploymentRequest.Application, deploymentRequest.Zone)

		if appError != nil {
			if appError.Code() == 404 {
				// Create new resource if none was found
				createdResourceId, err := fasit.createResource(resource, fasitEnvironmentClass, fasitEnvironment, hostname, deploymentRequest)
				if err != nil {
					return nil, fmt.Errorf("failed creating resource: %s of type %s with path %s. (%s)", resource.Alias, resource.ResourceType, resource.Path, err)
				}
				exposedResourceIds = append(exposedResourceIds, createdResourceId)
			} else {
				// Failed contacting Fasit
				return nil, appError
			}

		} else {
			// Updating Fasit resource
			updatedResourceId, err := fasit.updateResource(existingResource, resource, fasitEnvironmentClass, fasitEnvironment, hostname, deploymentRequest)
			if err != nil {
				return nil, fmt.Errorf("failed updating resource: %s of type %s with path %s. (%s)", resource.Alias, resource.ResourceType, resource.Path, err)
			}
			exposedResourceIds = append(exposedResourceIds, updatedResourceId)

		}
	}
	return exposedResourceIds, nil
}

func getResourceIds(usedResources []NaisResource) (usedResourceIds []int) {
	for _, resource := range usedResources {
		if resource.resourceType != "LoadBalancerConfig" {
			usedResourceIds = append(usedResourceIds, resource.id)
		}
	}
	return usedResourceIds
}

func FetchFasitResources(fasit FasitClientAdapter, application string, environment string, zone string, usedResources []UsedResource) (naisresources []NaisResource, err error) {
	resourceRequests := DefaultResourceRequests()

	for _, resource := range usedResources {
		resourceRequests = append(resourceRequests, ResourceRequest{
			Alias:        resource.Alias,
			ResourceType: resource.ResourceType,
			PropertyMap:  resource.PropertyMap,
		})
	}

	naisresources, err = fasit.GetScopedResources(resourceRequests, environment, application, zone)
	if err != nil {
		return naisresources, err
	}

	if lbResource, e := fasit.getLoadBalancerConfig(application, environment); e == nil {
		if lbResource != nil {
			naisresources = append(naisresources, *lbResource)
		}
	} else {
		glog.Warning("failed getting loadbalancer config for application %s in environment %s: %s ", application, environment, e)
	}

	return naisresources, nil

}
func arrayToString(a []int) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", ",", -1), "[]")
}

// Updates Fasit with information
func updateFasit(fasit FasitClientAdapter, deploymentRequest NaisDeploymentRequest, usedResources []NaisResource, manifest NaisManifest, hostname, fasitEnvironmentClass, fasitEnvironment, domain string) error {

	usedResourceIds := getResourceIds(usedResources)
	var exposedResourceIds []int
	var err error

	if len(manifest.FasitResources.Exposed) > 0 {
		if len(hostname) == 0 {
			return fmt.Errorf("unable to create resources when no ingress nor loadbalancer is specified")
		}
		exposedResourceIds, err = CreateOrUpdateFasitResources(fasit, manifest.FasitResources.Exposed, hostname, fasitEnvironmentClass, fasitEnvironment, deploymentRequest)
		if err != nil {
			return err
		}
	}

	glog.Infof("exposed: %s\nused: %s", arrayToString(exposedResourceIds), arrayToString(usedResourceIds))

	if err := fasit.createApplicationInstance(deploymentRequest, fasitEnvironment, domain, exposedResourceIds, usedResourceIds); err != nil {
		return err
	}

	return nil
}

func (fasit FasitClient) doRequest(r *http.Request) ([]byte, AppError) {
	requestCounter.With(nil).Inc()

	client := &http.Client{}
	resp, err := client.Do(r)

	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return []byte{}, appError{err, "Error contacting fasit", http.StatusInternalServerError}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorCounter.WithLabelValues("read_body").Inc()
		return []byte{}, appError{err, "Could not read body", http.StatusInternalServerError}
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode == 404 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return []byte{}, appError{nil, fmt.Sprintf("item not found in Fasit: %s", string(body)), http.StatusNotFound}
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return []byte{}, appError{nil, fmt.Sprintf("error contacting Fasit: %s", string(body)), resp.StatusCode}
	}

	return body, nil

}
func (fasit FasitClient) getScopedResource(resourcesRequest ResourceRequest, fasitEnvironment, application, zone string) (NaisResource, AppError) {
	req, err := fasit.buildRequest("GET", "/api/v2/scopedresource", map[string]string{
		"alias":       resourcesRequest.Alias,
		"type":        resourcesRequest.ResourceType,
		"environment": fasitEnvironment,
		"application": application,
		"zone":        zone,
	})

	if err != nil {
		return NaisResource{}, appError{err, "unable to create request", 500}
	}

	body, appErr := fasit.doRequest(req)
	if appErr != nil {
		return NaisResource{}, appErr
	}

	var fasitResource FasitResource

	err = json.Unmarshal(body, &fasitResource)
	if err != nil {
		errorCounter.WithLabelValues("unmarshal_body").Inc()
		return NaisResource{}, appError{err, "could not unmarshal body", 500}
	}

	resource, err := fasit.mapToNaisResource(fasitResource, resourcesRequest.PropertyMap)
	if err != nil {
		return NaisResource{}, appError{err, "unable to map response to Nais resource", 500}
	}
	return resource, nil
}

func SafeMarshal(v interface{}) ([]byte, error) {
	/*	String values encode as JSON strings coerced to valid UTF-8, replacing invalid bytes with the Unicode replacement rune.
		The angle brackets "<" and ">" are escaped to "\u003c" and "\u003e" to keep some browsers from misinterpreting JSON output as HTML.
		Ampersand "&" is also escaped to "\u0026" for the same reason. This escaping can be disabled using an Encoder that had SetEscapeHTML(false) called on it.	*/
	b, err := json.Marshal(v)
	b = bytes.Replace(b, []byte("\\u0026"), []byte("&"), -1)
	return b, err
}
func (fasit FasitClient) createResource(resource ExposedResource, fasitEnvironmentClass, environment, hostname string, deploymentRequest NaisDeploymentRequest) (int, error) {
	payload, err := SafeMarshal(buildResourcePayload(resource, NaisResource{}, fasitEnvironmentClass, environment, deploymentRequest.Zone, hostname))
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return 0, fmt.Errorf("unable to create payload (%s)", err)
	}

	req, err := http.NewRequest("POST", fasit.FasitUrl+"/api/v2/resources/", bytes.NewBuffer(payload))
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return 0, fmt.Errorf("unable to create request: %s", err)
	}

	req.SetBasicAuth(deploymentRequest.Username, deploymentRequest.Password)
	req.Header.Set("Content-Type", "application/json")
	if deploymentRequest.OnBehalfOf != "" {
		req.Header.Set("x-onbehalfof", deploymentRequest.OnBehalfOf)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return 0, fmt.Errorf("unable to contact Fasit: %s", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "POST").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return 0, fmt.Errorf("fasit returned: %s (%s)", body, strconv.Itoa(resp.StatusCode))
	}

	location := strings.Split(resp.Header.Get("Location"), "/")
	id, err := strconv.Atoi(location[len(location)-1])
	if err != nil {
		return 0, fmt.Errorf("didn't receive a valid resource ID from Fasit: %s", err)
	}

	return id, nil
}
func (fasit FasitClient) updateResource(existingResource NaisResource, resource ExposedResource, fasitEnvironmentClass, environment, hostname string, deploymentRequest NaisDeploymentRequest) (int, error) {
	requestCounter.With(nil).Inc()

	payload, err := SafeMarshal(buildResourcePayload(resource, existingResource, fasitEnvironmentClass, environment, deploymentRequest.Zone, hostname))
	glog.Infof("Updating resource with the following payload: %s", payload)
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return 0, fmt.Errorf("unable to create payload (%s)", err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/v2/resources/%d", fasit.FasitUrl, existingResource.id), bytes.NewBuffer(payload))
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return 0, fmt.Errorf("unable to create request: %s", err)
	}
	glog.Infof("Putting to: %s/api/v2/resources/%d", fasit.FasitUrl, existingResource.id)
	req.SetBasicAuth(deploymentRequest.Username, deploymentRequest.Password)
	req.Header.Set("Content-Type", "application/json")
	if deploymentRequest.OnBehalfOf != "" {
		req.Header.Set("x-onbehalfof", deploymentRequest.OnBehalfOf)
	}

	_, appErr := fasit.doRequest(req)
	if appErr != nil {
		return 0, appErr
	}

	return existingResource.id, nil
}

func (fasit FasitClient) GetFasitEnvironment(environmentName string) (string, error) {
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitUrl+"/api/v2/environments/"+environmentName, nil)
	if err != nil {
		return "", fmt.Errorf("could not create request: %s", err)
	}

	resp, appErr := fasit.doRequest(req)
	if appErr != nil {
		return "", appErr
	}

	type FasitEnvironment struct {
		EnvironmentClass string `json:"environmentclass"`
	}
	var fasitEnvironment FasitEnvironment
	if err := json.Unmarshal(resp, &fasitEnvironment); err != nil {
		return "", fmt.Errorf("unable to read environmentclass from response: %s", err)
	}

	return fasitEnvironment.EnvironmentClass, nil
}

func (fasit FasitClient) GetFasitApplication(application string) error {
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitUrl+"/api/v2/applications/"+application, nil)
	if err != nil {
		return fmt.Errorf("could not create request: %s", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return fmt.Errorf("unable to contact Fasit: %s", err)
	}
	defer resp.Body.Close()

	if err != nil {
		return fmt.Errorf("error contacting Fasit: %s", err)
	}

	if resp.StatusCode == 200 {
		return nil
	}
	return fmt.Errorf("could not find application %s in Fasit", application)
}

func (fasit FasitClient) mapToNaisResource(fasitResource FasitResource, propertyMap map[string]string) (resource NaisResource, err error) {
	resource.name = fasitResource.Alias
	resource.resourceType = fasitResource.ResourceType
	resource.properties = fasitResource.Properties
	resource.propertyMap = propertyMap
	resource.id = fasitResource.Id
	resource.scope = fasitResource.Scope

	if len(fasitResource.Secrets) > 0 {
		secret, err := resolveSecret(fasitResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return NaisResource{}, fmt.Errorf("unable to resolve secret: %s", err)
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
		for _, line := range strings.Split(fasitResource.Properties["applicationProperties"], "\n") {
			line = strings.TrimSpace(line)

			if len(line) > 0 {
				parts := strings.SplitN(line, "=", 2)
				resource.properties[parts[0]] = parts[1]
			}
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

func parseLoadBalancerConfig(config []byte) (map[string]string, error) {
	jsn, err := gabs.ParseJSON(config)
	if err != nil {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return nil, fmt.Errorf("error parsing load balancer config: %s ", config)
	}

	ingresses := make(map[string]string)
	lbConfigs, _ := jsn.Children()
	if len(lbConfigs) == 0 {
		return nil, nil
	}

	for _, lbConfig := range lbConfigs {
		host, found := lbConfig.Path("properties.url").Data().(string)
		if !found {
			glog.Warning("no host found for loadbalancer config: %s", lbConfig)
			continue
		}
		path, _ := lbConfig.Path("properties.contextRoots").Data().(string)
		ingresses[host] = path
	}

	if len(ingresses) == 0 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return nil, fmt.Errorf("no loadbalancer config found for: %s", config)
	}
	return ingresses, nil
}

func parseFilesObject(files map[string]interface{}) (fileName string, fileUrl string, e error) {
	jsn, err := gabs.Consume(files)
	if err != nil {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return "", "", fmt.Errorf("error parsing fasit json: %s ", files)
	}

	fileName, fileNameFound := jsn.Path("keystore.filename").Data().(string)
	if !fileNameFound {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return "", "", fmt.Errorf("error parsing fasit json. Filename not found: %s ", files)
	}

	fileUrl, fileUrlfound := jsn.Path("keystore.ref").Data().(string)
	if !fileUrlfound {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return "", "", fmt.Errorf("error parsing fasit json. Fileurl not found: %s ", files)
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
		return map[string]string{}, fmt.Errorf("error contacting fasit when resolving secret: %s", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		if requestDump, e := httputil.DumpRequest(req, false); e == nil {
			glog.Errorf("Fasit request: ", requestDump)
		}
		return map[string]string{}, fmt.Errorf("fasit gave error message when resolving secret: %s (HTTP %v)", body, strconv.Itoa(resp.StatusCode))
	}

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

func (fasit FasitClient) environmentNameFromNamespaceBuilder(namespace, clustername string) string {
	re := regexp.MustCompile(`^[utqp][0-9]*$`)

	if namespace == "default" || len(namespace) == 0 {
		return clustername
	} else if !re.MatchString(namespace) {
		return namespace + "-" + clustername
	}
	return namespace
}

func generateScope(resource ExposedResource, existingResource NaisResource, fasitEnvironmentClass, environment, zone string) Scope {
	if resource.AllZones {
		return Scope{
			EnvironmentClass: fasitEnvironmentClass,
			Environment:      environment,
		}
	}
	if existingResource.id > 0 {
		return existingResource.scope
	}
	return Scope{
		EnvironmentClass: fasitEnvironmentClass,
		Environment:      environment,
		Zone:             zone,
	}
}

func buildApplicationInstancePayload(deploymentRequest NaisDeploymentRequest, fasitEnvironment, subDomain string, exposedResourceIds, usedResourceIds []int) ApplicationInstancePayload {
	// Need to make an empty array of Resources in order for json.Marshall to return [] and not null
	// see https://danott.co/posts/json-marshalling-empty-slices-to-empty-arrays-in-go.html for details
	emptyResources := make([]Resource, 0)
	domain := strings.Join(strings.Split(subDomain, ".")[1:], ".")
	applicationInstancePayload := ApplicationInstancePayload{
		Application:      deploymentRequest.Application,
		Environment:      fasitEnvironment,
		Version:          deploymentRequest.Version,
		ClusterName:      "nais",
		Domain:           domain,
		ExposedResources: emptyResources,
		UsedResources:    emptyResources,
	}
	if len(exposedResourceIds) > 0 {
		for _, id := range exposedResourceIds {
			applicationInstancePayload.ExposedResources = append(applicationInstancePayload.ExposedResources, Resource{id})
		}
	}
	if len(usedResourceIds) > 0 {
		for _, id := range usedResourceIds {
			applicationInstancePayload.UsedResources = append(applicationInstancePayload.UsedResources, Resource{id})
		}
	}

	return applicationInstancePayload
}

func buildResourcePayload(resource ExposedResource, existingResource NaisResource, fasitEnvironmentClass, fasitEnvironment, zone, hostname string) ResourcePayload {
	// Reference of valid resources in Fasit
	// ['DataSource', 'MSSQLDataSource', 'DB2DataSource', 'LDAP', 'BaseUrl', 'Credential', 'Certificate', 'OpenAm', 'Cics', 'RoleMapping', 'QueueManager', 'WebserviceEndpoint', 'RestService', 'WebserviceGateway', 'EJB', 'Datapower', 'EmailAddress', 'SMTPServer', 'Queue', 'Topic', 'DeploymentManager', 'ApplicationProperties', 'MemoryParameters', 'LoadBalancer', 'LoadBalancerConfig', 'FileLibrary', 'Channel
	if strings.EqualFold("restservice", resource.ResourceType) {
		return RestResourcePayload{
			Type:  "RestService",
			Alias: resource.Alias,
			Properties: RestProperties{
				Url:         "https://" + hostname + resource.Path,
				Description: resource.Description,
			},
			Scope: generateScope(resource, existingResource, fasitEnvironmentClass, fasitEnvironment, zone),
		}

	} else if strings.EqualFold("WebserviceEndpoint", resource.ResourceType) {
		Url, _ := url.Parse("http://maven.adeo.no/nexus/service/local/artifact/maven/redirect")
		q := url.Values{}
		q.Add("r", "m2internal")
		q.Add("g", resource.WsdlGroupId)
		q.Add("a", resource.WsdlArtifactId)
		q.Add("v", resource.WsdlVersion)
		q.Add("e", "zip")
		Url.RawQuery = q.Encode()

		return WebserviceResourcePayload{
			Type:  "WebserviceEndpoint",
			Alias: resource.Alias,
			Properties: WebserviceProperties{
				EndpointUrl:   "https://" + hostname + resource.Path,
				WsdlUrl:       Url.String(),
				SecurityToken: resource.SecurityToken,
				Description:   resource.Description,
			},
			Scope: generateScope(resource, existingResource, fasitEnvironmentClass, fasitEnvironment, zone),
		}
	} else {
		return nil
	}
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
