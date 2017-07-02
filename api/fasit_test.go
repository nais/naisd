package api

import(
	"github.com/stretchr/testify/assert"
	"testing"
	"gopkg.in/h2non/gock.v1"
	"os"
)

func TestGettingResource(t *testing.T){

	fasit := FasitAdapter{"https://fasit.basta.no"}

	reader, _ := os.Open("testdata/fasitResponse.json")

	defer gock.Off()
	gock.New("https://fasit.basta.no").
		Get("/api/v2/scopedresource/appName").
		Reply(200).Body(reader)


	resource, err := fasit.getResource("appName", "db", "environemnt", "application", "zone")

	assert.NoError(t, err)
	assert.Equal(t, "bastaDB", resource.Alias)
	assert.Equal(t, "datasource", resource.ResourceType)
	assert.Equal(t, "jdbc:oracle:thin:@//a01dbfl030.adeo.no:1521/basta", resource.Properties.Url)
	assert.Equal(t, "basta", resource.Properties.Username)
	assert.Equal(t, "https://fasit.adeo.no/api/v2/secrets/2586446", resource.Secrets.Password.Ref)

}




