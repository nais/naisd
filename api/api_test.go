package api

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/util/intstr"
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
	handler := http.HandlerFunc(api.deploy)

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

func TestValidDeploymentRequestAndAppConfigCreateResources(t *testing.T) {
	appName := "appname"
	namespace := "namespace"
	image := "name/Container:latest"
	containerPort := 123
	version := "123"
	resourceAlias := "alias1"
	resourceType := "db"

	service := &v1.Service{ObjectMeta: v1.ObjectMeta{
		Name:      appName,
		Namespace: namespace,
	}}

	deployment := &v1beta1.Deployment{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32p(1),
			Strategy: v1beta1.DeploymentStrategy{
				Type: v1beta1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &v1beta1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(0),
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(1),
					},
				},
			},
			RevisionHistoryLimit: int32p(10),
			Template: v1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:   appName,
					Labels: map[string]string{"app": appName},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  appName,
							Image: image,
							Ports: []v1.ContainerPort{
								{ContainerPort: int32(containerPort)},
							},
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("100m"),
									v1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							Env: []v1.EnvVar{{
								Name:  "app_version",
								Value: version,
							}},
							ImagePullPolicy: v1.PullIfNotPresent,
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
					DNSPolicy:     v1.DNSClusterFirst,
				},
			},
		},
	}

	ingress := &v1beta1.Ingress{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: appName + ".nais.devillo.no",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
										ServiceName: appName,
										ServicePort: intstr.IntOrString{IntVal: 80},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(service, deployment, ingress)

	api := Api{clientset, "https://fasit.local"}

	depReq := NaisDeploymentRequest{
		Application:  appName,
		Version:      version,
		Environment:  namespace,
		AppConfigUrl: "http://repo.com/app",
	}

	config := NaisAppConfig{
		[]Container{
			{
				Name:  appName,
				Image: image,
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
		FasitResources{
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
		MatchParam("zone", "zone").
		Reply(200).File("testdata/fasitResponse.json")


	json, _ := json.Marshal(depReq)

	body := strings.NewReader(string(json))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.deploy)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
}

func TestAppConfigUnmarshal(t *testing.T) {
	const repopath = "https://appconfig.repo"

	gock.New(repopath).
		Reply(200).
		File("testdata/nais.yaml")

	appConfig, err := fetchAppConfig(repopath)

	assert.NoError(t, err)

	assert.Equal(t, "/api", appConfig.FasitResources.Exposed[0].Path)
	assert.Equal(t, "datasource", appConfig.FasitResources.Used[0].ResourceType)
}
