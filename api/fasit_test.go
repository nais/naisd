package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

func TestResourceEnvironmentVariableName(t *testing.T) {
	t.Run("Resource should be underscored and uppercased", func(t *testing.T) {
		resource := NaisResource{
			1,
			"test.resource",
			"type",
			Scope{"u", "u1", ZONE_FSS},
			map[string]string{},
			map[string]string{},
			map[string]string{},
			map[string][]byte{},
			nil,
		}
		assert.Equal(t, "TEST_RESOURCE_KEY", resource.ToEnvironmentVariable("key"))
		assert.Equal(t, "test_resource_key", resource.ToResourceVariable("key"))

		resource = NaisResource{
			1,
			"test.resource",
			"applicationproperties",
			Scope{"u", "u1", ZONE_FSS},
			map[string]string{
				"foo.var-with.mixed_stuff": "fizz",
			},
			map[string]string{},
			map[string]string{},
			map[string][]byte{},
			nil,
		}
		assert.Equal(t, "FOO_VAR_WITH_MIXED_STUFF", resource.ToEnvironmentVariable("foo.var-with.mixed_stuff"))
		assert.Equal(t, "foo_var_with_mixed_stuff", resource.ToResourceVariable("foo.var-with.mixed_stuff"))
	})

	t.Run("Property mapping should decide variable name", func(t *testing.T) {
		resource := NaisResource{
			1,
			"test.resource",
			"applicationproperties",
			Scope{"u", "u1", ZONE_FSS},
			map[string]string{
				"foo.var-with.mixed_stuff": "fizz",
			},
			map[string]string{
				"foo.var-with.mixed_stuff": "SOMETHING_NEW",
			},
			map[string]string{},
			map[string][]byte{},
			nil,
		}
		assert.Equal(t, "SOMETHING_NEW", resource.ToEnvironmentVariable("foo.var-with.mixed_stuff"))
		assert.Equal(t, "something_new", resource.ToResourceVariable("foo.var-with.mixed_stuff"))

		resource = NaisResource{
			1,
			"test.resource",
			"datasource",
			Scope{"u", "u1", ZONE_FSS},
			map[string]string{
				"url":      "fizzbuzz",
				"username": "fizz",
				"password": "buzz",
			},
			map[string]string{
				"username": "DB_USER",
				"password": "DB_PW",
			},
			map[string]string{},
			map[string][]byte{},
			nil,
		}
		assert.Equal(t, "TEST_RESOURCE_URL", resource.ToEnvironmentVariable("url"))
		assert.Equal(t, "test_resource_url", resource.ToResourceVariable("url"))
		assert.Equal(t, "DB_USER", resource.ToEnvironmentVariable("username"))
		assert.Equal(t, "db_user", resource.ToResourceVariable("username"))
		assert.Equal(t, "DB_PW", resource.ToEnvironmentVariable("password"))
		assert.Equal(t, "db_pw", resource.ToResourceVariable("password"))
	})
}

func TestGettingResource(t *testing.T) {
	alias := "alias1"
	resourceType := "datasource"
	environment := "environment"
	application := "application"
	zone := "zone"

	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse.json")

	resource, err := fasit.getScopedResource(ResourceRequest{alias, resourceType, map[string]string{"username": "DB_USER"}}, environment, application, zone)

	assert.Nil(t, err)
	assert.Equal(t, alias, resource.name)
	assert.Equal(t, 848186, resource.id)
	assert.Equal(t, resourceType, resource.resourceType)
	assert.Equal(t, "jdbc:oracle:thin:@//a01dbfl030.adeo.no:1521/basta", resource.properties["url"])
	assert.Equal(t, "basta", resource.properties["username"])
	assert.Equal(t, "DB_USER", resource.propertyMap["username"])
	assert.Equal(t, "p", resource.scope.EnvironmentClass)
}

