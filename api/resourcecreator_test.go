package api

import (
	"testing"
	"fmt"
	"encoding/json"
	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, "appname", service.ObjectMeta.Name)
	assert.Equal(t, int32(123), service.Spec.Ports[0].TargetPort.IntVal)
	assert.Equal(t, map[string]string{"app": "appname"}, service.Spec.Selector)
}
func TestCorrectDeployment(t *testing.T) {
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
		Version:      "latest",
		Environment:  "",
		AppConfigUrl: ""}

	r := ResourceCreator{appConfig, req}

	deployment := *r.CreateDeployment()

	j,_ := json.Marshal(deployment)
	fmt.Println(string(j))

	assert.Equal(t, "appname" ,deployment.Name)
	assert.Equal(t, "appname" ,deployment.Spec.Template.Name)
	assert.Equal(t, "appname" ,deployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "docker.hub/app:latest" ,deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, int32(123) ,deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	assert.Equal(t, "latest" ,deployment.Spec.Template.Spec.Containers[0].Env[0].Value)


}
