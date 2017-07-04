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

	resource, err := fasit.getResource(ResourceRequest{alias, resourceType}, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, alias, resource.name)
	assert.Equal(t, resourceType, resource.resourceType)
	assert.Equal(t, "jdbc:oracle:thin:@//a01dbfl030.adeo.no:1521/basta", resource.properties["url"])
	assert.Equal(t, "basta", resource.properties["username"])
	assert.Equal(t, "https://fasit.adeo.no/api/v2/secrets/2586446", resource.secret)

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

	resources := []ResourceRequest{}
	resources = append(resources, ResourceRequest{alias, resourceType})
	resources = append(resources, ResourceRequest{alias2, resourceType})
	resources = append(resources, ResourceRequest{alias3, resourceType})

	resourcesReplies, err  := fasit.getResources(resources, environment, application, zone)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(resourcesReplies))
	assert.Equal(t, alias, resourcesReplies[0].name)
	assert.Equal(t, alias2, resourcesReplies[1].name)
	assert.Equal(t, alias3, resourcesReplies[2].name)


}