func TestCreatingApplicationInstance(t *testing.T) {

	defer gock.Off()

	gock.New("https://fasit.local").
		Post("/api/v2/applicationinstances").
		HeaderPresent("Authorization").
		MatchHeader("Content-Type", "application/json").
		Reply(201).
		BodyString("aiit")

	fasit := FasitClient{"https://fasit.local", "", ""}
	exposedResourceIds, usedResourceIds := []int{1, 2, 3}, []int{4, 5, 6}
	deploymentRequest := NaisDeploymentRequest{Application: "app", Environment: "env", Version: "123"}

	t.Run("A valid payload creates ApplicationInstance", func(t *testing.T) {
		err := fasit.createApplicationInstance(deploymentRequest, "", "", exposedResourceIds, usedResourceIds)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})
}
func TestCreatingResource(t *testing.T) {
	environment := "environment"
	class := "u"
	hostname := "hostname"
	id := 4242
	exposedResource := ExposedResource{
		Alias:        "alias1",
		ResourceType: "RestService",
		Path:         "",
	}
	deploymentRequest := NaisDeploymentRequest{
		Application: "application",
		Zone:        "zone",
	}

	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()

	t.Run("createResource returns error if fasit is unreachable", func(t *testing.T) {
		_, err := fasit.createResource(exposedResource, class, environment, hostname, deploymentRequest)
		assert.Error(t, err)
	})
	gock.New("https://fasit.local").
		Post("/api/v2/resources").
		HeaderPresent("Authorization").
		MatchHeader("Content-Type", "application/json").
		Reply(201).
		SetHeader("Location", fmt.Sprintf("http://localhost:8089/v2/resources/%d", id))

	t.Run("createResource returns ID when called", func(t *testing.T) {
		createdResourceId, err := fasit.createResource(exposedResource, class, environment, hostname, deploymentRequest)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
		assert.Equal(t, id, createdResourceId)
	})
	gock.New("https://fasit.local").
		Post("/api/v2/resources").
		HeaderPresent("Authorization").
		Reply(501).
		BodyString("bish")
	t.Run("createResource errs when Fasit fails", func(t *testing.T) {
		createdResourceId, err := fasit.createResource(exposedResource, class, environment, hostname, deploymentRequest)
		assert.Error(t, err)
		assert.Equal(t, 0, createdResourceId)
	})
}

func TestUpdateResource(t *testing.T) {
	environment := "environment"
	class := "u"
	hostname := "hostname"
	exposedResource := ExposedResource{
		Alias:        "alias1",
		ResourceType: "RestService",
		Path:         "",
	}
	deploymentRequest := NaisDeploymentRequest{
		Application: "application",
		Zone:        "zone",
	}
	naisResource := NaisResource{id: 4242}

	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()

	t.Run("updateResource returns error if fasit is unreachable", func(t *testing.T) {
		_, err := fasit.updateResource(naisResource, exposedResource, class, environment, hostname, deploymentRequest)
		assert.Error(t, err)
	})
	gock.New("https://fasit.local").
		Put("/api/v2/resources/"+fmt.Sprint(naisResource.id)).
		HeaderPresent("Authorization").
		MatchHeader("Content-Type", "application/json").
		Reply(200)

	t.Run("updateResource returns ID when called", func(t *testing.T) {
		createdResourceId, err := fasit.updateResource(naisResource, exposedResource, class, environment, hostname, deploymentRequest)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
		assert.Equal(t, naisResource.id, createdResourceId)
	})
	gock.New("https://fasit.local").
		Put("/api/v2/resources/"+fmt.Sprint(naisResource.id)).
		HeaderPresent("Authorization").
		HeaderPresent("x-onbehalfof").
		MatchHeader("Content-Type", "application/json").
		Reply(200)

	t.Run("x-onbehalfof header not set when no OnBehalfOf flag is present", func(t *testing.T) {
		createdResourceId, _ := fasit.updateResource(naisResource, exposedResource, class, environment, hostname, deploymentRequest)
		assert.False(t, gock.IsDone())
		assert.Equal(t, 0, createdResourceId)
	})
	t.Run("OnBehalfOf flag sets x-onbehalfof header", func(t *testing.T) {
		deploymentRequest.OnBehalfOf = "username"
		createdResourceId, _ := fasit.updateResource(naisResource, exposedResource, class, environment, hostname, deploymentRequest)
		assert.True(t, gock.IsDone())
		assert.Equal(t, naisResource.id, createdResourceId)
	})
	gock.New("https://fasit.local").
		Put("/api/v2/resources/"+fmt.Sprint(naisResource.id)).
		HeaderPresent("Authorization").
		MatchHeader("Content-Type", "application/json").
		Reply(501).
		BodyString("bish")
	t.Run("updateResource errs when Fasit fails", func(t *testing.T) {
		createdResourceId, err := fasit.updateResource(naisResource, exposedResource, class, environment, hostname, deploymentRequest)
		assert.Error(t, err)
		assert.Equal(t, 0, createdResourceId)
	})

}

