package api

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/errors"
	k8sresource "k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	autoscalingv1 "k8s.io/client-go/pkg/apis/autoscaling/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/util/intstr"
	"strconv"
	"strings"
)

type DeploymentResult struct {
	Autoscaler *autoscalingv1.HorizontalPodAutoscaler
	Ingress    *v1beta1.Ingress
	Deployment *v1beta1.Deployment
	Secret     *v1.Secret
	Service    *v1.Service
}

// Creates a Kubernetes Service object
func createServiceDef(application, namespace string) *v1.Service {
	return &v1.Service{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      application,
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": application},
			Ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: DefaultPortName,
					},
				},
			},
		},
	}

}

func ResourceVariableName(resource NaisResource, key string) string {
	name := resource.name + "_" + key

	if resource.resourceType == "applicationproperties" {
		name = key
	}

	if strings.Contains(name, ".") {
		return strings.Replace(name, ".", "_", -1)
	}

	if strings.Contains(name, ":") {
		return strings.Replace(name, ":", "_", -1)
	}
	return name
}

func ResourceEnvironmentVariableName(resource NaisResource, key string) string {
	return strings.ToUpper(ResourceVariableName(resource, key))
}

func validLabelName(str string) string {
	tmpStr := strings.Replace(str, "_", "-", -1)
	return strings.ToLower(tmpStr)
}

// Creates a Kubernetes Deployment object
// If existingDeployment is provided, this is updated with modifiable fields
func createDeploymentDef(naisResources []NaisResource, appConfig NaisAppConfig, deploymentRequest NaisDeploymentRequest, existingDeployment *v1beta1.Deployment) *v1beta1.Deployment {
	if existingDeployment != nil {
		existingDeployment.Spec.Template.Spec = createPodSpec(deploymentRequest, appConfig, naisResources)
		existingDeployment.Spec.Template.ObjectMeta = createObjectMeta(deploymentRequest, appConfig, existingDeployment.Spec.Template.ObjectMeta.ResourceVersion)
		return existingDeployment
	} else {
		deployment := &v1beta1.Deployment{
			TypeMeta: unversioned.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1beta1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      deploymentRequest.Application,
				Namespace: deploymentRequest.Namespace,
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
				ProgressDeadlineSeconds: int32p(300),
				RevisionHistoryLimit:    int32p(10),
				Template: v1.PodTemplateSpec{
					ObjectMeta: createObjectMeta(deploymentRequest, appConfig, ""),
					Spec:       createPodSpec(deploymentRequest, appConfig, naisResources),
				},
			},
		}
		return deployment
	}

}

func createObjectMeta(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, resourceVersion string) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:   deploymentRequest.Application,
		ResourceVersion: resourceVersion,
		Labels: map[string]string{"app": deploymentRequest.Application},
		Annotations: map[string]string{
			"prometheus.io/scrape": strconv.FormatBool(appConfig.Prometheus.Enabled),
			"prometheus.io/port":   DefaultPortName,
			"prometheus.io/path":   appConfig.Prometheus.Path,
		},
	}
}

