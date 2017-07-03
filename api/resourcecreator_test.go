package api

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/util/intstr"
	"testing"
)

func TestService(t *testing.T) {

	appName := "appname"
	nameSpace := "namesspace"
	targetPort := 234
	port := 123
	clusterIp := "11.22.33.44"
	resourceVersion := "sdfrdd"
	image := "docker.hub/app"
	version := "13"
	newVersion := "14"

	appConfig := defaaultAppConfig(appName, image, port, targetPort)

	req := DeploymentRequest{
		Application:  appName,
		Version:      version,
		Environment:  nameSpace,
		AppConfigUrl: ""}

	svc := defaultService(appName, nameSpace, resourceVersion, clusterIp, port)
	deployment := defaultDeployment(appName, nameSpace, image, port, version)
	ingress := createDefaultIngress(appName, nameSpace)

	r := ResourceCreator{appConfig, req}

	t.Run("AValidDeploymentRequestAndAppConfigCreatesANewService", func(t *testing.T) {
		service := *r.CreateService()

		assert.Equal(t, appName, service.ObjectMeta.Name)
		assert.Equal(t, int32(targetPort), service.Spec.Ports[0].TargetPort.IntVal)
		assert.Equal(t, map[string]string{"app": appName}, service.Spec.Selector)
	})
	t.Run("AValidServiceCanBeUpdated", func(t *testing.T) {
		service := *r.UpdateService(*svc)

		assert.Equal(t, appName, service.ObjectMeta.Name)
		assert.Equal(t, resourceVersion, service.ObjectMeta.ResourceVersion)
		assert.Equal(t, int32(targetPort), service.Spec.Ports[0].TargetPort.IntVal)
		assert.Equal(t, map[string]string{"app": appName}, service.Spec.Selector)
	})

	t.Run("AValidDeploymentRequestAndAppConfigCreatesANewDeployment", func(t *testing.T) {
		deployment := *r.CreateDeployment([]NaisResource{})

		assert.Equal(t, appName, deployment.Name)
		assert.Equal(t, appName, deployment.Spec.Template.Name)
		assert.Equal(t, appName, deployment.Spec.Template.Spec.Containers[0].Name)
		assert.Equal(t, image+":"+version, deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, int32(port), deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, version, deployment.Spec.Template.Spec.Containers[0].Env[0].Value)
	})

	t.Run("AValidDeploymentCanBeUpdated", func(t *testing.T) {
		r.DeploymentRequest.Version = newVersion

		updatedDeployment := *r.UpdateDeployment(deployment, []NaisResource{})

		assert.Equal(t, appName, updatedDeployment.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].Name)
		assert.Equal(t, image+":"+newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, int32(port), updatedDeployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Env[0].Value)
	})
	t.Run("AValidDeploymentRequestAndAppConfigCreatesANewIngress", func(t *testing.T) {
		ingress := ResourceCreator{AppConfig: appConfig, DeploymentRequest: req}.CreateIngress()

		assert.Equal(t, appName, ingress.ObjectMeta.Name)
		assert.Equal(t, appName+".nais.devillo.no", ingress.Spec.Rules[0].Host)
		assert.Equal(t, appName, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
	})

	t.Run("AValidIngressCanBeUpdated", func(t *testing.T) {
		updatedIngress := ResourceCreator{AppConfig: appConfig, DeploymentRequest: req}.updateIngress(ingress)

		assert.Equal(t, appName, updatedIngress.ObjectMeta.Name)
		assert.Equal(t, appName+".nais.devillo.no", updatedIngress.Spec.Rules[0].Host)
		assert.Equal(t, appName, updatedIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), updatedIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
	})

}
func defaaultAppConfig(appName string, image string, port int, targetPort int) AppConfig {
	return AppConfig{
		[]Container{
			{
				Name:  appName,
				Image: image,
				Ports: []Port{
					{
						Name:       "portname",
						Port:       port,
						Protocol:   "http",
						TargetPort: targetPort,
					},
				},
			},
		},[]Resource{},
	}
}
func defaultService(appName string, nameSpace string, resourceVersion string, clusterIp string, port int) *v1.Service {
	return &v1.Service{TypeMeta: unversioned.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	},
		ObjectMeta: v1.ObjectMeta{
			Name:            appName,
			Namespace:       nameSpace,
			ResourceVersion: resourceVersion,
		},
		Spec: v1.ServiceSpec{
			Type:      v1.ServiceTypeClusterIP,
			ClusterIP: clusterIp,
			Selector:  map[string]string{"app": appName},
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
}
func defaultDeployment(appName string, namespace string, image string, port int, version string) *v1beta1.Deployment {
	return &v1beta1.Deployment{
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
								{ContainerPort: int32(port), Protocol: v1.ProtocolTCP},
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
}
func createDefaultIngress(appName string, nameSpace string) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      appName,
			Namespace: nameSpace,
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
}
