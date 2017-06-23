package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
)

func TestCreateNewService(t *testing.T) {

	appName := "appname"
	nameSpace := "namesspace"
	port := 234

	appConfig := AppConfig{
		[]Container{
			{
				Name:  appName,
				Image: "docker.hub/app",
				Ports: []Port{
					{
						Name:       "portname",
						Port:       123,
						Protocol:   "http",
						TargetPort: port,
					},
				},
			},
		},
	}

	req := DeploymentRequest{
		Application:  appName,
		Version:      "",
		Environment:  nameSpace,
		AppConfigUrl: ""}

	r := ResourceCreator{appConfig, req}

	service := *r.CreateService()

	assert.Equal(t, "appname", service.ObjectMeta.Name)
	assert.Equal(t, int32(port), service.Spec.Ports[0].TargetPort.IntVal)
	assert.Equal(t, map[string]string{"app": "appname"}, service.Spec.Selector)
}


func TestUpdateService(t *testing.T) {

	appName := "appname"
	nameSpace := "namesspace"
	port := 234
	clusterIp := "11.22.33.44"
	resourceVersion := "sdfrdd"

	appConfig := AppConfig{
		[]Container{
			{
				Name:  appName,
				Image: "docker.hub/app",
				Ports: []Port{
					{
						Name:       "portname",
						Port:       123,
						Protocol:   "http",
						TargetPort: port,
					},
				},
			},
		},
	}

	req := DeploymentRequest{
		Application:  appName,
		Version:      "",
		Environment:  nameSpace,
		AppConfigUrl: ""}


	svc := &v1.Service{ TypeMeta: unversioned.TypeMeta{
	Kind:       "Service",
	APIVersion: "v1",
},
	ObjectMeta: v1.ObjectMeta{
		Name: appName,
		Namespace: nameSpace,
		ResourceVersion: resourceVersion,
	},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			ClusterIP: clusterIp,
			Selector: map[string]string{"app": appName},
			Ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(port),
				},
			},
		},
	},
}


	r := ResourceCreator{appConfig, req}

	service := *r.UpdateService(*svc)

	assert.Equal(t, "appname", service.ObjectMeta.Name)
	assert.Equal(t, resourceVersion, service.ObjectMeta.ResourceVersion)
	assert.Equal(t, int32(port), service.Spec.Ports[0].TargetPort.IntVal)
	assert.Equal(t, int32(port), service.Spec.Ports[0].TargetPort.IntVal)
	assert.Equal(t, map[string]string{"app": appName}, service.Spec.Selector)
}

func TestCreateDeployment(t *testing.T) {
	appName := "appname"
	nameSpace := "namesspace"
	port := 234
	appConfig := AppConfig{
		[]Container{
			{
				Name:  "appname",
				Image: "docker.hub/app",
				Ports: []Port{
					{
						Name:       "portname",
						Port:       port,
						Protocol:   "http",
						TargetPort: 123,
					},
				},
			},
		},
	}

	req := DeploymentRequest{
		Application:  appName,
		Version:      "latest",
		Environment:  nameSpace,
		AppConfigUrl: ""}

	r := ResourceCreator{appConfig, req}

	deployment := *r.CreateDeployment()

	assert.Equal(t, appName ,deployment.Name)
	assert.Equal(t, appName ,deployment.Spec.Template.Name)
	assert.Equal(t, appName ,deployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "docker.hub/app:latest" ,deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, int32(port) ,deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	assert.Equal(t, "latest" ,deployment.Spec.Template.Spec.Containers[0].Env[0].Value)


}