func createPodSpec(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, naisResources []NaisResource) v1.PodSpec {
	podSpec := v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  deploymentRequest.Application,
				Image: fmt.Sprintf("%s:%s", appConfig.Image, deploymentRequest.Version),
				Ports: []v1.ContainerPort{
					{ContainerPort: int32(appConfig.Port), Protocol: v1.ProtocolTCP, Name: DefaultPortName},
				},
				Resources: createResourceLimits(appConfig.Resources.Requests.Cpu, appConfig.Resources.Requests.Memory, appConfig.Resources.Limits.Cpu, appConfig.Resources.Limits.Memory),
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						HTTPGet: &v1.HTTPGetAction{
							Path: appConfig.Healthcheck.Liveness.Path,
							Port: intstr.FromString(DefaultPortName),
						},
					},
					InitialDelaySeconds: int32(appConfig.Healthcheck.Liveness.InitialDelay),
				},
				ReadinessProbe: &v1.Probe{
					Handler: v1.Handler{
						HTTPGet: &v1.HTTPGetAction{
							Path: appConfig.Healthcheck.Readiness.Path,
							Port: intstr.FromString(DefaultPortName),
						},
					},
					InitialDelaySeconds: int32(appConfig.Healthcheck.Readiness.InitialDelay),
				},
				Env:             createEnvironmentVariables(deploymentRequest, naisResources),
				ImagePullPolicy: v1.PullIfNotPresent,
			},
		},
		RestartPolicy: v1.RestartPolicyAlways,
		DNSPolicy:     v1.DNSClusterFirst,
	}

	if hasCertificate(naisResources) {
		podSpec.Volumes = append(podSpec.Volumes, createCertificateVolume(deploymentRequest, naisResources))
		container := &podSpec.Containers[0]
		container.VolumeMounts = append(container.VolumeMounts, createCertificateVolumeMount(deploymentRequest, naisResources))
	}

	return podSpec
}

func hasCertificate(naisResources []NaisResource) bool {
	for _, resource := range naisResources {
		if len(resource.certificates) > 0 {
			return true
		}
	}
	return false
}

func createCertificateVolume(deploymentRequest NaisDeploymentRequest, resources []NaisResource) v1.Volume {
	var items []v1.KeyToPath
	for _, res := range resources {
		if res.certificates != nil {
			for k := range res.certificates {
				item := v1.KeyToPath{
					Key:  k,
					Path: k,
				}
				items = append(items, item)
			}
		}
	}

	if len(items) > 0 {
		return v1.Volume{
			Name: validLabelName(deploymentRequest.Application),
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: deploymentRequest.Application,
					Items:      items,
				},
			},
		}
	}

	return v1.Volume{}
}

func createCertificateVolumeMount(deploymentRequest NaisDeploymentRequest, resources []NaisResource) v1.VolumeMount {
	for _, res := range resources {
		if res.certificates != nil {
			return v1.VolumeMount{
				Name:      validLabelName(deploymentRequest.Application),
				MountPath: "/var/run/secrets/naisd.io/",
			}
		}
	}
	return v1.VolumeMount{}
}

func createEnvironmentVariables(deploymentRequest NaisDeploymentRequest, naisResources []NaisResource) []v1.EnvVar {
	envVars := createDefaultEnvironmentVariables(deploymentRequest.Version)

	for _, res := range naisResources {
		for variableName, v := range res.properties {
			envVar := v1.EnvVar{ResourceEnvironmentVariableName(res, variableName), v, nil}
			envVars = append(envVars, envVar)
		}
		if res.secret != nil {
			for k := range res.secret {
				envVar := v1.EnvVar{
					Name: ResourceEnvironmentVariableName(res, k),
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: deploymentRequest.Application,
							},
							Key: ResourceVariableName(res, k),
						},
					},
				}
				envVars = append(envVars, envVar)
			}
		}
	}
	return envVars
}

func createDefaultEnvironmentVariables(version string) []v1.EnvVar {
	return []v1.EnvVar{{
		Name:  "APP_VERSION",
		Value: version,
	}}
}

func createResourceLimits(requestsCpu string, requestsMemory string, limitsCpu string, limitsMemory string) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    k8sresource.MustParse(requestsCpu),
			v1.ResourceMemory: k8sresource.MustParse(requestsMemory),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    k8sresource.MustParse(limitsCpu),
			v1.ResourceMemory: k8sresource.MustParse(limitsMemory),
		},
	}
}

// Creates a Kubernetes Secret object
// If existingSecretId is provided, this is included in object so it can be used to update object
func createSecretDef(naisResources []NaisResource, existingSecret *v1.Secret, application, namespace string) *v1.Secret {
	if existingSecret != nil {
		existingSecret.Data = createSecretData(naisResources)
		return existingSecret
	} else {
		secret := &v1.Secret{
			TypeMeta: unversioned.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      application,
				Namespace: namespace,
			},
			Data: createSecretData(naisResources),
			Type: "Opaque",
		}

		if len(secret.Data) > 0 {
			return secret
		}
		return nil
	}
}

