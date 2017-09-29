package api

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
	"strings"
	"testing"
)

const (
	appName         = "appname"
	otherAppName    = "otherappname"
	namespace       = "namespace"
	image           = "docker.hub/app"
	port            = 6900
	resourceVersion = "12369"
	version         = "13"
	livenessPath    = "isAlive"
	readinessPath   = "isReady"
	cpuRequest      = "100m"
	cpuLimit        = "200m"
	memoryRequest   = "200Mi"
	memoryLimit     = "400Mi"
	clusterIP       = "1.2.3.4"
)

func newDefaultAppConfig() NaisAppConfig {
	appConfig := NaisAppConfig{
		Image: image,
		Port:  port,
		Healthcheck: Healthcheck{
			Readiness: Probe{
				Path:         readinessPath,
				InitialDelay: 20,
			},
			Liveness: Probe{
				Path:         livenessPath,
				InitialDelay: 20,
			},
		},
		Resources: ResourceRequirements{
			Requests: ResourceList{
				Memory: memoryRequest,
				Cpu:    cpuRequest,
			},
			Limits: ResourceList{
				Memory: memoryLimit,
				Cpu:    cpuLimit,
			},
		},
		Prometheus: PrometheusConfig{
			Path:    "/path",
			Enabled: true,
		},
	}

	return appConfig

}

func TestResourceEnvironmentVariableName(t *testing.T) {
	t.Run("Resource should be underscored and uppercased", func(t *testing.T) {
		resource := NaisResource{
			"test.resource",
			"type",
			map[string]string{},
			map[string]string{},
			map[string][]byte{},
		}
		assert.Equal(t, "TEST_RESOURCE_KEY", ResourceEnvironmentVariableName(resource, "key"))
	})
}

