package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"encoding/json"
	"gopkg.in/h2non/gock.v1"

)

func TestAnIncorrectPayloadGivesError(t *testing.T) {
	api := Api{}

	body := strings.NewReader("gibberish")

	req, err := http.NewRequest("POST", "/deploy", body)

	if err != nil {
		panic("could not create req")
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.deploy)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 400, rr.Code)
}

func TestAValidDeploymentRequestCreatesResources(t *testing.T) {
	api := Api{}

	depReq := DeploymentRequest{
		Application:  "appname",
		Version:      "",
		Environment:  "",
		AppConfigUrl: "http://repo.com/app",
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
	handler := http.HandlerFunc(api.deploy)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 500, rr.Code)
}
