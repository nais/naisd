package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	"os"
	"strconv"
	"strings"

	"github.com/nais/naisd/api/constant"
	"github.com/nais/naisd/api/naisrequest"
	redisapi "github.com/spotahome/redis-operator/api/redisfailover/v1alpha2"
	k8sautoscaling "k8s.io/api/autoscaling/v1"
	k8score "k8s.io/api/core/v1"
	k8sextensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	RootMountPoint           = "/var/run/secrets/naisd.io/"
	AlertsConfigMapNamespace = "nais"
	AlertsConfigMapName      = "app-rules"
)

type DeploymentResult struct {
	Autoscaler      *k8sautoscaling.HorizontalPodAutoscaler
	Ingress         *k8sextensions.Ingress
	Deployment      *k8sextensions.Deployment
	Secret          *k8score.Secret
	Service         *k8score.Service
	Redis           *redisapi.RedisFailover
	AlertsConfigMap *k8score.ConfigMap
	ServiceAccount  *k8score.ServiceAccount
	Namespace       *k8score.Namespace
}

// Creates a Kubernetes Service object
func createServiceDef(spec app.Spec) *k8score.Service {
	return &k8score.Service{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: generateObjectMeta(spec),
		Spec: k8score.ServiceSpec{
			Type: k8score.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app":         spec.Application,
				"environment": spec.Environment,
			},
			Ports: []k8score.ServicePort{
				{
					Name:     "http",
					Protocol: k8score.ProtocolTCP,
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

func validLabelName(str string) string {
	tmpStr := strings.Replace(str, "_", "-", -1)
	return strings.ToLower(tmpStr)
}

// Creates a Kubernetes Deployment object
// If existingDeployment is provided, this is updated with modifiable fields
func createDeploymentDef(spec app.Spec, naisResources []NaisResource, manifest NaisManifest, deploymentRequest naisrequest.Deploy, existingDeployment *k8sextensions.Deployment, istioEnabled bool) (*k8sextensions.Deployment, error) {
	deploymentSpec, err := createDeploymentSpec(spec, deploymentRequest, manifest, naisResources, istioEnabled)

	if err != nil {
		return nil, err
	}

	if existingDeployment != nil {
		existingDeployment.Spec = deploymentSpec
		return existingDeployment, nil
	} else {
		deployment := &k8sextensions.Deployment{
			TypeMeta: k8smeta.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1beta1",
			},
			ObjectMeta: generateObjectMeta(spec),
			Spec:       deploymentSpec,
		}
		return deployment, nil
	}
}

func createDeploymentSpec(spec app.Spec, deploymentRequest naisrequest.Deploy, manifest NaisManifest, naisResources []NaisResource, istioEnabled bool) (k8sextensions.DeploymentSpec, error) {
	podSpec, err := createPodSpec(spec, deploymentRequest, manifest, naisResources)

	if err != nil {
		return k8sextensions.DeploymentSpec{}, err
	}

	return k8sextensions.DeploymentSpec{
		Replicas: int32p(1),
		Strategy: k8sextensions.DeploymentStrategy{
			Type: k8sextensions.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &k8sextensions.RollingUpdateDeployment{
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
		Template: k8score.PodTemplateSpec{
			ObjectMeta: createPodObjectMetaWithAnnotations(spec, manifest, istioEnabled),
			Spec:       podSpec,
		},
	}, nil
}

func createPodObjectMetaWithAnnotations(spec app.Spec, manifest NaisManifest, istioEnabled bool) k8smeta.ObjectMeta {
	objectMeta := generateObjectMeta(spec)
	objectMeta.Annotations = map[string]string{
		"prometheus.io/scrape": strconv.FormatBool(manifest.Prometheus.Enabled),
		"prometheus.io/port":   DefaultPortName,
		"prometheus.io/path":   manifest.Prometheus.Path,
	}

	if istioEnabled && manifest.Istio.Enabled {
		objectMeta.Annotations["sidecar.istio.io/inject"] = "true"
	}

	if len(manifest.Logformat) > 0 {
		objectMeta.Annotations["nais.io/logformat"] = manifest.Logformat
	}

	if len(manifest.Logtransform) > 0 {
		objectMeta.Annotations["nais.io/logtransform"] = manifest.Logtransform
	}

	return objectMeta
}

func createPodSpec(spec app.Spec, deploymentRequest naisrequest.Deploy, manifest NaisManifest, naisResources []NaisResource) (k8score.PodSpec, error) {
	envVars, err := createEnvironmentVariables(spec, deploymentRequest, manifest, naisResources)

	if err != nil {
		return k8score.PodSpec{}, err
	}

	podSpec := k8score.PodSpec{
		Containers: []k8score.Container{
			{
				Name:  spec.Application,
				Image: fmt.Sprintf("%s:%s", manifest.Image, deploymentRequest.Version),
				Ports: []k8score.ContainerPort{
					{ContainerPort: int32(manifest.Port), Protocol: k8score.ProtocolTCP, Name: DefaultPortName},
				},
				Resources: createResourceLimits(manifest.Resources.Requests.Cpu, manifest.Resources.Requests.Memory, manifest.Resources.Limits.Cpu, manifest.Resources.Limits.Memory),
				LivenessProbe: &k8score.Probe{
					Handler: k8score.Handler{
						HTTPGet: &k8score.HTTPGetAction{
							Path: manifest.Healthcheck.Liveness.Path,
							Port: intstr.FromString(DefaultPortName),
						},
					},
					InitialDelaySeconds: int32(manifest.Healthcheck.Liveness.InitialDelay),
					PeriodSeconds:       int32(manifest.Healthcheck.Liveness.PeriodSeconds),
					FailureThreshold:    int32(manifest.Healthcheck.Liveness.FailureThreshold),
					TimeoutSeconds:      int32(manifest.Healthcheck.Liveness.Timeout),
				},
				ReadinessProbe: &k8score.Probe{
					Handler: k8score.Handler{
						HTTPGet: &k8score.HTTPGetAction{
							Path: manifest.Healthcheck.Readiness.Path,
							Port: intstr.FromString(DefaultPortName),
						},
					},
					InitialDelaySeconds: int32(manifest.Healthcheck.Readiness.InitialDelay),
					PeriodSeconds:       int32(manifest.Healthcheck.Readiness.PeriodSeconds),
					FailureThreshold:    int32(manifest.Healthcheck.Readiness.FailureThreshold),
					TimeoutSeconds:      int32(manifest.Healthcheck.Readiness.Timeout),
				},
				Env:             envVars,
				ImagePullPolicy: k8score.PullIfNotPresent,
				Lifecycle:       createLifeCycle(manifest.PreStopHookPath),
			},
		},
		ServiceAccountName: spec.Application,
		RestartPolicy:      k8score.RestartPolicyAlways,
		DNSPolicy:          k8score.DNSClusterFirst,
	}

	if manifest.LeaderElection {
		podSpec.Containers = append(podSpec.Containers, createLeaderElectionContainer(spec))

		mainContainer := &podSpec.Containers[0]
		electorPathEnv := k8score.EnvVar{
			Name:  "ELECTOR_PATH",
			Value: "localhost:4040",
		}
		mainContainer.Env = append(mainContainer.Env, electorPathEnv)
	}

	if hasCertificate(naisResources) {
		podSpec.Volumes = append(podSpec.Volumes, createCertificateVolume(spec, naisResources))
		container := &podSpec.Containers[0]
		container.VolumeMounts = append(container.VolumeMounts, createCertificateVolumeMount(spec, naisResources))
	}

	return podSpec, nil
}

func createLeaderElectionContainer(spec app.Spec) k8score.Container {
	return k8score.Container{
		Name:            "elector",
		Image:           "gcr.io/google_containers/leader-elector:0.5",
		ImagePullPolicy: k8score.PullIfNotPresent,
		Resources: k8score.ResourceRequirements{
			Requests: k8score.ResourceList{
				k8score.ResourceCPU: k8sresource.MustParse("100m"),
			},
		},
		Ports: []k8score.ContainerPort{
			{ContainerPort: 4040, Protocol: k8score.ProtocolTCP},
		},
		Args: []string{"--election=" + spec.ResourceName(), "--http=localhost:4040", fmt.Sprintf("--election-namespace=%s", spec.Namespace())},
	}
}

func createLifeCycle(path string) *k8score.Lifecycle {
	if len(path) > 0 {
		return &k8score.Lifecycle{
			PreStop: &k8score.Handler{
				HTTPGet: &k8score.HTTPGetAction{
					Path: path,
					Port: intstr.FromString(DefaultPortName),
				},
			},
		}
	}

	return &k8score.Lifecycle{}
}

func hasCertificate(naisResources []NaisResource) bool {
	for _, resource := range naisResources {
		if len(resource.certificates) > 0 {
			return true
		}
	}
	return false
}

func createCertificateVolume(spec app.Spec, resources []NaisResource) k8score.Volume {
	var items []k8score.KeyToPath
	for _, res := range resources {
		if res.certificates != nil {
			for k := range res.certificates {
				item := k8score.KeyToPath{
					Key:  res.ToResourceVariable(k),
					Path: res.ToResourceVariable(k),
				}
				items = append(items, item)
			}
		}
	}

	if len(items) > 0 {
		return k8score.Volume{
			Name: validLabelName(spec.Application),
			VolumeSource: k8score.VolumeSource{
				Secret: &k8score.SecretVolumeSource{
					SecretName: spec.Application,
					Items:      items,
				},
			},
		}
	}

	return k8score.Volume{}
}

func createCertificateVolumeMount(spec app.Spec, resources []NaisResource) k8score.VolumeMount {
	for _, res := range resources {
		if res.certificates != nil {
			return k8score.VolumeMount{
				Name:      validLabelName(spec.Application),
				MountPath: RootMountPoint,
			}
		}
	}
	return k8score.VolumeMount{}
}

func checkForDuplicates(envVars []k8score.EnvVar, envVar k8score.EnvVar, property string, resource NaisResource) error {
	for _, existingEnvVar := range envVars {
		if envVar.Name == existingEnvVar.Name {
			return fmt.Errorf("found duplicate environment variable %s when adding %s for %s (%s)"+
				" Change the Fasit alias or use propertyMap to create unique variable names",
				existingEnvVar.Name, property, resource.name, resource.resourceType)
		}

		if envVar.ValueFrom == nil || envVar.ValueFrom.SecretKeyRef == nil ||
			existingEnvVar.ValueFrom == nil || existingEnvVar.ValueFrom.SecretKeyRef == nil {
			continue
		}

		if envVar.ValueFrom.SecretKeyRef.Key == existingEnvVar.ValueFrom.SecretKeyRef.Key {
			return fmt.Errorf("found duplicate secret key ref %s between %s and %s when adding %s for %s (%s)"+
				" Change the Fasit alias or use propertyMap to create unique variable names",
				existingEnvVar.ValueFrom.SecretKeyRef.Key, existingEnvVar.Name, envVar.Name,
				property, resource.name, resource.resourceType)
		}
	}

	return nil
}

func createEnvironmentVariables(spec app.Spec, deploymentRequest naisrequest.Deploy, manifest NaisManifest, naisResources []NaisResource) ([]k8score.EnvVar, error) {
	envVars := createDefaultEnvironmentVariables(&deploymentRequest)

	for _, res := range naisResources {
		for variableName, v := range res.properties {
			envVar := k8score.EnvVar{Name: res.ToEnvironmentVariable(variableName), Value: v}

			if err := checkForDuplicates(envVars, envVar, variableName, res); err != nil {
				return nil, err
			}

			envVars = append(envVars, envVar)
		}
		if res.secret != nil {
			for k := range res.secret {
				envVar := k8score.EnvVar{
					Name: res.ToEnvironmentVariable(k),
					ValueFrom: &k8score.EnvVarSource{
						SecretKeyRef: &k8score.SecretKeySelector{
							LocalObjectReference: k8score.LocalObjectReference{
								Name: spec.ResourceName(),
							},
							Key: res.ToResourceVariable(k),
						},
					},
				}

				if err := checkForDuplicates(envVars, envVar, k, res); err != nil {
					return nil, err
				}

				envVars = append(envVars, envVar)
			}
		}

		if res.certificates != nil {
			for k := range res.certificates {
				envVar := k8score.EnvVar{
					Name:  res.ToEnvironmentVariable(k),
					Value: res.MountPoint(k),
				}

				if err := checkForDuplicates(envVars, envVar, k, res); err != nil {
					return nil, err
				}

				envVars = append(envVars, envVar)

			}
		}
	}

	// If the deployment specifies webproxy=true in the nais manifest, the pods
	// will inherit naisd's proxy settings.  This is useful for automatic proxy
	// configuration so that apps don't need to be aware of infrastructure quirks.
	//
	// Additionally, proxy settings on Linux is in a messy state. Some
	// applications and libraries read the upper-case variables, while some read
	// the lower-case versions. We handle this by trying to read both versions
	// from the naisd environment, and propagating both versions to the pod.
	if manifest.WebProxy {
		for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
			value := getEnvDualCase(key)
			for _, mkey := range []string{strings.ToUpper(key), strings.ToLower(key)} {
				envVar := k8score.EnvVar{
					Name:  mkey,
					Value: value,
				}
				envVars = append(envVars, envVar)
			}
		}
	}

	return envVars, nil
}

func getEnvDualCase(name string) string {
	value, found := os.LookupEnv(strings.ToUpper(name))
	if found {
		return value
	}
	return os.Getenv(strings.ToLower(name))
}

func createDefaultEnvironmentVariables(request *naisrequest.Deploy) []k8score.EnvVar {
	envVars := []k8score.EnvVar{
		{
			Name:  "APP_NAME",
			Value: request.Application,
		},
		{
			Name:  "APP_VERSION",
			Value: request.Version,
		},
		{
			Name:  "APP_ENVIRONMENT",
			Value: request.Environment,
		},
	}

	if !request.SkipFasit {
		envVars = append(envVars, k8score.EnvVar{
			Name:  "FASIT_ENVIRONMENT_NAME",
			Value: request.FasitEnvironment,
		})
	}

	return envVars
}

func createResourceLimits(requestsCpu string, requestsMemory string, limitsCpu string, limitsMemory string) k8score.ResourceRequirements {
	return k8score.ResourceRequirements{
		Requests: k8score.ResourceList{
			k8score.ResourceCPU:    k8sresource.MustParse(requestsCpu),
			k8score.ResourceMemory: k8sresource.MustParse(requestsMemory),
		},
		Limits: k8score.ResourceList{
			k8score.ResourceCPU:    k8sresource.MustParse(limitsCpu),
			k8score.ResourceMemory: k8sresource.MustParse(limitsMemory),
		},
	}
}

// Creates a Kubernetes Secret object
// If existingSecretId is provided, this is included in object so it can be used to update object
func createSecretDef(spec app.Spec, naisResources []NaisResource, existingSecret *k8score.Secret) *k8score.Secret {
	if existingSecret != nil {
		existingSecret.Data = createSecretData(naisResources)
		return existingSecret
	} else {
		secret := &k8score.Secret{
			TypeMeta: k8smeta.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: generateObjectMeta(spec),
			Data:       createSecretData(naisResources),
			Type:       "Opaque",
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
				data[res.ToResourceVariable(k)] = []byte(v)
			}
		}
		if res.certificates != nil {
			for k, v := range res.certificates {
				data[res.ToResourceVariable(k)] = v
			}
		}
	}
	return data
}

// Creates a Kubernetes Ingress object
func createIngressDef(spec app.Spec) *k8sextensions.Ingress {
	return &k8sextensions.Ingress{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: generateObjectMeta(spec),
		Spec:       k8sextensions.IngressSpec{},
	}
}

func createIngressHostname(application, environment, subdomain string) string {
	if environment == "default" {
		return fmt.Sprintf("%s.%s", application, subdomain)
	} else {
		return fmt.Sprintf("%s-%s.%s", application, environment, subdomain)
	}
}

func createSBSPublicHostname(request naisrequest.Deploy) string {
	environment := request.FasitEnvironment
	if environment != constant.ENVIRONMENT_P {
		return fmt.Sprintf("tjenester-%s.nav.no", environment)
	} else {
		return "tjenester.nav.no"
	}
}

func createIngressRule(serviceName, host, path string) k8sextensions.IngressRule {
	return k8sextensions.IngressRule{
		Host: host,
		IngressRuleValue: k8sextensions.IngressRuleValue{
			HTTP: &k8sextensions.HTTPIngressRuleValue{
				Paths: []k8sextensions.HTTPIngressPath{
					{
						Backend: k8sextensions.IngressBackend{
							ServiceName: serviceName,
							ServicePort: intstr.IntOrString{IntVal: 80},
						},
						Path: strings.Replace("/"+path, "//", "/", 1), // make sure we always begin with exactly one slash
					},
				},
			},
		},
	}
}

// Creates a Kubernetes HorizontalPodAutoscaler object
// If existingAutoscaler is provided, this is updated with provided parameters
func createOrUpdateAutoscalerDef(spec app.Spec, min, max, cpuTargetPercentage int, existingAutoscaler *k8sautoscaling.HorizontalPodAutoscaler) *k8sautoscaling.HorizontalPodAutoscaler {
	if existingAutoscaler != nil {
		existingAutoscaler.Spec = createAutoscalerSpec(min, max, cpuTargetPercentage, spec.ResourceName())

		return existingAutoscaler
	} else {

		return &k8sautoscaling.HorizontalPodAutoscaler{
			TypeMeta: k8smeta.TypeMeta{
				Kind:       "HorizontalPodAutoscaler",
				APIVersion: "autoscaling/v1",
			},
			ObjectMeta: generateObjectMeta(spec),
			Spec:       createAutoscalerSpec(min, max, cpuTargetPercentage, spec.ResourceName()),
		}
	}
}

func createAutoscalerSpec(min, max, cpuTargetPercentage int, objectName string) k8sautoscaling.HorizontalPodAutoscalerSpec {
	return k8sautoscaling.HorizontalPodAutoscalerSpec{
		MinReplicas:                    int32p(int32(min)),
		MaxReplicas:                    int32(max),
		TargetCPUUtilizationPercentage: int32p(int32(cpuTargetPercentage)),
		ScaleTargetRef: k8sautoscaling.CrossVersionObjectReference{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
			Name:       objectName,
		},
	}
}

func createOrUpdateK8sResources(deploymentRequest naisrequest.Deploy, manifest NaisManifest, resources []NaisResource, clusterSubdomain string, istioEnabled bool, k8sClient kubernetes.Interface) (DeploymentResult, error) {
	var deploymentResult DeploymentResult
	client := clientHolder{k8sClient}

	spec := app.Spec{
		Application: deploymentRequest.Application,
		Environment: deploymentRequest.Environment,
		Team:        manifest.Team,
	}

	namespace, err := client.createNamespace(spec.Namespace())
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating namespace: %s", err)
	}
	err = client.waitForNamespaceReady(namespace)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while waiting for namespace to become ready: %s", err)
	}
	deploymentResult.Namespace = namespace

	serviceAccount, err := NewServiceAccountInterface(k8sClient).CreateIfNotExist(spec)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating service account: %s", err)
	}
	deploymentResult.ServiceAccount = serviceAccount

	service, err := createService(spec, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating service: %s", err)
	}
	deploymentResult.Service = service

	if manifest.Redis {
		redis, err := updateOrCreateRedisSentinelCluster(spec)
		if err != nil {
			return deploymentResult, fmt.Errorf("failed while creating Redis sentinel cluster: %s", err)
		}
		deploymentResult.Redis = redis
	}

	deployment, err := createOrUpdateDeployment(spec, deploymentRequest, manifest, resources, istioEnabled, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating deployment: %s", err)
	}
	deploymentResult.Deployment = deployment

	secret, err := createOrUpdateSecret(spec, resources, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating secret: %s", err)
	}
	deploymentResult.Secret = secret

	if !manifest.Ingress.Disabled {
		ingress, err := createOrUpdateIngress(spec, deploymentRequest, clusterSubdomain, resources, k8sClient)
		if err != nil {
			return deploymentResult, fmt.Errorf("failed while creating ingress: %s", err)
		}
		deploymentResult.Ingress = ingress
	}

	autoscaler, err := createOrUpdateAutoscaler(spec, manifest, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating autoscaler: %s", err)
	}

	deploymentResult.Autoscaler = autoscaler

	alertsConfigMap, err := createOrUpdateAlertRules(spec, manifest, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating alerts configmap (app-rules) %s", err)
	}
	deploymentResult.AlertsConfigMap = alertsConfigMap

	return deploymentResult, err
}