func TestGetLoadBalancerConfig(t *testing.T) {
	environment := "environment"
	application := "application"

	fasit := FasitClient{"https://fasit.local", "", ""}

	t.Run("Get load balancer config happy path", func(t *testing.T) {

		defer gock.Off()
		gock.New("https://fasit.local").
			Get("/api/v2/resources").
			MatchParam("environment", environment).
			MatchParam("application", application).
			MatchParam("type", "LoadBalancerConfig").
			Reply(200).File("testdata/fasitLbConfigResponse.json")

		resource, err := fasit.getLoadBalancerConfig("application", "environment")

		assert.NoError(t, err)
		assert.Equal(t, 2, len(resource.ingresses))
		assert.Equal(t, "LoadBalancerConfig", resource.resourceType)
	})

}
func TestGetResourceId(t *testing.T) {
	naisResources := []NaisResource{{id: 1}, {id: 2}, {id: 0, resourceType: "LoadBalancerConfig"}}
	resourceIds := getResourceIds(naisResources)
	assert.Equal(t, []int{1, 2}, resourceIds)
}

type FakeFasitClient struct {
	FasitUrl string
	FasitClient
}

func (fasit FakeFasitClient) getScopedResource(resourcesRequest ResourceRequest, environment, application, zone string) (NaisResource, AppError) {
	switch application {
	case "notfound":
		return NaisResource{}, appError{fmt.Errorf("not found"), "Resource not found in Fasit", 404}
	case "fasitError":
		return NaisResource{}, appError{fmt.Errorf("error from fasit"), "random error", 500}
	default:
		return NaisResource{id: 1}, nil
	}
}

func (fasit FakeFasitClient) createResource(resource ExposedResource, fasitEnvironmentClass, environment, hostname string, deploymentRequest NaisDeploymentRequest) (int, error) {
	switch deploymentRequest.Zone {
	case "failed":
		return 0, fmt.Errorf("random error")
	default:
		return 4242, nil
	}
}

var updateCalled bool

func (fasit FakeFasitClient) updateResource(existingResource NaisResource, resource ExposedResource, fasitEnvironmentClass, environment, hostname string, deploymentRequest NaisDeploymentRequest) (int, error) {
	updateCalled = true
	switch deploymentRequest.Zone {
	case "failed":
		return 0, fmt.Errorf("random error")
	default:
		return 1, nil

	}
}

var createApplicationInstanceCalled bool

func (fasit FakeFasitClient) createApplicationInstance(deploymentRequest NaisDeploymentRequest, fasitEnvironment, subDomain string, exposedResourceIds, usedResourceIds []int) error {
	createApplicationInstanceCalled = true
	return nil
}

func TestCreateOrUpdateFasitResources(t *testing.T) {

	alias := "alias1"
	resourceType := "RestService"
	environment := "environment"
	class := "u"
	hostname := "bish"

	exposedResource := ExposedResource{
		Alias:        alias,
		ResourceType: resourceType,
		Path:         "",
	}

	deploymentRequest := NaisDeploymentRequest{
		Application: "application",
		Zone:        "zone",
	}
	exposedResources := []ExposedResource{exposedResource, exposedResource}

	fakeFasitClient := FakeFasitClient{}

	// Using application field to identify which response to return from getScopedResource on FakeFasitClient
	t.Run("Resources are created when their resource ID isn't found in Fasit", func(t *testing.T) {
		deploymentRequest.Application = "notfound"
		resourceIds, err := CreateOrUpdateFasitResources(fakeFasitClient, exposedResources, hostname, class, environment, deploymentRequest)
		assert.NoError(t, err)
		assert.Equal(t, []int{4242, 4242}, resourceIds)
	})
	t.Run("Returns an error if contacting Fasit fails", func(t *testing.T) {
		deploymentRequest.Application = "fasitError"
		resourceIds, err := CreateOrUpdateFasitResources(fakeFasitClient, exposedResources, hostname, class, environment, deploymentRequest)
		assert.Error(t, err)
		assert.Nil(t, resourceIds)
		assert.True(t, strings.Contains(err.Error(), "random error: error from fasit (500)"))
	})

	// Using Zone field to identify which response to return from createResource on FakeFasitClient
	t.Run("Returns an error if unable to create resource", func(t *testing.T) {
		deploymentRequest.Application = "notfound"
		deploymentRequest.Zone = "failed"
		resourceIds, err := CreateOrUpdateFasitResources(fakeFasitClient, exposedResources, hostname, class, environment, deploymentRequest)
		assert.Error(t, err)
		assert.Nil(t, resourceIds)
		assert.True(t, strings.Contains(err.Error(), "failed creating resource: alias1 of type RestService with path . (random error)"))
	})
	t.Run("Updates Fasit if resources were found", func(t *testing.T) {
		updateCalled = false
		deploymentRequest.Zone = "zone"
		deploymentRequest.Application = "application"
		resourceIds, err := CreateOrUpdateFasitResources(fakeFasitClient, exposedResources, hostname, class, environment, deploymentRequest)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 1}, resourceIds)
		assert.True(t, updateCalled)
	})
	// Using Zone field to identify which response to return from updateResource on FakeFasitClient
	t.Run("Returns an error if unable to update resource", func(t *testing.T) {
		deploymentRequest.Zone = "failed"
		resourceIds, err := CreateOrUpdateFasitResources(fakeFasitClient, exposedResources, hostname, class, environment, deploymentRequest)
		assert.Error(t, err)
		assert.Nil(t, resourceIds)
		assert.True(t, strings.Contains(err.Error(), "failed updating resource: alias1 of type RestService with path . (random error)"))
	})
}

