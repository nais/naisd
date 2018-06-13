package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/h2non/gock.v1"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/fake"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type FakeDeployStatusViewer struct {
	deployStatusToReturn DeployStatus
	viewToReturn         DeploymentStatusView
	errToReturn          error
}

func (d FakeDeployStatusViewer) DeploymentStatusView(namespace, deployName, team string) (DeployStatus, DeploymentStatusView, error) {
	return d.deployStatusToReturn, d.viewToReturn, d.errToReturn
}

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

func TestDeployStatusHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/deploystatus/team/deployName/environment", strings.NewReader("whatever"))

	t.Run("Return 404 if deploy status is not found", func(t *testing.T) {
		mux := goji.NewMux()

		api := Api{DeploymentStatusViewer: FakeDeployStatusViewer{
			errToReturn: fmt.Errorf("not Found"),
		}}

		mux.Handle(pat.Get("/deploystatus/:team/:deployName/:environment"), appHandler(api.deploymentStatusHandler))

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Correct http code for a given deploy status", func(t *testing.T) {

		tests := []struct {
			deployStatus     DeployStatus
			expectedHttpCode int
		}{
			{
				Success,
				200,
			},
			{
				Failed,
				500,
			},
			{
				InProgress,
				202,
			},
		}

		for _, test := range tests {
			mux := goji.NewMux()

			api := Api{
				DeploymentStatusViewer: FakeDeployStatusViewer{
					deployStatusToReturn: test.deployStatus,
				},
			}
			mux.Handle(pat.Get("/deploystatus/:team/:deployName/:environment"), appHandler(api.deploymentStatusHandler))

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			assert.Equal(t, test.expectedHttpCode, rr.Code)
		}
	})
}

func TestNoManifestGivesError(t *testing.T) {
	api := Api{}

	manifestUrl := "http://repo.com/app"
	depReq := naisrequest.Deploy{
		Application:      "appname",
		Version:          "",
		FasitEnvironment: "",
		ManifestUrl:      manifestUrl,
		Zone:             "zone",
		Environment:      "environment",
	}

	defer gock.Off()

	gock.New("http://repo.com").
		Get("/app").
		Reply(400).
		JSON(map[string]string{"foo": "bar"})

	jsn, _ := json.Marshal(depReq)

	body := strings.NewReader(string(jsn))

	req, err := http.NewRequest("POST", "/deploy", body)

	if err != nil {
		panic("could not create req")
	}
	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 500, rr.Code)
	assert.Contains(t, string(rr.Body.Bytes()), manifestUrl)
}

func TestValidDeploymentRequestAndManifestCreateResources(t *testing.T) {
	appName := "appname"
	environment := "environment"
	fasitEnvronment := "environmentName"
	image := "name/Container"
	version := "123"
	resourceAlias := "alias1"
	resourceType := "db"
	zone := "zone"

	clientset := fake.NewSimpleClientset()

	api := Api{clientset, "https://fasit.local", "nais.example.tk", "test-cluster", false, nil}

	depReq := naisrequest.Deploy{
		Application:      appName,
		Version:          version,
		FasitEnvironment: fasitEnvronment,
		ManifestUrl:      "http://repo.com/app",
		Zone:             "zone",
		Environment:      environment,
	}

	manifest := NaisManifest{
		Image: image,
		Port:  321,
		Team:  teamName,
		FasitResources: FasitResources{
			Used: []UsedResource{{resourceAlias, resourceType, nil}},
		},
	}
	response := "anything"
	data, _ := yaml.Marshal(manifest)
	appInstanceResponse, _ := yaml.Marshal(response)

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", NavTruststoreFasitAlias).
		Reply(200).File("testdata/fasitTruststoreResponse.json")

	gock.New("https://fasit.local").
		Get("/api/v2/resources/3024713/file/keystore").
		Reply(200).
		BodyString("")

	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", resourceAlias).
		MatchParam("type", resourceType).
		MatchParam("environment", fasitEnvronment).
		MatchParam("application", appName).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse.json")

	gock.New("https://fasit.local").
		Get(fmt.Sprintf("/api/v2/environments/%s", fasitEnvronment)).
		Reply(200).
		JSON(map[string]string{"environmentclass": "u"})

	gock.New("https://fasit.local").
		Get("/api/v2/applications/" + appName).
		Reply(200).
		BodyString("anything")

	gock.New("https://fasit.local").
		Post("/api/v2/applicationinstances/").
		Reply(200).
		BodyString(string(appInstanceResponse))

	jsn, _ := json.Marshal(depReq)

	body := strings.NewReader(string(jsn))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
	assert.Equal(t, "result: \n- created deployment\n- created secret\n- created service\n- created ingress\n- created autoscaler\n- created namespace\n", string(rr.Body.Bytes()))
}