func createOrUpdateAlertRules(spec app.Spec, manifest NaisManifest, k8sClient kubernetes.Interface) (*k8score.ConfigMap, error) {
	if len(manifest.Alerts) == 0 {
		return nil, nil
	}

	configMap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing configmap: %s", err)
	}

	if configMap == nil {
		configMap = &k8score.ConfigMap{ObjectMeta: createObjectMeta(AlertsConfigMapName, AlertsConfigMapNamespace)}
	}

	configMapWithUpdatedAlertRules, err := addRulesToConfigMap(spec, configMap, manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to add alert rules to configmap: %s", err)
	}

	return createOrUpdateConfigMapResource(configMapWithUpdatedAlertRules, AlertsConfigMapNamespace, k8sClient)
}

func createOrUpdateAutoscaler(spec app.Spec, manifest NaisManifest, k8sClient kubernetes.Interface) (*k8sautoscaling.HorizontalPodAutoscaler, error) {
	autoscaler, err := getExistingAutoscaler(spec, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing autoscaler: %s", err)
	}

	autoscalerDef := createOrUpdateAutoscalerDef(spec, manifest.Replicas.Min, manifest.Replicas.Max, manifest.Replicas.CpuThresholdPercentage, autoscaler)
	return createOrUpdateAutoscalerResource(autoscalerDef, spec.Namespace(), k8sClient)
}