func TestResourceError(t *testing.T) {
	fasitClient := FasitClient{FasitUrl: "https://fasit.local"}
	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", NavTruststoreFasitAlias).
		Reply(200).File("testdata/fasitResponse.json")

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		Reply(404).BodyString("not found")

	resourceAlias := "resourcealias"
	resourceType := "baseurl"
	resource, err := FetchFasitResources(fasitClient, "app", "env", "123", []UsedResource{{Alias: resourceAlias, ResourceType: resourceType}})
	assert.Error(t, err)
	assert.Empty(t, resource)
	assert.True(t, strings.Contains(err.Error(), fmt.Sprintf("unable to get resource %s (%s)", resourceAlias, resourceType)))
}

func TestUpdateFasit(t *testing.T) {

	alias := "alias"
	resourceType := "restService"
	environment := "environment"
	class := "u"
	application := "application"
	hostname := "bish"
	clustername := "clustername"

	exposedResource := ExposedResource{
		Alias:        alias,
		ResourceType: resourceType,
		Path:         "",
	}

	usedResources := []NaisResource{{id: 1}, {id: 2}}
	exposedResources := []ExposedResource{exposedResource, exposedResource}

	deploymentRequest := NaisDeploymentRequest{
		Application: application,
		Environment: environment,
		Version:     version,
	}

	fakeFasitClient := FakeFasitClient{}
	manifest := NaisManifest{FasitResources: FasitResources{Exposed: exposedResources}}

	t.Run("Calling updateFasit with resources returns no error", func(t *testing.T) {
		createApplicationInstanceCalled = false
		err := updateFasit(fakeFasitClient, deploymentRequest, usedResources, manifest, hostname, class, clustername, "")
		assert.NoError(t, err)
		assert.True(t, createApplicationInstanceCalled)
	})
	t.Run("Calling updateFasit without hostname when you have exposed resources fails", func(t *testing.T) {
		createApplicationInstanceCalled = false
		err := updateFasit(fakeFasitClient, deploymentRequest, usedResources, manifest, "", class, clustername, "")
		assert.Error(t, err)
		assert.False(t, createApplicationInstanceCalled)
	})
	t.Run("Calling updateFasit without hostname when you have no exposed resources works", func(t *testing.T) {
		createApplicationInstanceCalled = false
		manifest.FasitResources.Exposed = nil
		err := updateFasit(fakeFasitClient, deploymentRequest, usedResources, manifest, "", class, clustername, "")
		assert.NoError(t, err)
		assert.True(t, createApplicationInstanceCalled)
	})
}

