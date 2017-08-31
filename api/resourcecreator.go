package api

import (
	"fmt"
	"strings"
	k8sresource "k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	autoscalingv1 "k8s.io/client-go/pkg/apis/autoscaling/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/errors"
)

// Creates a Kubernetes Service object
// If existingServiceId is provided, this is included in object so it can be used to update object
func createServiceDef(targetPort int, existingServiceId, application, namespace string) *v1.Service {
	return &v1.Service{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:            application,
			Namespace:       namespace,
			ResourceVersion: existingServiceId,
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": application},
			Ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(targetPort),
					},
				},
			},
		},
	}
}

func ResourceVariableName(resource NaisResource, key string)  string {
	return strings.Replace(resource.name, ".", "_", -1) + "_" + key
}

// Creates a Kubernetes Deployment object
// If existingDeploymentId is provided, this is included in object so it can be used to update object
func createDeploymentDef(resources []NaisResource, imageName, version string, port int, livenessPath, readinessPath,  existingDeploymentId, application, namespace string) *v1beta1.Deployment {
	deployment := &v1beta1.Deployment{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      application,
			Namespace: namespace,
			ResourceVersion: existingDeploymentId,
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
					Name:   application,
					Labels: map[string]string{"app": application},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  application,
							Image: fmt.Sprintf("%s:%s", imageName, version),
							Ports: []v1.ContainerPort{
								{ContainerPort: int32(port), Protocol: v1.ProtocolTCP},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: livenessPath,
										Port: intstr.FromInt(port),
									},
								},
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: readinessPath,
										Port: intstr.FromInt(port),
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

	containers := deployment.Spec.Template.Spec.Containers
	for _, res := range resources {
		for k, v := range res.properties {
			envVar := v1.EnvVar{ResourceVariableName(res,k), v, nil}
			containers[0].Env = append(containers[0].Env, envVar)
		}
		if res.secret != nil {
			for k := range res.secret {
				variableName := ResourceVariableName(res, k)
				envVar := v1.EnvVar{
					Name: variableName,
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: application,
							},
							Key: variableName,
						},
					},
				}
				containers[0].Env = append(containers[0].Env, envVar)
			}
		}
	}
	return deployment
}

// Creates a Kubernetes Secret object
// If existingSecretId is provided, this is included in object so it can be used to update object
func createSecretDef(resource []NaisResource, existingSecretId, application, namespace string) *v1.Secret {
	secret := &v1.Secret{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      application,
			Namespace: namespace,
			ResourceVersion: existingSecretId,
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

// Creates a Kubernetes Ingress object
// If existingIngressId is provided, this is included in object so it can be used to update object
func createIngressDef(subdomain, existingIngressId, application, namespace string) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      application,
			Namespace: namespace,
			ResourceVersion: existingIngressId,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: fmt.Sprintf("%s.%s", application, subdomain),
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
										ServiceName: application,
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

// Creates a Kubernetes HorizontalPodAutoscaler object
// If existingAutoscalerId is provided, this is included in object so it can be used to update object
func createAutoscalerDef(min, max, cpuTargetPercentage int, existingAutoscalerId, application, namespace string) *autoscalingv1.HorizontalPodAutoscaler {
	return &autoscalingv1.HorizontalPodAutoscaler{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:            application,
			Namespace:       namespace,
			ResourceVersion: existingAutoscalerId,
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			MinReplicas:                    int32p(int32(min)),
			MaxReplicas:                    int32(max),
			TargetCPUUtilizationPercentage: int32p(int32(cpuTargetPercentage)),
		},
	}
}

func createOrUpdateK8sResources(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, resources []NaisResource, clusterSubdomain string, k8sClient kubernetes.Interface) (DeploymentResult, error) {
	var deploymentResult DeploymentResult

	service, err := createOrUpdateService(deploymentRequest, appConfig, k8sClient)

	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating service: %s", err)
	}
	deploymentResult.Service = service

	deployment, err := createOrUpdateDeployment(deploymentRequest, appConfig, resources, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating deployment: %s", err)
	}
	deploymentResult.Deployment = deployment

	secret, err := createOrUpdateSecret(deploymentRequest, resources, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating secret: %s", err)
	}
	deploymentResult.Secret = secret

	ingress, err := createOrUpdateIngress(deploymentRequest, clusterSubdomain, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating ingress: %s", err)
	}
	deploymentResult.Ingress = ingress

	autoscaler, err := createOrUpdateAutoscaler(deploymentRequest, appConfig, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating or updating autoscaler: %s", err)
	}
	deploymentResult.Autoscaler = autoscaler

	return deploymentResult, err
}

func createOrUpdateAutoscaler(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, k8sClient kubernetes.Interface) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	autoscalerId, err := getExistingAutoscalerId(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing autoscaler id: %s", err)
	}

	autoscalerDef := createAutoscalerDef(appConfig.Replicas.Min, appConfig.Replicas.Max, appConfig.Replicas.CpuThresholdPercentage, autoscalerId, deploymentRequest.Application, deploymentRequest.Namespace)
	return createOrUpdateAutoscalerResource(autoscalerDef, deploymentRequest.Namespace, k8sClient)
}

