package api

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"gopkg.in/h2non/gock.v1"
)

func TestGettingResource(t *testing.T) {

	alias := "alias1"
	resourceType := "datasource"
	environment := "environment"
	application := "application"
	zone := "zone"

	fasit := FasitAdapter{"https://fasit.basta.no"}


	defer gock.Off()
	gock.New("https://fasit.basta.no").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse.json")

	resource, err := fasit.getResource(alias, resourceType, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, alias, resource.Alias)
	assert.Equal(t, resourceType, resource.ResourceType)
	assert.Equal(t, "jdbc:oracle:thin:@//a01dbfl030.adeo.no:1521/basta", resource.Properties.Url)
	assert.Equal(t, "basta", resource.Properties.Username)
	assert.Equal(t, "https://fasit.adeo.no/api/v2/secrets/2586446", resource.Secrets.Password.Ref)

}

func TestGettingListOfResources(t *testing.T) {

	alias := "alias1"
	alias2 := "alias2"
	alias3 := "alias3"

	resourceType := "datasource"
	environment := "environment"
	application := "application"
	zone := "zone"


	fasit := FasitAdapter{"https://fasit.basta.no"}


	defer gock.Off()
	gock.New("https://fasit.basta.no").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse.json")

	gock.New("https://fasit.basta.no").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias2).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse2.json")

	gock.New("https://fasit.basta.no").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias3).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse3.json")

	resources := []FasitResource{}
	resources = append(resources, FasitResource{Alias:alias, ResourceType:resourceType})
	resources = append(resources, FasitResource{Alias:alias2, ResourceType:resourceType})
	resources = append(resources, FasitResource{Alias:alias3, ResourceType:resourceType})

	resourcesReplies, err  := fasit.getResources(resources, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(resourcesReplies))
	assert.Equal(t, alias, resourcesReplies[0].Alias)
	assert.Equal(t, alias2, resourcesReplies[1].Alias)
	assert.Equal(t, alias3, resourcesReplies[2].Alias)


}
