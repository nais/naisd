package api

import (
	"fmt"
	"github.com/nais/naisd/naisresource"
	redisapi "github.com/spotahome/redis-operator/api/redisfailover/v1alpha2"
	k8sapps "k8s.io/api/apps/v1"
	k8sautoscaling "k8s.io/api/autoscaling/v1"
	k8score "k8s.io/api/core/v1"
	k8sextensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"strings"
)

const RootMountPoint = "/var/run/secrets/naisd.io/"

type DeploymentResult struct {
	Autoscaler      *k8sautoscaling.HorizontalPodAutoscaler
	Ingress         *k8sextensions.Ingress
	Deployment      *k8sapps.Deployment
	Secret          *k8score.Secret
	Service         *k8score.Service
	Redis           *redisapi.RedisFailover
	AlertsConfigMap *k8score.ConfigMap
}

func validLabelName(str string) string {
	tmpStr := strings.Replace(str, "_", "-", -1)
	return strings.ToLower(tmpStr)
}

// Creates a Kubernetes Deployment object
// If existingDeployment is provided, this is updated with modifiable fields

func hasCertificate(naisResources []NaisResource) bool {
	for _, resource := range naisResources {
		if len(resource.certificates) > 0 {
			return true
		}
	}
	return false
}

func createCertificateVolume(deploymentRequest NaisDeploymentRequest, resources []NaisResource) k8score.Volume {
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
			Name: validLabelName(deploymentRequest.Application),
			VolumeSource: k8score.VolumeSource{
				Secret: &k8score.SecretVolumeSource{
					SecretName: deploymentRequest.Application,
					Items:      items,
				},
			},
		}
	}

	return k8score.Volume{}
}

