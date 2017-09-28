package api

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/fake"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnIncorrectPayloadGivesError(t *testing.T) {
	api := Api{}

	body := strings.NewReader("gibberish")

	req, err := http.NewRequest("POST", "/deploy", body)

	if err != nil {
		panic("could not create req")
	}
	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 400, rr.Code)
}

func TestNoManifestGivesError(t *testing.T) {
	api := Api{}

	depReq := NaisDeploymentRequest{
		Application:  "appname",
		Version:      "",
		Environment:  "",
		AppConfigUrl: "http://repo.com/app",
		Zone:         "zone",
		Namespace:    "namespace",
	}

	defer gock.Off()

	gock.New("http://repo.com").
		Get("/app").
		Reply(400).
		JSON(map[string]string{"foo": "bar"})

	json, _ := json.Marshal(depReq)

	body := strings.NewReader(string(json))

	req, err := http.NewRequest("POST", "/deploy", body)

	if err != nil {
		panic("could not create req")
	}
	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 500, rr.Code)
}

func TestValidDeploymentRequestAndAppConfigCreateResources(t *testing.T) {
	appName := "appname"
	namespace := "namespace"
	image := "name/Container"
	version := "123"
	resourceAlias := "alias1"
	resourceType := "db"
	zone := "zone"

	clientset := fake.NewSimpleClientset()

	api := Api{clientset, "https://fasit.local", "nais.example.tk"}

	depReq := NaisDeploymentRequest{
		Application:  appName,
		Version:      version,
		Environment:  namespace,
		AppConfigUrl: "http://repo.com/app",
		Zone:         "zone",
		Namespace:    "namespace",
	}

	config := NaisAppConfig{
		Image: image,
		Port:  321,
		FasitResources: FasitResources{
			Used: []UsedResource{{resourceAlias, resourceType}},
		},
	}
	data, _ := yaml.Marshal(config)

	defer gock.Off()

	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", resourceAlias).
		MatchParam("type", resourceType).
		MatchParam("environment", namespace).
		MatchParam("application", appName).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse.json")

	json, _ := json.Marshal(depReq)

	body := strings.NewReader(string(json))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
	assert.Equal(t, "result: \n- created deployment\n- created service\n- created ingress\n- created autoscaler\n", string(rr.Body.Bytes()))
}
