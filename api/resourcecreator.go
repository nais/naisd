package api

import (
	"fmt"
	k8sresource "k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/util/intstr"
)

type K8sResourceCreator struct {
	AppConfig         NaisAppConfig
	DeploymentRequest NaisDeploymentRequest
}

func (r K8sResourceCreator) UpdateService(existingService v1.Service) *v1.Service {

	serviceSpec := r.CreateService()
	serviceSpec.ObjectMeta.ResourceVersion = existingService.ObjectMeta.ResourceVersion
	serviceSpec.Spec.ClusterIP = existingService.Spec.ClusterIP

	return serviceSpec

}

func (r K8sResourceCreator) CreateService() *v1.Service {

	return &v1.Service{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      r.DeploymentRequest.Application,
			Namespace: r.DeploymentRequest.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": r.DeploymentRequest.Application},
			Ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(r.AppConfig.Port.TargetPort),
					},
				},
			},
		},
	}
}

func (r K8sResourceCreator) UpdateDeployment(exisitingDeployment *v1beta1.Deployment, resource []NaisResource) *v1beta1.Deployment {
	deploymentSpec := r.CreateDeployment(resource)
	deploymentSpec.ObjectMeta.ResourceVersion = exisitingDeployment.ObjectMeta.ResourceVersion
	deploymentSpec.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:%s", r.AppConfig.Image, r.DeploymentRequest.Version)

	return deploymentSpec
}

func (r K8sResourceCreator) CreateDeployment(resource []NaisResource) *v1beta1.Deployment {
	appName := r.DeploymentRequest.Application
	namespace := r.DeploymentRequest.Namespace

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
							Name:  r.AppConfig.Name,
							Image: fmt.Sprintf("%s:%s", r.AppConfig.Image, r.DeploymentRequest.Version),
							Ports: []v1.ContainerPort{
								{ContainerPort: int32(r.AppConfig.Port.Port), Protocol: v1.ProtocolTCP},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: r.AppConfig.Healthcheck.Liveness.Path,
										Port: intstr.FromString(r.AppConfig.Port.Name),
									},
								},
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: r.AppConfig.Healthcheck.Readiness.Path,
										Port: intstr.FromString(r.AppConfig.Port.Name),
									},
								},
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    k8sresource.MustParse("100m"),
									v1.ResourceMemory: k8sresource.MustParse("256Mi"),
								},
							},
							Env: []v1.EnvVar{{
								Name:  "app_version",
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

	for _, res := range resource {
		for k, v := range res.properties {
			envVar := v1.EnvVar{res.name + "_" + k, v, nil}
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, envVar)
		}
		if res.secret != nil {
			for k := range res.secret {
				envVar := v1.EnvVar{
					Name: res.name + "_" + k,
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: r.AppConfig.Name + "-secrets",
							},
							Key: res.name + "_" + k,
						},
					},
				}
				deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, envVar)
			}
		}
	}
	return deployment
}

func (r K8sResourceCreator) CreateSecret(resource []NaisResource) *v1.Secret {
	secret := &v1.Secret{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      r.AppConfig.Name + "-secrets",
			Namespace: r.DeploymentRequest.Namespace,
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	}
	for _, res := range resource {
		if res.secret != nil {
			for k, v := range res.secret {
				secret.Data[res.name+"_"+k] = []byte(v)
			}

		}
	}
	if len(secret.Data) > 0 {
		return secret
	}
	return nil
}

func (r K8sResourceCreator) updateSecret(existingSecret *v1.Secret, resource []NaisResource) *v1.Secret {
	secretSpec := r.CreateSecret(resource)
	secretSpec.ObjectMeta.ResourceVersion = existingSecret.ObjectMeta.ResourceVersion

	return secretSpec
}

func (r K8sResourceCreator) CreateIngress(subdomain string) *v1beta1.Ingress {
	appName := r.DeploymentRequest.Application

	return &v1beta1.Ingress{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      appName,
			Namespace: r.DeploymentRequest.Namespace,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: fmt.Sprintf("%s.%s", appName, subdomain),
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

func (r K8sResourceCreator) updateIngress(existingIngress *v1beta1.Ingress, subdomain string) *v1beta1.Ingress {
	ingressSpec := r.CreateIngress(subdomain)
	ingressSpec.ObjectMeta.ResourceVersion = existingIngress.ObjectMeta.ResourceVersion

	return ingressSpec
}

func int32p(i int32) *int32 {
	return &i
}