// Returns nil,nil if ingress already exists. No reason to do update, as nothing can change
func createOrUpdateIngress(spec app.Spec, deploymentRequest naisrequest.Deploy, clusterSubdomain string, naisResources []NaisResource, k8sClient kubernetes.Interface) (*k8sextensions.Ingress, error) {
	ingress, err := getExistingIngress(spec, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing ingress id: %s", err)
	}

	if ingress == nil {
		ingress = createIngressDef(spec)
	}

	ingress.Spec.Rules = createIngressRules(spec, deploymentRequest, clusterSubdomain, naisResources)
	return createOrUpdateIngressResource(ingress, spec.Namespace(), k8sClient)
}

func createIngressRules(spec app.Spec, deploymentRequest naisrequest.Deploy, clusterSubdomain string, naisResources []NaisResource) []k8sextensions.IngressRule {
	var ingressRules []k8sextensions.IngressRule

	defaultIngressRule := createIngressRule(spec.Application, createIngressHostname(spec.Application, deploymentRequest.Environment, clusterSubdomain), "")
	ingressRules = append(ingressRules, defaultIngressRule)

	if deploymentRequest.Zone == constant.ZONE_SBS {
		ingressRules = append(ingressRules, createIngressRule(spec.Application, createSBSPublicHostname(deploymentRequest), spec.Application))
	}

	for _, naisResource := range naisResources {
		if naisResource.resourceType == "LoadBalancerConfig" && len(naisResource.ingresses) > 0 {
			for _, ingress := range naisResource.ingresses {
				ingressRules = append(ingressRules, createIngressRule(spec.Application, ingress.Host, ingress.Path))
			}
		}
	}

	return ingressRules
}