func createOrUpdateIngress(deploymentRequest NaisDeploymentRequest, clusterSubdomain string, k8sClient kubernetes.Interface) (*v1beta1.Ingress, error) {
	existingIngressId, err := getExistingIngressId(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing ingress id: %s", err)
	}

	ingressDef := createIngressDef(clusterSubdomain, existingIngressId, deploymentRequest.Application, deploymentRequest.Namespace)
	return createOrUpdateIngressResource(ingressDef, deploymentRequest.Namespace, k8sClient)
}

func createOrUpdateService(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, k8sClient kubernetes.Interface) (*v1.Service, error) {
	existingServiceId, err := getExistingServiceId(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing service id: %s", err)
	}

	autoscalerDef := createServiceDef(appConfig.Port.TargetPort, existingServiceId, deploymentRequest.Application, deploymentRequest.Namespace)
	return createOrUpdateServiceResource(autoscalerDef, deploymentRequest.Namespace, k8sClient)
}

func createOrUpdateDeployment(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, naisResources []NaisResource, k8sClient kubernetes.Interface) (*v1beta1.Deployment, error) {
	existingDeploymentId, err := getExistingDeploymentId(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing deployment id: %s", err)
	}

	deploymentDef := createDeploymentDef(naisResources, appConfig.Image, deploymentRequest.Version, appConfig.Port.Port, appConfig.Healthcheck.Liveness.Path, appConfig.Healthcheck.Readiness.Path, existingDeploymentId, deploymentRequest.Application, deploymentRequest.Namespace)
	return createOrUpdateDeploymentResource(deploymentDef, deploymentRequest.Namespace, k8sClient)
}

func createOrUpdateSecret(deploymentRequest NaisDeploymentRequest, naisResources []NaisResource, k8sClient kubernetes.Interface) (*v1.Secret, error) {
	existingSecretId, err := getExistingSecretId(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing autoscaler id: %s", err)
	}

	if secretDef := createSecretDef(naisResources, existingSecretId, deploymentRequest.Application, deploymentRequest.Namespace); secretDef != nil {
		return createOrUpdateSecretResource(secretDef, deploymentRequest.Namespace, k8sClient)
	} else {
		return nil, nil
	}
}

func getExistingServiceId(application string, namespace string, k8sClient kubernetes.Interface) (string, error) {
	serviceClient := k8sClient.CoreV1().Services(namespace)
	service, err := serviceClient.Get(application)

	switch {
	case err == nil:
		return service.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingSecretId(application string, namespace string, k8sClient kubernetes.Interface) (string, error) {
	secretClient := k8sClient.CoreV1().Secrets(namespace)
	secret, err := secretClient.Get(application)
	switch {
	case err == nil:
		return secret.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingDeploymentId(application string, namespace string, k8sClient kubernetes.Interface) (string, error) {
	deploymentClient := k8sClient.ExtensionsV1beta1().Deployments(namespace)
	deployment, err := deploymentClient.Get(application)

	switch {
	case err == nil:
		return deployment.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingIngressId(application string, namespace string, k8sClient kubernetes.Interface) (string, error) {
	ingressClient := k8sClient.ExtensionsV1beta1().Ingresses(namespace)
	ingress, err := ingressClient.Get(application)

	switch {
	case err == nil:
		return ingress.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingAutoscalerId(application string, namespace string, k8sClient kubernetes.Interface) (string, error) {
	autoscalerClient := k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace)
	autoscaler, err := autoscalerClient.Get(application)

	switch {
	case err == nil:
		return autoscaler.ObjectMeta.ResourceVersion, err
	case errors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("unexpected error: %s", err)
	}
}

func createOrUpdateAutoscalerResource(autoscalerSpec *autoscalingv1.HorizontalPodAutoscaler, namespace string, k8sClient kubernetes.Interface) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	if autoscalerSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(autoscalerSpec)
	} else {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(autoscalerSpec)
	}
}

func createOrUpdateIngressResource(ingressSpec *v1beta1.Ingress, namespace string, k8sClient kubernetes.Interface) (*v1beta1.Ingress, error) {
	if ingressSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.ExtensionsV1beta1().Ingresses(namespace).Update(ingressSpec)
	} else {
		return k8sClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingressSpec)
	}
}

func createOrUpdateDeploymentResource(deploymentSpec *v1beta1.Deployment, namespace string, k8sClient kubernetes.Interface) (*v1beta1.Deployment, error) {
	if deploymentSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.ExtensionsV1beta1().Deployments(namespace).Update(deploymentSpec)
	} else {
		return k8sClient.ExtensionsV1beta1().Deployments(namespace).Create(deploymentSpec)
	}
}

func createOrUpdateServiceResource(serviceSpec *v1.Service, namespace string, k8sClient kubernetes.Interface) (*v1.Service, error) {
	if serviceSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.CoreV1().Services(namespace).Update(serviceSpec)
	} else {
		return k8sClient.CoreV1().Services(namespace).Create(serviceSpec)
	}
}

func createOrUpdateSecretResource(secretSpec *v1.Secret, namespace string, k8sClient kubernetes.Interface) (*v1.Secret, error) {
	if secretSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.CoreV1().Secrets(namespace).Update(secretSpec)
	} else {
		return k8sClient.CoreV1().Secrets(namespace).Create(secretSpec)
	}
}

func int32p(i int32) *int32 {
	return &i
}
