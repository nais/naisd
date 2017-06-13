package api

import (
	"testing"
	"fmt"
	"encoding/json"
	"reflect"
)

func TestCorrectService(t *testing.T) {

	appConfig := AppConfig{
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

	req := DeploymentRequest{
		Application:  "appname",
		Version:      "",
		Environment:  "",
		AppConfigUrl: ""}

	r := ResourceCreator{appConfig, req}

	service := *r.CreateService()
	j,_ := json.Marshal(service)
	fmt.Println(string(j))

	if service.ObjectMeta.Name != "appname" {
		t.Error("Service name does not match application name")
	}
	if service.Spec.Ports[0].TargetPort.IntVal != 123 {
		t.Error("Target Port does not match")
	}
	if !reflect.DeepEqual( service.Spec.Selector, map[string]string{"app":"appname"}) {
		t.Error("selector does not match ")
	}
}