func createService(spec app.Spec, k8sClient kubernetes.Interface) (*k8score.Service, error) {
	existingService, err := getExistingService(spec, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing service: %s", err)
	}

	if existingService != nil {
		return nil, nil // we have done nothing
	}

	serviceDef := createServiceDef(spec)
	return createServiceResource(serviceDef, spec.Namespace(), k8sClient)
}

func createOrUpdateDeployment(spec app.Spec, deploymentRequest naisrequest.Deploy, manifest NaisManifest, naisResources []NaisResource, istioEnabled bool, k8sClient kubernetes.Interface) (*k8sextensions.Deployment, error) {
	existingDeployment, err := getExistingDeployment(spec, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing deployment: %s", err)
	}

	deploymentDef, err := createDeploymentDef(spec, naisResources, manifest, deploymentRequest, existingDeployment, istioEnabled)

	if err != nil {
		return nil, fmt.Errorf("unable to create deployment: %s", err)
	}

	return createOrUpdateDeploymentResource(deploymentDef, spec.Namespace(), k8sClient)
}

func createOrUpdateSecret(spec app.Spec, naisResources []NaisResource, k8sClient kubernetes.Interface) (*k8score.Secret, error) {
	existingSecret, err := getExistingSecret(spec, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing secret: %s", err)
	}

	if secretDef := createSecretDef(spec, naisResources, existingSecret); secretDef != nil {
		return createOrUpdateSecretResource(secretDef, spec.Namespace(), k8sClient)
	} else {
		return nil, nil
	}
}