func createCertificateVolumeMount(deploymentRequest NaisDeploymentRequest, resources []NaisResource) k8score.VolumeMount {
	for _, res := range resources {
		if res.certificates != nil {
			return k8score.VolumeMount{
				Name:      validLabelName(deploymentRequest.Application),
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

func createEnvironmentVariables(deploymentRequest NaisDeploymentRequest, naisResources []NaisResource) ([]k8score.EnvVar, error) {
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
								Name: deploymentRequest.Application,
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
	return envVars, nil
}

func createDefaultEnvironmentVariables(request *NaisDeploymentRequest) []k8score.EnvVar {
	return []k8score.EnvVar{{
		Name:  "APP_NAME",
		Value: request.Application,
	},
		{
			Name:  "APP_VERSION",
			Value: request.Version,
		},
		{
			Name:  "FASIT_ENVIRONMENT_NAME",
			Value: request.FasitEnvironment,
		}}
}

// Creates a Kubernetes Secret object
// If existingSecretId is provided, this is included in object so it can be used to update object
func createSecretDef(naisResources []NaisResource, existingSecret *k8score.Secret, application, namespace, teamName string) *k8score.Secret {
	if existingSecret != nil {
		existingSecret.Data = createSecretData(naisResources)
		return existingSecret
	} else {
		secret := &k8score.Secret{
			TypeMeta: k8smeta.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: createObjectMeta(application, namespace, teamName),
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
func createIngressDef(application, namespace, teamName string) *k8sextensions.Ingress {
	return &k8sextensions.Ingress{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: createObjectMeta(application, namespace, teamName),
		Spec:       k8sextensions.IngressSpec{},
	}
}

func createIngressHostname(application, namespace, subdomain string) string {
	if namespace == "default" {
		return fmt.Sprintf("%s.%s", application, subdomain)
	} else {
		return fmt.Sprintf("%s-%s.%s", application, namespace, subdomain)
	}
}

func createSBSPublicHostname(request NaisDeploymentRequest) string {
	environment := request.FasitEnvironment
	if environment != ENVIRONMENT_P {
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

func createOrUpdateK8sResources(deploymentRequest NaisDeploymentRequest, manifest NaisManifest, resources []NaisResource, clusterSubdomain string, istioEnabledGlobally bool, k8sClient kubernetes.Interface) (DeploymentResult, error) {
	var deploymentResult DeploymentResult

	objectMeta := createObjectMeta(deploymentRequest.Application, deploymentRequest.Namespace, manifest.Team)

	service, err := naisresource.CreateService(objectMeta, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating service: %s", err)
	}
	deploymentResult.Service = service

	if manifest.Redis {
		redis, err := createRedisSentinelCluster(deploymentRequest, manifest.Team)
		if err != nil {
			return deploymentResult, fmt.Errorf("failed while creating Redis sentinel cluster: %s", err)
		}
		deploymentResult.Redis = redis
	}

	deployment, err := createOrUpdateDeployment(objectMeta, deploymentRequest, manifest, resources, istioEnabledGlobally, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating deployment: %s", err)
	}
	deploymentResult.Deployment = deployment

	secret, err := createOrUpdateSecret(deploymentRequest, resources, k8sClient, manifest.Team)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating secret: %s", err)
	}
	deploymentResult.Secret = secret

	if !manifest.Ingress.Disabled {
		ingress, err := createOrUpdateIngress(deploymentRequest, manifest.Team, clusterSubdomain, resources, k8sClient)
		if err != nil {
			return deploymentResult, fmt.Errorf("failed while creating ingress: %s", err)
		}
		deploymentResult.Ingress = ingress
	}

	autoscaler, err := naisresource.CreateOrUpdateAutoscaler(objectMeta, manifest.Replicas.Min, manifest.Replicas.Max, manifest.Replicas.CpuThresholdPercentage, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating autoscaler: %s", err)
	}

	deploymentResult.Autoscaler = autoscaler

	alertsConfigMap, err := createOrUpdateAlertRules(deploymentRequest, manifest, k8sClient)
	if err != nil {
		return deploymentResult, fmt.Errorf("failed while creating or updating app-alerts configmap %s", err)
	}
	deploymentResult.AlertsConfigMap = alertsConfigMap

	return deploymentResult, err
}

func createOrUpdateAlertRules(deploymentRequest NaisDeploymentRequest, manifest NaisManifest, k8sClient kubernetes.Interface) (*k8score.ConfigMap, error) {
	if len(manifest.Alerts) == 0 {
		return nil, nil
	}

	alertsConfigMapNamespace := "nais"
	alertsConfigMapName := "app-alerts"
	configMap, err := getExistingConfigMap(alertsConfigMapName, alertsConfigMapNamespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing configmap: %s", err)
	}

	if configMap == nil {
		configMap = createConfigMapDef(alertsConfigMapName, alertsConfigMapNamespace, manifest.Team)
	}

	configMapWithUpdatedAlertRules, err := addRulesToConfigMap(configMap, deploymentRequest, manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to add alert rules to configmap: %s", err)
	}

	return createOrUpdateConfigMapResource(configMapWithUpdatedAlertRules, alertsConfigMapNamespace, k8sClient)
}

// Returns nil,nil if ingress already exists. No reason to do update, as nothing can change
func createOrUpdateIngress(deploymentRequest NaisDeploymentRequest, teamName, clusterSubdomain string, naisResources []NaisResource, k8sClient kubernetes.Interface) (*k8sextensions.Ingress, error) {
	ingress, err := getExistingIngress(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing ingress id: %s", err)
	}

	if ingress == nil {
		ingress = createIngressDef(deploymentRequest.Application, deploymentRequest.Namespace, teamName)
	}

	ingress.Spec.TLS = []k8sextensions.IngressTLS{{SecretName: "istio-ingress-certs"}}
	ingress.Spec.Rules = createIngressRules(deploymentRequest, clusterSubdomain, naisResources)
	return createOrUpdateIngressResource(ingress, deploymentRequest.Namespace, k8sClient)
}

func createIngressRules(deploymentRequest NaisDeploymentRequest, clusterSubdomain string, naisResources []NaisResource) []k8sextensions.IngressRule {
	var ingressRules []k8sextensions.IngressRule

	defaultIngressRule := createIngressRule(deploymentRequest.Application, createIngressHostname(deploymentRequest.Application, deploymentRequest.Namespace, clusterSubdomain), "")
	ingressRules = append(ingressRules, defaultIngressRule)

	if deploymentRequest.Zone == ZONE_SBS {
		ingressRules = append(ingressRules, createIngressRule(deploymentRequest.Application, createSBSPublicHostname(deploymentRequest), deploymentRequest.Application))
	}

	for _, naisResource := range naisResources {
		if naisResource.resourceType == "LoadBalancerConfig" && len(naisResource.ingresses) > 0 {
			for host, path := range naisResource.ingresses {
				ingressRules = append(ingressRules, createIngressRule(deploymentRequest.Application, host, path))
			}
		}
	}

	return ingressRules
}

func createConfigMapDef(name, namespace, teamName string) *k8score.ConfigMap {
	meta := createObjectMeta(name, namespace, teamName)
	return &k8score.ConfigMap{ObjectMeta: meta}
}

func createOrUpdateSecret(deploymentRequest NaisDeploymentRequest, naisResources []NaisResource, k8sClient kubernetes.Interface, teamName string) (*k8score.Secret, error) {
	existingSecret, err := getExistingSecret(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing secret: %s", err)
	}

	if secretDef := createSecretDef(naisResources, existingSecret, deploymentRequest.Application, deploymentRequest.Namespace, teamName); secretDef != nil {
		return createOrUpdateSecretResource(secretDef, deploymentRequest.Namespace, k8sClient)
	} else {
		return nil, nil
	}
}

func getExistingSecret(application string, namespace string, k8sClient kubernetes.Interface) (*k8score.Secret, error) {
	secretClient := k8sClient.CoreV1().Secrets(namespace)
	secret, err := secretClient.Get(application, k8smeta.GetOptions{})
	switch {
	case err == nil:
		return secret, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func getExistingIngress(application string, namespace string, k8sClient kubernetes.Interface) (*k8sextensions.Ingress, error) {
	ingressClient := k8sClient.ExtensionsV1beta1().Ingresses(namespace)
	ingress, err := ingressClient.Get(application, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return ingress, err
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

func createOrUpdateIngressResource(ingressSpec *k8sextensions.Ingress, namespace string, k8sClient kubernetes.Interface) (*k8sextensions.Ingress, error) {
	if ingressSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.ExtensionsV1beta1().Ingresses(namespace).Update(ingressSpec)
	} else {
		return k8sClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingressSpec)
	}
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

func createObjectMeta(applicationName, namespace, teamName string) k8smeta.ObjectMeta {
	labels := map[string]string{"app": applicationName}

	if teamName != "" {
		labels["team"] = teamName
	}

	return k8smeta.ObjectMeta{
		Name:      applicationName,
		Namespace: namespace,
		Labels:    labels,
	}
}

func createPodAnnotationsMap(prometheusEnabled bool, prometheusPath string, istioEnabledForDeployment, istioEnabledGlobally bool) map[string]string {
	annotations := map[string]string{
		"prometheus.io/scrape": strconv.FormatBool(prometheusEnabled),
		"prometheus.io/port":   DefaultPortName,
		"prometheus.io/path":   prometheusPath,
	}

	if istioEnabledForDeployment && istioEnabledGlobally {
		annotations["sidecar.istio.io/inject"] = "true"
	}

	return annotations
}

func naisProbeToK8sProbe(probe Probe) k8score.Probe {
	return naisresource.CreateProbe(probe.Path, int32(probe.InitialDelay), int32(probe.Timeout), int32(probe.PeriodSeconds), int32(probe.FailureThreshold))
}

func assembleDeploymentSpec(meta k8smeta.ObjectMeta, deploymentRequest NaisDeploymentRequest, manifest NaisManifest, naisResources []NaisResource, istioEnabledGlobally bool) (k8sapps.DeploymentSpec, error) {
	environmentVariables, err := createEnvironmentVariables(deploymentRequest, naisResources)

	if err != nil {
		return k8sapps.DeploymentSpec{}, fmt.Errorf("unable to create environment: %s", err)
	}

	volumes := make([]k8score.Volume, 0)
	volumeMounts := make([]k8score.VolumeMount, 0)
	if hasCertificate(naisResources) {
		volumes = append(volumes, createCertificateVolume(deploymentRequest, naisResources))
		volumeMounts = append(volumeMounts, createCertificateVolumeMount(deploymentRequest, naisResources))
	}

	sidecars := make([]k8score.Container, 0)
	if manifest.LeaderElection {
		sidecars = append(sidecars, naisresource.CreateLeaderElectionContainer(meta.Name))
		electorPathEnv := naisresource.CreateEnvVar("ELECTOR_PATH", "localhost:4040")
		environmentVariables = append(environmentVariables, electorPathEnv)
	}

	if manifest.Redis {
		sidecars = append(sidecars, createRedisExporterContainer(deploymentRequest.Application))
	}

	containerLifecycle := naisresource.CreateLifeCycle(manifest.PreStopHookPath)
	resourceLimits := resourcesToResourceLimits(manifest.Resources)
	livenessProbe := naisProbeToK8sProbe(manifest.Healthcheck.Liveness)
	readinessProbe := naisProbeToK8sProbe(manifest.Healthcheck.Readiness)
	taggedImage := fmt.Sprintf("%s:%s", manifest.Image, deploymentRequest.Version)
	containerSpec := naisresource.CreateContainerSpec(meta, taggedImage, manifest.Port, livenessProbe, readinessProbe, containerLifecycle, resourceLimits, environmentVariables, volumeMounts)

	podSpec := naisresource.CreatePodSpec(containerSpec, sidecars, volumes)
	podAnnotations := createPodAnnotationsMap(manifest.Prometheus.Enabled, manifest.Prometheus.Path, manifest.Istio.Enabled, istioEnabledGlobally)

	return naisresource.CreateDeploymentSpec(meta, podAnnotations, podSpec), nil
}

func resourcesToResourceLimits(resources ResourceRequirements) k8score.ResourceRequirements {
	return naisresource.CreateResourceLimits(resources.Requests.Cpu, resources.Requests.Memory, resources.Limits.Cpu, resources.Limits.Memory)
}

func createOrUpdateDeployment(meta k8smeta.ObjectMeta, deploymentRequest NaisDeploymentRequest, manifest NaisManifest, naisResources []NaisResource, istioEnabledGlobally bool, k8sClient kubernetes.Interface) (*k8sapps.Deployment, error) {
	deployment, err := naisresource.GetExistingDeployment(meta.Name, meta.Namespace, k8sClient)
	if err != nil {
		return nil, fmt.Errorf("unable to get existing deployment: %s", err)
	}

	deploymentSpec, err := assembleDeploymentSpec(meta, deploymentRequest, manifest, naisResources, istioEnabledGlobally)
	if err != nil {
		return nil, fmt.Errorf("unable to assemble deployment spec: %s", err)
	}

	if deployment == nil {
		deployment = naisresource.CreateDeploymentDef(meta)
		deployment.Spec = deploymentSpec
		return k8sClient.AppsV1().Deployments(meta.Namespace).Create(deployment)
	} else {
		deployment.Spec = deploymentSpec
		return k8sClient.AppsV1().Deployments(meta.Namespace).Update(deployment)
	}
}
