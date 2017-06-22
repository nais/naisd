package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"encoding/json"
	"gopkg.in/h2non/gock.v1"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/client-go/pkg/api/v1"
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

func TestNoManifestGivesError(t *testing.T) {
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

func TestValidDeploymentRequestAndAppConfigCreateResources(t *testing.T){


	//&v1.Service{ObjectMeta: ObjectMeta{Name:appname,GenerateName:,Namespace:t,SelfLink:,UID:,ResourceVersion:,Generation:0,CreationTimestamp:0001-01-01 00:00:00 +0000 UTC,DeletionTimestamp:<nil>,DeletionGracePeriodSeconds:nil,Labels:map[string]string{},Annotations:map[string]string{},OwnerReferences:[],Finalizers:[],ClusterName:,},Spec:ServiceSpec{Ports:[{ TCP 80 {0 123 } 0}],Selector:map[string]string{app: appname,},ClusterIP:,Type:ClusterIP,ExternalIPs:[],DeprecatedPublicIPs:[],SessionAffinity:,LoadBalancerIP:,LoadBalancerSourceRanges:[],ExternalName:,},Status:ServiceStatus{LoadBalancer:LoadBalancerStatus{Ingress:[],},},}


	service :=&v1.Service{ObjectMeta: v1.ObjectMeta{
		Name: "appname",
		Namespace: "t",
	}}

	clientset := fake.NewSimpleClientset(service)

	api := Api{clientset}


	depReq := DeploymentRequest{
		Application:  "appname",
		Version:      "1.1",
		Environment:  "t",
		AppConfigUrl: "http://repo.com/app",
	}

	defer gock.Off()

	config := AppConfig{
		[]Container{
			{
				Name:  "appname",
				Image: "docker.hub/app",
				Ports: []Port{
					{
						Name:       "portname",
						Port:       123,
						Protocol:   "http",
						TargetPort: 321,
					},
				},
			},
		},
	}
	data,_ := yaml.Marshal(config)

	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))

	json,_ := json.Marshal(depReq)

	body := strings.NewReader(string(json))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.deploy)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
}