func TestBuildingFasitPayloads(t *testing.T) {
	application := "appName"
	fasitEnvironment := "t1000"
	subDomain := "nais.devillo.no"
	environment := "t1000"
	class := "t"
	version := "2.1"
	exposedResourceIds := []int{1, 2, 3}
	usedResourceIds := []int{4, 5, 6}
	zone := ZONE_FSS
	alias := "resourceAlias"
	path := "/myPath"
	hostname := "hostname"
	wsdlGroupId := "myGroup"
	wsdlArtifactId := "myArtifactId"
	securityToken := "LDAP"
	allZones := true
	description := "myDescription"
	wsdlPath := fmt.Sprintf("http://maven.adeo.no/nexus/service/local/artifact/maven/redirect?a=%s&e=zip&g=%s&r=m2internal&v=%s", wsdlArtifactId, wsdlGroupId, version)

	deploymentRequest := NaisDeploymentRequest{
		Application: application,
		Environment: environment,
		Version:     version,
	}

	restResource := ExposedResource{
		Alias:        alias,
		ResourceType: "RestService",
		Path:         path,
		Description:  description,
	}
	webserviceResource := ExposedResource{
		Alias:          alias,
		ResourceType:   "WebserviceEndpoint",
		Path:           path,
		WsdlGroupId:    wsdlGroupId,
		WsdlArtifactId: wsdlArtifactId,
		WsdlVersion:    version,
		SecurityToken:  securityToken,
		Description:    description,
	}
	exposedResources := []Resource{Resource{1}, Resource{2}, Resource{3}}
	usedResources := []Resource{Resource{4}, Resource{5}, Resource{6}}

	t.Run("Building ApplicationInstancePayload", func(t *testing.T) {
		payload := buildApplicationInstancePayload(deploymentRequest, fasitEnvironment, subDomain, exposedResourceIds, usedResourceIds)
		assert.Equal(t, application, payload.Application)
		assert.Equal(t, environment, payload.Environment)
		assert.Equal(t, version, payload.Version)
		assert.Equal(t, exposedResources, payload.ExposedResources)
		assert.Equal(t, usedResources, payload.UsedResources)
	})
	t.Run("Marshalling payload with both exposed and used resources works", func(t *testing.T) {
		payload, err := json.Marshal(buildApplicationInstancePayload(deploymentRequest, fasitEnvironment, subDomain, exposedResourceIds, usedResourceIds))
		assert.NoError(t, err)
		n := len(payload)
		assert.Equal(t, "{\"application\":\"appName\",\"environment\":\"t1000\",\"version\":\"2.1\",\"exposedresources\":[{\"id\":1},{\"id\":2},{\"id\":3}],\"usedresources\":[{\"id\":4},{\"id\":5},{\"id\":6}],\"clustername\":\"nais\",\"domain\":\"devillo.no\"}", string(payload[:n]))
	})
	t.Run("Marshalling payload with no exposed resources returns empty array in json", func(t *testing.T) {
		emptyResourceList := []int{}
		payload, err := json.Marshal(buildApplicationInstancePayload(deploymentRequest, fasitEnvironment, subDomain, emptyResourceList, usedResourceIds))
		assert.NoError(t, err)
		n := len(payload)
		assert.Equal(t, "{\"application\":\"appName\",\"environment\":\"t1000\",\"version\":\"2.1\",\"exposedresources\":[],\"usedresources\":[{\"id\":4},{\"id\":5},{\"id\":6}],\"clustername\":\"nais\",\"domain\":\"devillo.no\"}", string(payload[:n]))
	})
	t.Run("Marshalling payload with no used resources returns empty array in json", func(t *testing.T) {
		emptyResourceList := []int{}
		payload, err := json.Marshal(buildApplicationInstancePayload(deploymentRequest, fasitEnvironment, subDomain, exposedResourceIds, emptyResourceList))
		assert.NoError(t, err)
		n := len(payload)
		assert.Equal(t, "{\"application\":\"appName\",\"environment\":\"t1000\",\"version\":\"2.1\",\"exposedresources\":[{\"id\":1},{\"id\":2},{\"id\":3}],\"usedresources\":[],\"clustername\":\"nais\",\"domain\":\"devillo.no\"}", string(payload[:n]))
	})
	t.Run("Building RestService ResourcePayload", func(t *testing.T) {
		payloadReturn := buildResourcePayload(restResource, NaisResource{}, class, environment, zone, hostname)
		payload, _ := payloadReturn.(RestResourcePayload)
		assert.Equal(t, "RestService", payload.Type)
		assert.Equal(t, alias, payload.Alias)
		assert.Equal(t, "https://"+hostname+path, payload.Properties.Url)
		assert.Equal(t, description, payload.Properties.Description)
		assert.Equal(t, environment, payload.Scope.Environment)
		assert.Equal(t, zone, payload.Scope.Zone)
	})
	t.Run("Marshalling restResource payloads yields expected result", func(t *testing.T) {
		payload, err := json.Marshal(buildResourcePayload(restResource, NaisResource{}, class, environment, zone, hostname))
		assert.NoError(t, err)
		n := len(payload)
		assert.Equal(t, "{\"alias\":\"resourceAlias\",\"scope\":{\"environmentclass\":\"t\",\"environment\":\"t1000\",\"zone\":\"fss\"},\"type\":\"RestService\",\"properties\":{\"url\":\"https://hostname/myPath\",\"description\":\"myDescription\"}}", string(payload[:n]))
	})
	t.Run("Building WebserviceEndpoint ResourcePayload", func(t *testing.T) {
		payloadReturn := buildResourcePayload(webserviceResource, NaisResource{}, class, environment, zone, hostname)
		payload, _ := payloadReturn.(WebserviceResourcePayload)
		assert.Equal(t, "WebserviceEndpoint", payload.Type)
		assert.Equal(t, alias, payload.Alias)
		assert.Equal(t, wsdlPath, payload.Properties.WsdlUrl)
		assert.Equal(t, description, payload.Properties.Description)
		assert.Equal(t, environment, payload.Scope.Environment)
		assert.Equal(t, zone, payload.Scope.Zone)
	})
	t.Run("Marshalling Webservice payloads yields expected result", func(t *testing.T) {

		payload, err := json.Marshal(buildResourcePayload(webserviceResource, NaisResource{}, class, environment, zone, hostname))
		payload = bytes.Replace(payload, []byte("\\u0026"), []byte("&"), -1)
		assert.NoError(t, err)
		n := len(payload)
		assert.Equal(t, "{\"alias\":\"resourceAlias\",\"scope\":{\"environmentclass\":\"t\",\"environment\":\"t1000\",\"zone\":\"fss\"},\"type\":\"WebserviceEndpoint\",\"properties\":{\"endpointUrl\":\"https://hostname/myPath\",\"wsdlUrl\":\"http://maven.adeo.no/nexus/service/local/artifact/maven/redirect?a=myArtifactId&e=zip&g=myGroup&r=m2internal&v=2.1\",\"securityToken\":\"LDAP\",\"description\":\"myDescription\"}}", string(payload[:n]))
	})
	t.Run("Building RestService ResourcePayload with AllZones returns wider scope", func(t *testing.T) {
		restResource.AllZones = allZones
		payloadReturn := buildResourcePayload(restResource, NaisResource{}, class, environment, zone, hostname)
		payload, _ := payloadReturn.(RestResourcePayload)
		assert.Equal(t, environment, payload.Scope.Environment)
		assert.Empty(t, payload.Scope.Zone)
	})
}