func createSecretData(naisResources []NaisResource) map[string][]byte {
	data := map[string][]byte{}
	for _, res := range naisResources {
		if res.secret != nil {
			for k, v := range res.secret {
				data[res.name+"_"+k] = []byte(v)
			}
		}
		if res.certificates != nil {
			for k, v := range res.certificates {
				data[k] = v
			}
		}
	}
	return data
}

// Creates a Kubernetes Ingress object
func createIngressDef(subdomain, application, namespace string) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      application,
			Namespace: namespace,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: createIngressHostname(application, namespace, subdomain),
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

func createIngressHostname(application, namespace, subdomain string) string {
	if namespace == "default" {
		return fmt.Sprintf("%s.%s", application, subdomain)
	} else {
		return fmt.Sprintf("%s-%s.%s", application, namespace, subdomain)
	}
}

// Creates a Kubernetes HorizontalPodAutoscaler object
// If existingAutoscaler is provided, this is updated with provided parameters
func createOrUpdateAutoscalerDef(min, max, cpuTargetPercentage int, existingAutoscaler *autoscalingv1.HorizontalPodAutoscaler, application, namespace string) *autoscalingv1.HorizontalPodAutoscaler {
	if existingAutoscaler != nil {
		existingAutoscaler.Spec = createAutoscalerSpec(min, max, cpuTargetPercentage, application)

		return existingAutoscaler
	} else {

		return &autoscalingv1.HorizontalPodAutoscaler{
			TypeMeta: unversioned.TypeMeta{
				Kind:       "HorizontalPodAutoscaler",
				APIVersion: "autoscaling/v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      application,
				Namespace: namespace,
			},
			Spec: createAutoscalerSpec(min, max, cpuTargetPercentage, application),
		}
	}
}

func createAutoscalerSpec(min, max, cpuTargetPercentage int, application string) autoscalingv1.HorizontalPodAutoscalerSpec {
	return autoscalingv1.HorizontalPodAutoscalerSpec{
		MinReplicas:                    int32p(int32(min)),
		MaxReplicas:                    int32(max),
		TargetCPUUtilizationPercentage: int32p(int32(cpuTargetPercentage)),
		ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
			Name:       application,
		},
	}
}

