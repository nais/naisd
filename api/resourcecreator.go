package api

import (
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/util/intstr"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/api/resource"
	"fmt"
)

type ResourceCreator struct {
	AppConfig         AppConfig
	DeploymentRequest DeploymentRequest
}

func (r ResourceCreator) CreateService() *v1.Service {
	appName := r.DeploymentRequest.Application

	return &v1.Service{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": appName},
			Ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(r.AppConfig.Containers[0].Ports[0].Port),
					},
				},
			},
		},
	}
}

func (r ResourceCreator) CreateDeployment() *v1beta1.Deployment {
	appName := r.DeploymentRequest.Application

	return &v1beta1.Deployment{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
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
							Name:  r.AppConfig.Containers[0].Name,
							Image: fmt.Sprintf("%s:%s", r.AppConfig.Containers[0].Image, r.DeploymentRequest.Version),
							Ports: []v1.ContainerPort{
								{ContainerPort: int32(r.AppConfig.Containers[0].Ports[0].Port), Protocol: v1.ProtocolTCP},
							},
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("100m"),
									v1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							Env: []v1.EnvVar{{
								Name: "app_version",
								Value: r.DeploymentRequest.Version,
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

func (r ResourceCreator) CreateIngress() *v1beta1.Ingress {
	appName := r.DeploymentRequest.Application

	return &v1beta1.Ingress{
		TypeMeta: unversioned.TypeMeta{
			Kind: "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: fmt.Sprintf("%s.nais.devillo.no", appName),
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

func int32p(i int32) *int32 {
	return &i
}