func TestGenerateScope(t *testing.T) {
	resource := ExposedResource{AllZones: false}
	existingResource := NaisResource{}
	fasitEnvironmentClass := "u"
	environment := "u1"
	zone := ZONE_FSS
	t.Run("default scope set when creating a resource", func(t *testing.T) {
		scope := generateScope(resource, existingResource, fasitEnvironmentClass, environment, zone)
		defaultScope := Scope{EnvironmentClass: fasitEnvironmentClass, Environment: environment, Zone: zone}
		assert.Equal(t, defaultScope, scope)

	})
	t.Run("All Zones returns wide scope", func(t *testing.T) {
		resource.AllZones = true
		scope := generateScope(resource, existingResource, fasitEnvironmentClass, environment, zone)
		allZoneScope := Scope{EnvironmentClass: fasitEnvironmentClass, Environment: environment}
		assert.Equal(t, allZoneScope, scope)
	})
	t.Run("Updating a resource uses scope from existing resource", func(t *testing.T) {
		existingResource.scope = Scope{EnvironmentClass: fasitEnvironmentClass, Zone: zone}
		existingResource.id = 2
		resource.AllZones = false
		scope := generateScope(resource, existingResource, fasitEnvironmentClass, environment, zone)
		jsn, _ := json.Marshal(scope)
		n := len(jsn)
		assert.Equal(t, existingResource.scope, scope)
		assert.Equal(t, "{\"environmentclass\":\"u\",\"zone\":\"fss\"}", string(jsn[:n]))
	})

}