func createOrUpdateK8sResources(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, resources []NaisResource, clusterSubdomain string, k8sClient kubernetes.Interface) (DeploymentResult, error) {
	var deploymentResult DeploymentResult

	service, err := createService(deploymentRequest, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating service: %s", err)
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

	ingress, err := createIngress(deploymentRequest, clusterSubdomain, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("Failed while creating ingress: %s", err)
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
	autoscaler, err := getExistingAutoscaler(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing autoscaler: %s", err)
	}

	autoscalerDef := createOrUpdateAutoscalerDef(appConfig.Replicas.Min, appConfig.Replicas.Max, appConfig.Replicas.CpuThresholdPercentage, autoscaler, deploymentRequest.Application, deploymentRequest.Namespace)
	return createOrUpdateAutoscalerResource(autoscalerDef, deploymentRequest.Namespace, k8sClient)
}

// Returns nil,nil if ingress already exists. No reason to do update, as nothing can change
func createIngress(deploymentRequest NaisDeploymentRequest, clusterSubdomain string, k8sClient kubernetes.Interface) (*v1beta1.Ingress, error) {
	existingIngress, err := getExistingIngress(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing ingress id: %s", err)
	}

	if existingIngress != nil {
		return nil, nil // we have done nothing
	}

	ingressDef := createIngressDef(clusterSubdomain, deploymentRequest.Application, deploymentRequest.Namespace)
	return createIngressResource(ingressDef, deploymentRequest.Namespace, k8sClient)
}

func createService(deploymentRequest NaisDeploymentRequest, k8sClient kubernetes.Interface) (*v1.Service, error) {
	existingService, err := getExistingService(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing service: %s", err)
	}

	if existingService != nil {
		return nil, nil // we have done nothing
	}

	serviceDef := createServiceDef(deploymentRequest.Application, deploymentRequest.Namespace)
	return createServiceResource(serviceDef, deploymentRequest.Namespace, k8sClient)
}

func createOrUpdateDeployment(deploymentRequest NaisDeploymentRequest, appConfig NaisAppConfig, naisResources []NaisResource, k8sClient kubernetes.Interface) (*v1beta1.Deployment, error) {
	existingDeployment, err := getExistingDeployment(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing deployment: %s", err)
	}

	deploymentDef := createDeploymentDef(naisResources, appConfig, deploymentRequest, existingDeployment)
	return createOrUpdateDeploymentResource(deploymentDef, deploymentRequest.Namespace, k8sClient)
}

func createOrUpdateSecret(deploymentRequest NaisDeploymentRequest, naisResources []NaisResource, k8sClient kubernetes.Interface) (*v1.Secret, error) {
	existingSecret, err := getExistingSecret(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("Unable to get existing secret: %s", err)
	}

	if secretDef := createSecretDef(naisResources, existingSecret, deploymentRequest.Application, deploymentRequest.Namespace); secretDef != nil {
		return createOrUpdateSecretResource(secretDef, deploymentRequest.Namespace, k8sClient)
	} else {
		return nil, nil
	}
}

func getExistingService(application string, namespace string, k8sClient kubernetes.Interface) (*v1.Service, error) {
	serviceClient := k8sClient.CoreV1().Services(namespace)
	service, err := serviceClient.Get(application)

	switch {
	case err == nil:
		return service, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingSecret(application string, namespace string, k8sClient kubernetes.Interface) (*v1.Secret, error) {
	secretClient := k8sClient.CoreV1().Secrets(namespace)
	secret, err := secretClient.Get(application)
	switch {
	case err == nil:
		return secret, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingDeployment(application string, namespace string, k8sClient kubernetes.Interface) (*v1beta1.Deployment, error) {
	deploymentClient := k8sClient.ExtensionsV1beta1().Deployments(namespace)
	deployment, err := deploymentClient.Get(application)

	switch {
	case err == nil:
		return deployment, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingIngress(application string, namespace string, k8sClient kubernetes.Interface) (*v1beta1.Ingress, error) {
	ingressClient := k8sClient.ExtensionsV1beta1().Ingresses(namespace)
	ingress, err := ingressClient.Get(application)

	switch {
	case err == nil:
		return ingress, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingAutoscaler(application string, namespace string, k8sClient kubernetes.Interface) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	autoscalerClient := k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace)
	autoscaler, err := autoscalerClient.Get(application)

	switch {
	case err == nil:
		return autoscaler, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func createOrUpdateAutoscalerResource(autoscalerSpec *autoscalingv1.HorizontalPodAutoscaler, namespace string, k8sClient kubernetes.Interface) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	if autoscalerSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(autoscalerSpec)
	} else {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(autoscalerSpec)
	}
}

func createIngressResource(ingressSpec *v1beta1.Ingress, namespace string, k8sClient kubernetes.Interface) (*v1beta1.Ingress, error) {
	return k8sClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingressSpec)
}

func createOrUpdateDeploymentResource(deploymentSpec *v1beta1.Deployment, namespace string, k8sClient kubernetes.Interface) (*v1beta1.Deployment, error) {
	if deploymentSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.ExtensionsV1beta1().Deployments(namespace).Update(deploymentSpec)
	} else {
		return k8sClient.ExtensionsV1beta1().Deployments(namespace).Create(deploymentSpec)
	}
}

func createServiceResource(serviceSpec *v1.Service, namespace string, k8sClient kubernetes.Interface) (*v1.Service, error) {
	return k8sClient.CoreV1().Services(namespace).Create(serviceSpec)
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