func getExistingService(spec app.Spec, k8sClient kubernetes.Interface) (*k8score.Service, error) {
	serviceClient := k8sClient.CoreV1().Services(spec.Namespace())
	service, err := serviceClient.Get(spec.ResourceName(), k8smeta.GetOptions{})

	switch {
	case err == nil:
		return service, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingSecret(spec app.Spec, k8sClient kubernetes.Interface) (*k8score.Secret, error) {
	secretClient := k8sClient.CoreV1().Secrets(spec.Namespace())
	secret, err := secretClient.Get(spec.ResourceName(), k8smeta.GetOptions{})
	switch {
	case err == nil:
		return secret, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingDeployment(spec app.Spec, k8sClient kubernetes.Interface) (*k8sextensions.Deployment, error) {
	deploymentClient := k8sClient.ExtensionsV1beta1().Deployments(spec.Application)
	deployment, err := deploymentClient.Get(spec.ResourceName(), k8smeta.GetOptions{})

	switch {
	case err == nil:
		return deployment, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingIngress(spec app.Spec, k8sClient kubernetes.Interface) (*k8sextensions.Ingress, error) {
	ingressClient := k8sClient.ExtensionsV1beta1().Ingresses(spec.Namespace())
	ingress, err := ingressClient.Get(spec.ResourceName(), k8smeta.GetOptions{})

	switch {
	case err == nil:
		return ingress, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingAutoscaler(spec app.Spec, k8sClient kubernetes.Interface) (*k8sautoscaling.HorizontalPodAutoscaler, error) {
	autoscalerClient := k8sClient.AutoscalingV1().HorizontalPodAutoscalers(spec.Namespace())
	autoscaler, err := autoscalerClient.Get(spec.ResourceName(), k8smeta.GetOptions{})

	switch {
	case err == nil:
		return autoscaler, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingConfigMap(configMapName string, namespace string, k8sClient kubernetes.Interface) (*k8score.ConfigMap, error) {
	configMapClient := k8sClient.CoreV1().ConfigMaps(namespace)
	configMap, err := configMapClient.Get(configMapName, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return configMap, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func createOrUpdateAutoscalerResource(autoscalerSpec *k8sautoscaling.HorizontalPodAutoscaler, namespace string, k8sClient kubernetes.Interface) (*k8sautoscaling.HorizontalPodAutoscaler, error) {
	if autoscalerSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(autoscalerSpec)
	} else {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(autoscalerSpec)
	}
}

func createOrUpdateIngressResource(ingressSpec *k8sextensions.Ingress, namespace string, k8sClient kubernetes.Interface) (*k8sextensions.Ingress, error) {
	if ingressSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.ExtensionsV1beta1().Ingresses(namespace).Update(ingressSpec)
	} else {
		return k8sClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingressSpec)
	}
}

func createOrUpdateDeploymentResource(deploymentSpec *k8sextensions.Deployment, namespace string, k8sClient kubernetes.Interface) (*k8sextensions.Deployment, error) {
	if deploymentSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.ExtensionsV1beta1().Deployments(namespace).Update(deploymentSpec)
	} else {
		return k8sClient.ExtensionsV1beta1().Deployments(namespace).Create(deploymentSpec)
	}
}

func createServiceResource(serviceSpec *k8score.Service, namespace string, k8sClient kubernetes.Interface) (*k8score.Service, error) {
	return k8sClient.CoreV1().Services(namespace).Create(serviceSpec)
}

func createOrUpdateSecretResource(secretSpec *k8score.Secret, namespace string, k8sClient kubernetes.Interface) (*k8score.Secret, error) {
	if secretSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.CoreV1().Secrets(namespace).Update(secretSpec)
	} else {
		return k8sClient.CoreV1().Secrets(namespace).Create(secretSpec)
	}
}

func createOrUpdateConfigMapResource(configMapSpec *k8score.ConfigMap, namespace string, k8sClient kubernetes.Interface) (*k8score.ConfigMap, error) {
	if configMapSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.CoreV1().ConfigMaps(namespace).Update(configMapSpec)
	} else {
		return k8sClient.CoreV1().ConfigMaps(namespace).Create(configMapSpec)
	}
}

func int32p(i int32) *int32 {
	return &i
}

func generateObjectMeta(spec app.Spec) k8smeta.ObjectMeta {
	objectMeta := createObjectMeta(spec.ResourceName(), spec.Namespace())
	objectMeta.Labels = map[string]string{
		"app":         spec.Application,
		"environment": spec.Environment,
		"team":        spec.Team,
	}

	return objectMeta
}

func createObjectMeta(objectName, namespace string) k8smeta.ObjectMeta {
	return k8smeta.ObjectMeta{Name: objectName, Namespace: namespace}
}

func mergeObjectMeta(exisitingObjectMeta, newObjectMeta k8smeta.ObjectMeta) k8smeta.ObjectMeta {
	exisitingObjectMeta.Name = newObjectMeta.Name
	exisitingObjectMeta.Namespace = newObjectMeta.Namespace
	for k, v := range newObjectMeta.Labels {
		exisitingObjectMeta.Labels[k] = v
	}

	return exisitingObjectMeta
}