func TestGetFasitEnvironment(t *testing.T) {
	namespace := "namespace"

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/environments/" + namespace).
		Reply(200).
		JSON(map[string]string{"environmentclass": "u"})

	fasit := FasitClient{"https://fasit.local", "", ""}
	t.Run("Returns an error if environment isn't found", func(t *testing.T) {
		_, err := fasit.GetFasitEnvironment("notExisting")
		assert.Error(t, err)
		assert.False(t, gock.IsDone())
	})
	t.Run("Returns no error if environment is found", func(t *testing.T) {
		_, err := fasit.GetFasitEnvironment(namespace)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

}
func TestGetFasitApplication(t *testing.T) {
	application := "appname"

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/applications/" + application).
		Reply(200).
		BodyString("anything")

	fasit := FasitClient{"https://fasit.local", "", ""}

	t.Run("Returns err if application isn't found", func(t *testing.T) {
		err := fasit.GetFasitApplication("Nonexistant")
		assert.Error(t, err)
		assert.False(t, gock.IsDone())
	})

	t.Run("Returns no error if application is found", func(t *testing.T) {
		err := fasit.GetFasitApplication(application)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

}
func TestGettingListOfResources(t *testing.T) {
	alias := "alias1"
	alias2 := "alias2"
	alias3 := "alias3"
	alias4 := "alias4"

	resourceType := "datasource"
	environment := "environment"
	application := "application"
	zone := "zone"

	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse.json")

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias2).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse2.json")

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias3).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse3.json")

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias4).
		MatchParam("type", "applicationproperties").
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse4.json")

	resources := []ResourceRequest{}
	resources = append(resources, ResourceRequest{alias, resourceType, nil})
	resources = append(resources, ResourceRequest{alias2, resourceType, nil})
	resources = append(resources, ResourceRequest{alias3, resourceType, nil})
	resources = append(resources, ResourceRequest{alias4, "applicationproperties", nil})

	resourcesReplies, err := fasit.GetScopedResources(resources, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, 4, len(resourcesReplies))
	assert.Equal(t, alias, resourcesReplies[0].name)
	assert.Equal(t, alias2, resourcesReplies[1].name)
	assert.Equal(t, alias3, resourcesReplies[2].name)
	assert.Equal(t, alias4, resourcesReplies[3].name)
	assert.Equal(t, 3, len(resourcesReplies[3].properties))
	assert.Equal(t, "value1", resourcesReplies[3].properties["key1"])
	assert.Equal(t, "dc=preprod,dc=local", resourcesReplies[3].properties["key2"])
	assert.Equal(t, "value3", resourcesReplies[3].properties["key3"])
}

func TestResourceWithArbitraryPropertyKeys(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", "alias").
		Reply(200).File("testdata/fasitResponse-arbitrary-keys.json")

	resource, appError := fasit.getScopedResource(ResourceRequest{"alias", "DataSource", nil}, "dev", "app", "zone")
	assert.Nil(t, appError)

	assert.Equal(t, "1", resource.properties["a"])
	assert.Equal(t, "2", resource.properties["b"])
	assert.Equal(t, "3", resource.properties["c"])
}

func TestResolvingSecret(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		fasit := FasitClient{"https://fasit.local", "", ""}

		defer gock.Off()
		gock.New("https://fasit.local").
			Get("/api/v2/scopedresource").
			MatchParam("alias", "aliaset").
			Reply(200).File("testdata/response-with-secret.json")

		gock.New("https://fasit.adeo.no").
			Get("/api/v2/secrets/696969").
			HeaderPresent("Authorization").
			Reply(200).BodyString("hemmelig")

		resource, appError := fasit.getScopedResource(ResourceRequest{"aliaset", "DataSource", nil}, "dev", "app", "zone")

		assert.Nil(t, appError)

		assert.Equal(t, "1", resource.properties["a"])
		assert.Equal(t, "hemmelig", resource.secret["password"])
	})

	t.Run("Unauthorized to get secret", func(t *testing.T) {
		fasit := FasitClient{"https://fasit.local", "", ""}

		defer gock.Off()
		gock.New("https://fasit.local").
			Get("/api/v2/scopedresource").
			MatchParam("alias", "aliaset").
			Reply(200).File("testdata/response-with-secret.json")

		gock.New("https://fasit.adeo.no").
			Get("/api/v2/secrets/696969").
			HeaderPresent("Authorization").
			Reply(401).BodyString("no access")

		_, appError := fasit.getScopedResource(ResourceRequest{"aliaset", "DataSource", nil}, "dev", "app", "zone")

		assert.NotNil(t, appError)
		assert.Contains(t, appError.Error(), "no access", "propagates fasit response to enduser")
	})
}

func TestResolveCertificates(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}

	t.Run("Fetch certificate file for resources of type certificate", func(t *testing.T) {

		defer gock.Off()
		gock.New("https://fasit.local").
			Get("/api/v2/scopedresource").
			MatchParam("alias", "alias").
			Reply(200).File("testdata/fasitCertResponse.json")
		gock.New("https://fasit.adeo.no").
			Get("/api/v2/resources/3024713/file/keystore").
			Reply(200).Body(bytes.NewReader([]byte("Some binary format")))

		resource, appError := fasit.getScopedResource(ResourceRequest{"alias", "Certificate", nil}, "dev", "app", "zone")

		assert.Nil(t, appError)

		assert.Equal(t, "Some binary format", string(resource.certificates["keystore"]))

	})

	t.Run("Ignore non certificate resources with files ", func(t *testing.T) {

		defer gock.Off()
		gock.New("https://fasit.local").
			Get("/api/v2/scopedresource").
			MatchParam("alias", "alias").
			Reply(200).File("testdata/fasitFilesNoCertifcateResponse.json").
			Done()

		resource, appError := fasit.getScopedResource(ResourceRequest{"alias", "Certificate", nil}, "dev", "app", "zone")

		assert.Nil(t, appError)

		assert.Equal(t, 0, len(resource.certificates))

	})

}