func TestService(t *testing.T) {
	service := createServiceDef(appName, namespace)
	service.Spec.ClusterIP = clusterIP
	clientset := fake.NewSimpleClientset(service)

	t.Run("Fetching nonexistant service yields nil and no error", func(t *testing.T) {
		nonExistantService, err := getExistingService("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistantService)
	})

	t.Run("Fetching an existing service yields service and no error", func(t *testing.T) {
		existingService, err := getExistingService(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, service, existingService)
	})

	t.Run("when no service exists, a new one is created", func(t *testing.T) {
		service, err := createService(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName, Version: version}, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, service.ObjectMeta.Name)
		assert.Equal(t, DefaultPortName, service.Spec.Ports[0].TargetPort.StrVal)
		assert.Equal(t, map[string]string{"app": otherAppName}, service.Spec.Selector)
	})
	t.Run("when service exists, nothing happens", func(t *testing.T) {
		nilValue, err := createService(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})
}

func TestDeployment(t *testing.T) {
	newVersion := "14"
	resource1Name := "r1"
	resource1Type := "db"
	resource1Key := "key1"
	resource1Value := "value1"
	secret1Key := "password"
	secret1Value := "secret"
	cert1Key := "cert1key"
	cert1Value := []byte("cert1Value")

	resource2Name := "r2"
	resource2Type := "db"
	resource2Key := "key2"
	resource2Value := "value2"
	secret2Key := "password"
	secret2Value := "anothersecret"
	cert2Key := "cert2key"
	cert2Value := []byte("cert2Value")

	invalidlyNamedResourceNameDot := "dots.are.not.allowed"
	invalidlyNamedResourceTypeDot := "restservice"
	invalidlyNamedResourceKeyDot := "key"
	invalidlyNamedResourceValueDot := "value"
	invalidlyNamedResourceSecretKeyDot := "secretkey"
	invalidlyNamedResourceSecretValueDot := "secretvalue"

	invalidlyNamedResourceNameColon := "colon:are:not:allowed"
	invalidlyNamedResourceTypeColon := "restservice"
	invalidlyNamedResourceKeyColon := "key"
	invalidlyNamedResourceValueColon := "value"
	invalidlyNamedResourceSecretKeyColon := "secretkey"
	invalidlyNamedResourceSecretValueColon := "secretvalue"

	naisResources := []NaisResource{
		{
			resource1Name,
			resource1Type,
			map[string]string{resource1Key: resource1Value},
			map[string]string{secret1Key: secret1Value},
			map[string][]byte{cert1Key: cert1Value},
		},
		{
			resource2Name,
			resource2Type,
			map[string]string{resource2Key: resource2Value},
			map[string]string{secret2Key: secret2Value},
			map[string][]byte{cert2Key: cert2Value},
		},
		{
			"resource3",
			"applicationproperties",
			map[string]string{
				"key1": "value1",
			},
			map[string]string{},
			nil,
		},
		{
			"resource4",
			"applicationproperties",
			map[string]string{
				"key2.Property": "dc=preprod,dc=local",
			},
			map[string]string{},
			nil,
		},
		{
			invalidlyNamedResourceNameDot,
			invalidlyNamedResourceTypeDot,
			map[string]string{invalidlyNamedResourceKeyDot: invalidlyNamedResourceValueDot},
			map[string]string{invalidlyNamedResourceSecretKeyDot: invalidlyNamedResourceSecretValueDot},
			nil,
		},
		{
			invalidlyNamedResourceNameColon,
			invalidlyNamedResourceTypeColon,
			map[string]string{invalidlyNamedResourceKeyColon: invalidlyNamedResourceValueColon},
			map[string]string{invalidlyNamedResourceSecretKeyColon: invalidlyNamedResourceSecretValueColon},
			nil,
		},
	}

	deployment := createDeploymentDef(naisResources, newDefaultAppConfig(), NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, nil)
	deployment.ObjectMeta.ResourceVersion = resourceVersion

	clientset := fake.NewSimpleClientset(deployment)

	t.Run("Nonexistant deployment yields empty string and no error", func(t *testing.T) {
		nilValue, err := getExistingDeployment("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing deployment yields def and no error", func(t *testing.T) {
		id, err := getExistingDeployment(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, id.ObjectMeta.ResourceVersion)
	})

	t.Run("when no deployment exists, it's created", func(t *testing.T) {
		deployment, err := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName, Version: version}, newDefaultAppConfig(), naisResources, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, deployment.Name)
		assert.Equal(t, "", deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, otherAppName, deployment.Spec.Template.Name)

		containers := deployment.Spec.Template.Spec.Containers
		container := containers[0]
		assert.Equal(t, otherAppName, container.Name)
		assert.Equal(t, image+":"+version, container.Image)
		assert.Equal(t, int32(port), container.Ports[0].ContainerPort)
		assert.Equal(t, DefaultPortName, container.Ports[0].Name)
		assert.Equal(t, livenessPath, container.LivenessProbe.HTTPGet.Path)
		assert.Equal(t, readinessPath, container.ReadinessProbe.HTTPGet.Path)
		assert.Equal(t, intstr.FromString(DefaultPortName), container.ReadinessProbe.HTTPGet.Port)
		assert.Equal(t, intstr.FromString(DefaultPortName), container.LivenessProbe.HTTPGet.Port)
		assert.Equal(t, int32(20), deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds)
		assert.Equal(t, int32(20), deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds)

		ptr := func(p resource.Quantity) *resource.Quantity {
			return &p
		}
		assert.Equal(t, memoryRequest, ptr(container.Resources.Requests["memory"]).String())
		assert.Equal(t, memoryLimit, ptr(container.Resources.Limits["memory"]).String())
		assert.Equal(t, cpuRequest, ptr(container.Resources.Requests["cpu"]).String())
		assert.Equal(t, cpuLimit, ptr(container.Resources.Limits["cpu"]).String())
		assert.Equal(t, map[string]string{
			"prometheus.io/scrape": "true",
			"prometheus.io/path":   "/path",
			"prometheus.io/port":   "http",
		}, deployment.Spec.Template.Annotations)

		env := container.Env
		assert.Equal(t, 22, len(env))
		assert.Equal(t, "app_version", env[0].Name)
		assert.Equal(t, version, env[0].Value)
		assert.Equal(t, "APP_VERSION", env[1].Name)
		assert.Equal(t, version, env[1].Value)

		assert.Equal(t, resource1Name+"_"+resource1Key, env[2].Name)
		assert.Equal(t, "value1", env[2].Value)
		assert.Equal(t, strings.ToUpper(resource1Name+"_"+resource1Key), env[3].Name)
		assert.Equal(t, "value1", env[3].Value)

		assert.Equal(t, resource1Name+"_"+secret1Key, env[4].Name)
		assert.Equal(t, createSecretRef(otherAppName, secret1Key, resource1Name), env[4].ValueFrom)
		assert.Equal(t, strings.ToUpper(resource1Name+"_"+secret1Key), env[5].Name)
		assert.Equal(t, createSecretRef(otherAppName, secret1Key, resource1Name), env[5].ValueFrom)

		assert.Equal(t, resource2Name+"_"+resource2Key, env[6].Name)
		assert.Equal(t, "value2", env[6].Value)
		assert.Equal(t, strings.ToUpper(resource2Name+"_"+resource2Key), env[7].Name)
		assert.Equal(t, "value2", env[7].Value)

		assert.Equal(t, resource2Name+"_"+secret2Key, env[8].Name)
		assert.Equal(t, createSecretRef(otherAppName, secret2Key, resource2Name), env[8].ValueFrom)
		assert.Equal(t, strings.ToUpper(resource2Name+"_"+secret2Key), env[9].Name)
		assert.Equal(t, createSecretRef(otherAppName, secret2Key, resource2Name), env[9].ValueFrom)

		assert.Equal(t, "key1", env[10].Name)
		assert.Equal(t, "KEY1", env[11].Name)

		assert.Equal(t, "key2_Property", env[12].Name)
		assert.Equal(t, "KEY2_PROPERTY", env[13].Name)

		assert.Equal(t, "dots_are_not_allowed_key", env[14].Name)
		assert.Equal(t, "DOTS_ARE_NOT_ALLOWED_KEY", env[15].Name)

		assert.Equal(t, "dots_are_not_allowed_secretkey", env[16].Name)
		assert.Equal(t, "DOTS_ARE_NOT_ALLOWED_SECRETKEY", env[17].Name)

		assert.Equal(t, "colon_are_not_allowed_key", env[18].Name)
		assert.Equal(t, "COLON_ARE_NOT_ALLOWED_KEY", env[19].Name)

		assert.Equal(t, "colon_are_not_allowed_secretkey", env[20].Name)
		assert.Equal(t, "COLON_ARE_NOT_ALLOWED_SECRETKEY", env[21].Name)
	})

	t.Run("when a deployment exists, its updated", func(t *testing.T) {
		updatedDeployment, err := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: newVersion}, newDefaultAppConfig(), naisResources, clientset)
		assert.NoError(t, err)

		assert.Equal(t, resourceVersion, deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, appName, updatedDeployment.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].Name)
		assert.Equal(t, image+":"+newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, int32(port), updatedDeployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Env[0].Value)
	})

	t.Run("Prometheus annotations are updated on an existing deployment", func(t *testing.T) {

		appConfig := newDefaultAppConfig()
		appConfig.Prometheus.Path = "/newPath"
		appConfig.Prometheus.Enabled = false

		updatedDeployment, err := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, appConfig, naisResources, clientset)
		assert.NoError(t, err)

		assert.Equal(t, map[string]string{
			"prometheus.io/scrape": "false",
			"prometheus.io/path":   "/newPath",
			"prometheus.io/port":   "http",
		}, updatedDeployment.Spec.Template.Annotations)
	})

	t.Run("File secrets are mounted correctly for an updated deployment", func(t *testing.T) {

		updatedCertKey := "updatedkey"
		updatedCertValue := []byte("updatedCertValue")

		updatedResource := []NaisResource{
			{
				resource1Name,
				resource1Type,
				nil,
				nil,
				map[string][]byte{updatedCertKey: updatedCertValue},
			},
		}

		updatedDeployment, err := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, newDefaultAppConfig(), updatedResource, clientset)
		assert.NoError(t, err)

		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Volumes))
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Volumes[0].Name)
		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Volumes[0].Secret.Items))
		assert.Equal(t, updatedCertKey, updatedDeployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Key)

		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts))
		assert.Equal(t, "/var/run/secrets/naisd.io/", updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	})

	t.Run("File secrets are mounted correctly for a new deployment", func(t *testing.T) {
		deployment, _ := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, newDefaultAppConfig(), naisResources, clientset)

		assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Volumes))
		assert.Equal(t, appName, deployment.Spec.Template.Spec.Volumes[0].Name)
		assert.Equal(t, 2, len(deployment.Spec.Template.Spec.Volumes[0].Secret.Items))
		assert.Equal(t, cert1Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Key)
		assert.Equal(t, cert1Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Path)
		assert.Equal(t, cert2Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[1].Key)
		assert.Equal(t, cert2Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[1].Path)

		assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts))
		assert.Equal(t, "/var/run/secrets/naisd.io/", deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
		assert.Equal(t, appName, deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)

	})

	t.Run("No volume or volume mounts are added when application does not depende on a Fasit Certificate", func(t *testing.T) {
		resources := []NaisResource{
			{
				resource1Name,
				resource1Type,
				nil,
				nil,
				nil,
			},
		}

		deployment, err := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, newDefaultAppConfig(), resources, clientset)

		assert.NoError(t, err)

		spec := deployment.Spec.Template.Spec
		assert.Empty(t, spec.Volumes, "Unexpected volumes")
		assert.Empty(t, spec.Containers[0].VolumeMounts, "Unexpected volume mounts.")

	})
}

