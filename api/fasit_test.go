package api

import(
	"github.com/stretchr/testify/assert"
	"testing"
	"gopkg.in/h2non/gock.v1"
)

func TestGettingResource(t *testing.T){

	fasit := FasitAdapter{"https://fasit.basta.no"}

	defer gock.Off()
	gock.New("https://fasit.basta.no").
		Get("/resources/appName").
		Reply(400)


	_, err := fasit.getResource("appName", "db")
	assert.EqualError(t, err, "Fasit gave errormessage: 400" )

}