func TestParseLoadBalancerConfig(t *testing.T) {
	t.Run("Parse array of load balancer config correctly", func(t *testing.T) {
		b, _ := ioutil.ReadFile("testdata/fasitLbConfigResponse.json")
		result, err := parseLoadBalancerConfig(b)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "ctxroot", result["subdomainwithctxroot.host.tld"])
		assert.Equal(t, "", result["subdomain.host.tld"])
	})

	t.Run("Err if no loadbalancer config is found", func(t *testing.T) {
		_, err := parseLoadBalancerConfig([]byte(`["json1","json2"]`))
		assert.Error(t, err)
	})

	t.Run("Empty map if empty response", func(t *testing.T) {
		result, err := parseLoadBalancerConfig([]byte(`[]`))
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestBuildEnvironmentNameFromNamespace(t *testing.T) {

	fasitClient := FasitClient{}
	clusterName := "testcluster"

	t.Run("'Standard' Fasit environments will return unchanged", func(t *testing.T) {
		namespace := "t1"
		environmentName := fasitClient.environmentNameFromNamespaceBuilder(namespace, clusterName)
		assert.Equal(t, namespace, environmentName)
	})
	t.Run("'Standard' Fasit environments will return unchanged", func(t *testing.T) {
		namespace := "t1000"
		environmentName := fasitClient.environmentNameFromNamespaceBuilder(namespace, clusterName)
		assert.Equal(t, namespace, environmentName)
	})
	t.Run("'Standard' Fasit environments will return unchanged", func(t *testing.T) {
		namespace := "q1"
		environmentName := fasitClient.environmentNameFromNamespaceBuilder(namespace, clusterName)
		assert.Equal(t, namespace, environmentName)
	})
	t.Run("'Standard' Fasit environments will return unchanged", func(t *testing.T) {
		namespace := "p"
		environmentName := fasitClient.environmentNameFromNamespaceBuilder(namespace, clusterName)
		assert.Equal(t, namespace, environmentName)
	})
	t.Run("'default' namespace returns clustername", func(t *testing.T) {
		namespace := "default"
		environmentName := fasitClient.environmentNameFromNamespaceBuilder(namespace, clusterName)
		assert.Equal(t, clusterName, environmentName)
	})
	t.Run("empty namespace returns clustername", func(t *testing.T) {
		namespace := ""
		environmentName := fasitClient.environmentNameFromNamespaceBuilder(namespace, clusterName)
		assert.Equal(t, clusterName, environmentName)
	})
	t.Run("'project' specific namespaces returns clustername + namespace", func(t *testing.T) {
		namespace := "projectX"
		environmentName := fasitClient.environmentNameFromNamespaceBuilder(namespace, clusterName)
		assert.Equal(t, namespace+"-"+clusterName, environmentName)
	})
}

func TestParseFilesObject(t *testing.T) {
	t.Run("Parse filename and fileurl correctly", func(t *testing.T) {
		var jsonMap map[string]interface{}
		json.Unmarshal([]byte(`{
			"keystore": {
				"filename": "keystore",
				"ref": "https://file.url"
			}}`), &jsonMap)
		fileName, fileUrl, err := parseFilesObject(jsonMap)

		assert.NoError(t, err)
		assert.Equal(t, "keystore", fileName)
		assert.Equal(t, "https://file.url", fileUrl)

	})

	t.Run("Err if filename not found ", func(t *testing.T) {
		var jsonMap map[string]interface{}
		json.Unmarshal([]byte(`{
			"keystore": {
				"ref": "https://file.url"
			}}`), &jsonMap)
		_, _, err := parseFilesObject(jsonMap)

		assert.Error(t, err)
	})

	t.Run("Err if fileurl not found ", func(t *testing.T) {
		var jsonMap map[string]interface{}
		json.Unmarshal([]byte(`{
			"keystore": {
				"filename": "keystore",
			}}`), &jsonMap)
		_, _, err := parseFilesObject(jsonMap)

		assert.Error(t, err)
	})
}
