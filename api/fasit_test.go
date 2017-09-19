package api

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"testing"
	"encoding/json"
	"bytes"
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

	resource, err := fasit.getResource(ResourceRequest{alias, resourceType}, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, alias, resource.name)
	assert.Equal(t, resourceType, resource.resourceType)
	assert.Equal(t, "jdbc:oracle:thin:@//a01dbfl030.adeo.no:1521/basta", resource.properties["url"])
	assert.Equal(t, "basta", resource.properties["username"])
}

func TestGettingListOfResources(t *testing.T) {

	alias := "alias1"
	alias2 := "alias2"
	alias3 := "alias3"

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

	resources := []ResourceRequest{}
	resources = append(resources, ResourceRequest{alias, resourceType})
	resources = append(resources, ResourceRequest{alias2, resourceType})
	resources = append(resources, ResourceRequest{alias3, resourceType})

	resourcesReplies, err := fasit.GetResources(resources, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(resourcesReplies))
	assert.Equal(t, alias, resourcesReplies[0].name)
	assert.Equal(t, alias2, resourcesReplies[1].name)
	assert.Equal(t, alias3, resourcesReplies[2].name)
}

func TestResourceWithArbitraryPropertyKeys(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", "alias").
		Reply(200).File("testdata/fasitResponse-arbitrary-keys.json")

	resource, err := fasit.getResource(ResourceRequest{"alias", "DataSource"}, "dev", "app", "zone")

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

	resource, err := fasit.getResource(ResourceRequest{"aliaset", "DataSource"}, "dev", "app", "zone")

	assert.NoError(t, err)

	assert.Equal(t, "1", resource.properties["a"])
	assert.Equal(t, "hemmelig", resource.secret["password"])
}

func TestResolveFile(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", "alias").
		Reply(200).File("testdata/fasitCertResponse.json")
	gock.New("https://fasit.adeo.no").
		Get("/api/v2/resources/3024713/file/keystore").
		Reply(200).Body(bytes.NewReader([]byte("Some binary format")))


	resource, err := fasit.getResource(ResourceRequest{"alias", "Certificate"}, "dev", "app", "zone")

	assert.NoError(t, err)

	assert.Equal(t, "Some binary format", string(resource.files["keystore"]))
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