func TestValidDeploymentRequestAndManifestCreateAlerts(t *testing.T) {
	appName := "appname"
	environment := "environment"
	fasitEnvironment := "environmentName"
	image := "name/Container"
	version := "123"
	alertName := "alias1"
	alertExpr := "db"

	clientset := fake.NewSimpleClientset()

	api := Api{clientset, "https://fasit.local", "nais.example.tk", "test-cluster", false, nil}

	depReq := naisrequest.Deploy{
		Application:      appName,
		Version:          version,
		FasitEnvironment: fasitEnvironment,
		ManifestUrl:      "http://repo.com/app",
		Zone:             "zone",
		Environment:      environment,
	}

	manifest := NaisManifest{
		Image: image,
		Port:  321,
		Team:  teamName,
		Alerts: []PrometheusAlertRule{
			{
				Alert: alertName,
				Expr:  alertExpr,
				For:   "5m",
				Annotations: map[string]string{
					"action": "alertAction",
				},
			},
		},
	}

	data, _ := yaml.Marshal(manifest)

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", NavTruststoreFasitAlias).
		Reply(200).File("testdata/fasitTruststoreResponse.json")

	gock.New("https://fasit.local").
		Get("/api/v2/resources/3024713/file/keystore").
		Reply(200).
		BodyString("")

	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))

	jsn, _ := json.Marshal(depReq)

	body := strings.NewReader(string(jsn))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
	assert.Equal(t, "result: \n- created deployment\n- created secret\n- created service\n- created ingress\n- created autoscaler\n- updated alerts configmap (app-rules)\n- created namespace\n", string(rr.Body.Bytes()))
}

func TestThatFasitIsSkippedOnValidDeployment(t *testing.T) {
	appName := "appname"
	environment := "environment"
	image := "name/Container"
	version := "123"
	alertName := "alias1"
	alertExpr := "db"

	clientset := fake.NewSimpleClientset()

	api := Api{clientset, "https://fasit.local", "nais.example.tk", "test-cluster", false, nil}

	depReq := naisrequest.Deploy{
		Application: appName,
		Version:     version,
		ManifestUrl: "http://repo.com/app",
		SkipFasit:   true,
		Zone:        "zone",
		Environment: environment,
	}

	manifest := NaisManifest{
		Image: image,
		Port:  321,
		Team:  teamName,
		Alerts: []PrometheusAlertRule{
			{
				Alert: alertName,
				Expr:  alertExpr,
				For:   "5m",
				Annotations: map[string]string{
					"action": "alertAction",
				},
			},
		},
	}

	data, _ := yaml.Marshal(manifest)

	defer gock.Off()
	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))

	jsn, _ := json.Marshal(depReq)

	body := strings.NewReader(string(jsn))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
	assert.Equal(t, "result: \n- created deployment\n- created service\n- created ingress\n- created autoscaler\n- updated alerts configmap (app-rules)\n- created namespace\n", string(rr.Body.Bytes()))
}

func TestMissingResources(t *testing.T) {
	resourceAlias := "alias1"
	resourceType := "db"

	manifest := NaisManifest{
		Image: "name/Container",
		Port:  321,
		Team:  teamName,
		FasitResources: FasitResources{
			Used: []UsedResource{{resourceAlias, resourceType, nil}},
		},
	}
	data, _ := yaml.Marshal(manifest)

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", NavTruststoreFasitAlias).
		Reply(200).File("testdata/fasitResponse.json")
	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))
	gock.New("https://fasit.local").
		Get("/api/v2/environments/environment").
		Reply(200).
		JSON(map[string]string{"environmentclass": "u"})
	gock.New("https://fasit.local").
		Get("/api/v2/applications/appname").
		Reply(200).
		BodyString("anything")
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		Reply(404)

	req, _ := http.NewRequest("POST", "/deploy", strings.NewReader(CreateDefaultDeploymentRequest()))

	rr := httptest.NewRecorder()
	api := Api{fake.NewSimpleClientset(), "https://fasit.local", "nais.example.tk", "clustername", false, nil}
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 400, rr.Code)
	assert.True(t, gock.IsDone())

	assert.Contains(t, string(rr.Body.Bytes()), fmt.Sprintf("unable to get resource %s (%s)", resourceAlias, resourceType))
}

func CreateDefaultDeploymentRequest() string {
	jsn, _ := json.Marshal(naisrequest.Deploy{
		Application:      "appname",
		Version:          "123",
		FasitEnvironment: "environment",
		ManifestUrl:      "http://repo.com/app",
		Zone:             "zone",
		Environment:      "environment",
	})

	return string(jsn)
}

