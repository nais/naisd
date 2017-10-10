package api

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"strings"
	"testing"
)

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

	resource, err := fasit.getScopedResource(ResourceRequest{alias, resourceType}, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, alias, resource.name)
	assert.Equal(t, resourceType, resource.resourceType)
	assert.Equal(t, "jdbc:oracle:thin:@//a01dbfl030.adeo.no:1521/basta", resource.properties["url"])
	assert.Equal(t, "basta", resource.properties["username"])
}
func TestGettingLBConfig(t *testing.T) {

	environment := "environment"
	application := "application"

	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/resources").
		MatchParam("environment", environment).
		MatchParam("application", application).
		Reply(200).File("testdata/fasitLbConfigResponse.json")

	resources, err := fasit.getLoadBalancerConfig("application","environment")

	assert.NoError(t, err)
	assert.Equal(t,len(resources),1)
}

func TestResourceError(t *testing.T) {
	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		Reply(404).BodyString("not found")

	resource, err := fetchFasitResources("https://fasit.local", NaisDeploymentRequest{Application: "app", Environment: "env", Version: "123"}, NaisAppConfig{FasitResources: FasitResources{Used: []UsedResource{{Alias: "resourcealias", ResourceType: "baseurl"}}}})

	assert.Error(t, err)
	assert.Empty(t, resource)
	assert.True(t, strings.Contains(err.Error(), "not found (404)"))
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
	resources = append(resources, ResourceRequest{alias, resourceType})
	resources = append(resources, ResourceRequest{alias2, resourceType})
	resources = append(resources, ResourceRequest{alias3, resourceType})
	resources = append(resources, ResourceRequest{alias4, "applicationproperties"})

	resourcesReplies, err := fasit.GetScopedResources(resources, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, 4, len(resourcesReplies))
	assert.Equal(t, alias, resourcesReplies[0].name)
	assert.Equal(t, alias2, resourcesReplies[1].name)
	assert.Equal(t, alias3, resourcesReplies[2].name)
	assert.Equal(t, alias4, resourcesReplies[3].name)
	assert.Equal(t, 2, len(resourcesReplies[3].properties))
	assert.Equal(t, "value1", resourcesReplies[3].properties["key1"])
	assert.Equal(t, "dc=preprod,dc=local", resourcesReplies[3].properties["key2"])
}

func TestResourceWithArbitraryPropertyKeys(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", "alias").
		Reply(200).File("testdata/fasitResponse-arbitrary-keys.json")

	resource, err := fasit.getScopedResource(ResourceRequest{"alias", "DataSource"}, "dev", "app", "zone")

	assert.NoError(t, err)

	assert.Equal(t, "1", resource.properties["a"])
	assert.Equal(t, "2", resource.properties["b"])
	assert.Equal(t, "3", resource.properties["c"])
}

func TestResolvingSecret(t *testing.T) {
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

	resource, err := fasit.getScopedResource(ResourceRequest{"aliaset", "DataSource"}, "dev", "app", "zone")

	assert.NoError(t, err)

	assert.Equal(t, "1", resource.properties["a"])
	assert.Equal(t, "hemmelig", resource.secret["password"])
}

func TestResolveCertifcates(t *testing.T) {
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

		resource, err := fasit.getScopedResource(ResourceRequest{"alias", "Certificate"}, "dev", "app", "zone")

		assert.NoError(t, err)

		assert.Equal(t, "Some binary format", string(resource.certificates["srvvarseloppgave_cert_keystore"]))

	})

	t.Run("Ignore non certificate resources with files ", func(t *testing.T) {

		defer gock.Off()
		gock.New("https://fasit.local").
			Get("/api/v2/scopedresource").
			MatchParam("alias", "alias").
			Reply(200).File("testdata/fasitFilesNoCertifcateResponse.json").
			Done()

		resource, err := fasit.getScopedResource(ResourceRequest{"alias", "Certificate"}, "dev", "app", "zone")

		assert.NoError(t, err)

		assert.Equal(t, 0, len(resource.certificates))

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