func TestIngress(t *testing.T) {
	appName := "appname"
	namespace := "default"
	subDomain := "example.no"
	ingress := createIngressDef(subDomain, appName, namespace)
	ingress.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(ingress)

	t.Run("Nonexistant ingress yields nil and no error", func(t *testing.T) {
		ingress, err := getExistingIngress("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, ingress)
	})

	t.Run("Existing ingress yields def and no error", func(t *testing.T) {
		existingIngress, err := getExistingIngress(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingIngress.ObjectMeta.ResourceVersion)
	})

	t.Run("when no ingress exists, a new one is created", func(t *testing.T) {
		ingress, err := createIngress(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName}, subDomain, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, ingress.ObjectMeta.Name)
		assert.Equal(t, otherAppName+"."+subDomain, ingress.Spec.Rules[0].Host)
		assert.Equal(t, otherAppName, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
	})

	t.Run("when ingress is created in non-default namespace, hostname is postfixed with namespace", func(t *testing.T) {
		namespace := "nondefault"
		ingress, err := createIngress(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName}, subDomain, clientset)
		assert.NoError(t, err)
		assert.Equal(t, otherAppName+"-"+namespace+"."+subDomain, ingress.Spec.Rules[0].Host)
	})

	t.Run("when an ingress exists, nothing happens", func(t *testing.T) {
		nilValue, err := createIngress(NaisDeploymentRequest{Namespace: namespace, Application: appName}, subDomain, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})
}

func TestCreateOrUpdateSecret(t *testing.T) {
	appName := "appname"
	namespace := "namespace"
	resource1Name := "r1"
	resource1Type := "db"
	resource1Key := "key1"
	resource1Value := "value1"
	secret1Key := "password"
	secret1Value := "secret"
	resource2Name := "r2"
	resource2Type := "db"
	resource2Key := "key2"
	resource2Value := "value2"
	secret2Key := "password"
	secret2Value := "anothersecret"
	fileKey1 := "fileKey1"
	fileKey2 := "fileKey2"
	fileValue1 := []byte("fileValue1")
	fileValue2 := []byte("fileValue2")
	files1 := map[string][]byte{fileKey1: fileValue1}
	files2 := map[string][]byte{fileKey2: fileValue2}

	naisResources := []NaisResource{
		{resource1Name, resource1Type, map[string]string{resource1Key: resource1Value}, map[string]string{secret1Key: secret1Value}, files1},
		{resource2Name, resource2Type, map[string]string{resource2Key: resource2Value}, map[string]string{secret2Key: secret2Value}, files2}}

	secret := createSecretDef(naisResources, nil, appName, namespace)
	secret.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(secret)

	t.Run("Nonexistant secret yields nil and no error", func(t *testing.T) {
		nilValue, err := getExistingSecret("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing secret yields def and no error", func(t *testing.T) {
		existingSecret, err := getExistingSecret(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingSecret.ObjectMeta.ResourceVersion)
	})

	t.Run("when no secret exists, a new one is created", func(t *testing.T) {
		secret, err := createOrUpdateSecret(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName}, naisResources, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, otherAppName, secret.ObjectMeta.Name)
		assert.Equal(t, 4, len(secret.Data))
		assert.Equal(t, []byte(secret1Value), secret.Data[resource1Name+"_"+secret1Key])
		assert.Equal(t, []byte(secret2Value), secret.Data[resource2Name+"_"+secret2Key])
		assert.Equal(t, fileValue1, secret.Data[fileKey1])
		assert.Equal(t, fileValue2, secret.Data[fileKey2])
	})

	t.Run("when a secret exists, it's updated", func(t *testing.T) {
		updatedSecretValue := "newsecret"
		updatedFileValue := []byte("newfile")
		secret, err := createOrUpdateSecret(NaisDeploymentRequest{Namespace: namespace, Application: appName}, []NaisResource{
			{resource1Name, resource1Type, nil, map[string]string{secret1Key: updatedSecretValue}, map[string][]byte{fileKey1: updatedFileValue}}}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, namespace, secret.ObjectMeta.Namespace)
		assert.Equal(t, appName, secret.ObjectMeta.Name)
		assert.Equal(t, []byte(updatedSecretValue), secret.Data[resource1Name+"_"+secret1Key])
		assert.Equal(t, updatedFileValue, secret.Data[fileKey1])
	})
}

func TestCreateOrUpdateAutoscaler(t *testing.T) {
	autoscaler := createOrUpdateAutoscalerDef(1, 2, 3, nil, appName, namespace)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler)

	t.Run("nonexistant autoscaler yields empty string and no error", func(t *testing.T) {
		nonExistingAutoscaler, err := getExistingAutoscaler("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistingAutoscaler)
	})

	t.Run("existing autoscaler yields id and no error", func(t *testing.T) {
		existingAutoscaler, err := getExistingAutoscaler(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingAutoscaler.ObjectMeta.ResourceVersion)
	})

	t.Run("when no autoscaler exists, a new one is created", func(t *testing.T) {
		autoscaler, err := createOrUpdateAutoscaler(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName}, NaisAppConfig{Replicas: Replicas{Max: 1, Min: 2, CpuThresholdPercentage: 69}}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, int32(1), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, int32p(2), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32p(69), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, namespace, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, otherAppName, autoscaler.ObjectMeta.Name)
		assert.Equal(t, otherAppName, autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})

	t.Run("when autoscaler exists, it's updated", func(t *testing.T) {
		cpuThreshold := 69
		minReplicas := 6
		maxReplicas := 9
		autoscaler, err := createOrUpdateAutoscaler(NaisDeploymentRequest{Namespace: namespace, Application: appName}, NaisAppConfig{Replicas: Replicas{CpuThresholdPercentage: cpuThreshold, Min: minReplicas, Max: maxReplicas}}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, namespace, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, appName, autoscaler.ObjectMeta.Name)
		assert.Equal(t, int32p(int32(cpuThreshold)), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, int32p(int32(minReplicas)), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32(maxReplicas), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, appName, autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})
}

func TestDNS1123ValidResourceNames(t *testing.T) {
	name := "key_underscore_Upper"
	naisResource := []NaisResource{
		{
			"name",
			"resourcrType",
			nil,
			nil,
			map[string][]byte{"key": []byte("value")},
		},
	}

	t.Run("Generate valid volume mount name", func(t *testing.T) {
		volumeMount := createCertificateVolumeMount(NaisDeploymentRequest{Namespace: namespace, Application: name}, naisResource)
		assert.Equal(t, "key-underscore-upper", volumeMount.Name)

	})

	t.Run("Generate valid volume name", func(t *testing.T) {
		volume := createCertificateVolume(NaisDeploymentRequest{Namespace: namespace, Application: name}, naisResource)
		assert.Equal(t, "key-underscore-upper", volume.Name)

	})

}

func TestCreateK8sResources(t *testing.T) {
	deploymentRequest := NaisDeploymentRequest{
		Application:  appName,
		Version:      version,
		Environment:  namespace,
		AppConfigUrl: "http://repo.com/app",
		Zone:         "zone",
		Namespace:    namespace,
	}

	appConfig := NaisAppConfig{
		Image: image,
		Port:  port,
		Resources: ResourceRequirements{
			Requests: ResourceList{
				Cpu:    cpuRequest,
				Memory: memoryRequest,
			},
			Limits: ResourceList{
				Cpu:    cpuLimit,
				Memory: memoryLimit,
			},
		},
	}

	naisResources := []NaisResource{
		{"resourceName", "resourceType", map[string]string{"resourceKey": "resource1Value"}, map[string]string{"secretKey": "secretValue"}, nil}}

	service := createServiceDef(appName, namespace)

	autoscaler := createOrUpdateAutoscalerDef(6, 9, 6, nil, appName, namespace)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler, service)

	t.Run("creates all resources", func(t *testing.T) {
		deploymentResult, error := createOrUpdateK8sResources(deploymentRequest, appConfig, naisResources, "nais.example.yo", clientset)
		assert.NoError(t, error)

		assert.NotEmpty(t, deploymentResult.Secret)
		assert.Nil(t, deploymentResult.Service, "nothing happens to service if it already exists")
		assert.NotEmpty(t, deploymentResult.Deployment)
		assert.NotEmpty(t, deploymentResult.Ingress)
		assert.NotEmpty(t, deploymentResult.Autoscaler)

		assert.Equal(t, resourceVersion, deploymentResult.Autoscaler.ObjectMeta.ResourceVersion, "autoscaler should have same id as the preexisting")
		assert.Equal(t, "", deploymentResult.Secret.ObjectMeta.ResourceVersion, "secret should not have any id set")
	})

	naisResourcesNoSecret := []NaisResource{
		{"resourceName", "resourceType", map[string]string{"resourceKey": "resource1Value"}, map[string]string{}, nil}}

	t.Run("omits secret creation when no secret resources ex", func(t *testing.T) {
		deploymentResult, error := createOrUpdateK8sResources(deploymentRequest, appConfig, naisResourcesNoSecret, "nais.example.yo", fake.NewSimpleClientset())
		assert.NoError(t, error)

		assert.Empty(t, deploymentResult.Secret)
		assert.NotEmpty(t, deploymentResult.Service)
	})

}

func createSecretRef(appName string, resKey string, resName string) *v1.EnvVarSource {
	return &v1.EnvVarSource{
		SecretKeyRef: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: appName,
			},
			Key: resName + "_" + resKey,
		},
	}
}