func TestValidateDeploymentRequest(t *testing.T) {
	t.Run("Empty fields should be marked invalid", func(t *testing.T) {
		invalid := naisrequest.Deploy{
			Application:      "",
			Version:          "",
			FasitEnvironment: "",
			Zone:             "",
			Environment:      "",
			FasitUsername:    "",
			FasitPassword:    "",
		}

		err := invalid.Validate()

		assert.NotNil(t, err)
		assert.Contains(t, err, errors.New("application is required and is empty"))
		assert.Contains(t, err, errors.New("version is required and is empty"))
		assert.Contains(t, err, errors.New("fasitEnvironment is required and is empty"))
		assert.Contains(t, err, errors.New("zone is required and is empty"))
		assert.Contains(t, err, errors.New("zone can only be fss, sbs or iapp"))
		assert.Contains(t, err, errors.New("environment is required and is empty"))
		assert.Contains(t, err, errors.New("fasitUsername is required and is empty"))
		assert.Contains(t, err, errors.New("fasitPassword is required and is empty"))
	})

	t.Run("Fasit properties are not required when Fasit is skipped", func(t *testing.T) {
		invalid := naisrequest.Deploy{
			Application: "",
			Version:     "",
			Zone:        "",
			Environment: "",
			SkipFasit:   true,
		}

		err := invalid.Validate()

		assert.NotNil(t, err)
		assert.Len(t, err, 5)
		assert.Contains(t, err, errors.New("application is required and is empty"))
		assert.Contains(t, err, errors.New("version is required and is empty"))
		assert.Contains(t, err, errors.New("zone is required and is empty"))
		assert.Contains(t, err, errors.New("zone can only be fss, sbs or iapp"))
		assert.Contains(t, err, errors.New("environment is required and is empty"))
	})
}

func TestEnsurePropertyCompatibility(t *testing.T) {
	teamManifest := NaisManifest{Team: teamName}
	t.Run("Should warn when specifying only namespace", func(t *testing.T) {
		deploy := naisrequest.Deploy{
			Application: "application",
			Namespace:   "t1",
		}

		warnings := ensurePropertyCompatibility(&deploy, &teamManifest)
		response := createResponse(DeploymentResult{}, warnings)

		assert.Contains(t, string(response), fmt.Sprintf("Specifying namespace is deprecated. Please adapt your pipelines to use the field 'Environment' instead. For this deploy, as you did not specify 'Environment' I've assumed the previous behaviour and set Environment to '%s' for you.", deploy.Environment))
	})

	t.Run("Should warn when specifying namespace and environment", func(t *testing.T) {
		deploy := naisrequest.Deploy{
			Application: "application",
			Namespace:   "t1",
			Environment: "t1",
		}

		warnings := ensurePropertyCompatibility(&deploy, &teamManifest)
		response := createResponse(DeploymentResult{}, warnings)

		assert.Contains(t, string(response), "Specifying namespace is deprecated and won't make any difference for this deploy. Please adapt your pipelines to only use the field 'Environment'.\n")
	})

	t.Run("Should not warn when not specifying namespace", func(t *testing.T) {
		deploy := naisrequest.Deploy{
			Application: "application",
			Environment: "t1",
		}

		warnings := ensurePropertyCompatibility(&deploy, &teamManifest)
		response := createResponse(DeploymentResult{}, warnings)

		assert.NotContains(t, string(response), "Specifying namespace")
		assert.Len(t, warnings, 0)
	})

	t.Run("Should not warn when specifying team", func(t *testing.T) {
		deploy := naisrequest.Deploy{}

		warnings := ensurePropertyCompatibility(&deploy, &teamManifest)
		response := createResponse(DeploymentResult{}, warnings)

		assert.NotContains(t, string(response), "Starting July 1. (01/07) team name is a mandatory part of the nais manifest. Please update your applications manifest to include 'team: yourTeamName' in order to be able to deploy after July 1.")
		assert.Len(t, warnings, 0)
	})

	t.Run("Should warn when not specifying team", func(t *testing.T) {
		noTeamManifest := NaisManifest{}

		deploy := naisrequest.Deploy{}

		warnings := ensurePropertyCompatibility(&deploy, &noTeamManifest)
		response := createResponse(DeploymentResult{}, warnings)

		assert.Contains(t, string(response), "Starting July 1. (01/07) team name is a mandatory part of the nais manifest. Please update your applications manifest to include 'team: yourTeamName' in order to be able to deploy after July 1.")
	})
}
