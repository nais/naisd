package api

import (
	"os"
	"strings"
	"testing"

	"github.com/nais/naisd/api/constant"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	k8score "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	appName          = "appname"
	otherAppName     = "otherappname"
	teamName         = "aura"
	otherTeamName    = "bris"
	fasitEnvironment = "testenv"
	environment      = "environment"
	image            = "docker.hub/app"
	port             = 6900
	resourceVersion  = "12369"
	version          = "13"
	livenessPath     = "isAlive"
	readinessPath    = "isReady"
	cpuRequest       = "100m"
	cpuLimit         = "200m"
	memoryRequest    = "200Mi"
	memoryLimit      = "400Mi"
	clusterIP        = "1.2.3.4"
)

func newDefaultManifest() NaisManifest {
	manifest := NaisManifest{
		Image: image,
		Port:  port,
		Healthcheck: Healthcheck{
			Readiness: Probe{
				Path:             readinessPath,
				InitialDelay:     20,
				PeriodSeconds:    10,
				FailureThreshold: 3,
				Timeout:          2,
			},
			Liveness: Probe{
				Path:             livenessPath,
				InitialDelay:     20,
				PeriodSeconds:    10,
				FailureThreshold: 3,
				Timeout:          3,
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
		LeaderElection: false,
		Redis:          false,
		Team:           teamName,
	}

	return manifest

}

func TestService(t *testing.T) {
	service := createServiceDef(appName, environment, teamName)
	service.Spec.ClusterIP = clusterIP
	clientset := fake.NewSimpleClientset(service)

	t.Run("Fetching nonexistant service yields nil and no error", func(t *testing.T) {
		nonExistantService, err := getExistingService("nonexisting", teamName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistantService)
	})

	t.Run("Fetching an existing service yields service and no error", func(t *testing.T) {
		existingService, err := getExistingService(createObjectName(appName, environment), teamName, clientset)
		assert.NoError(t, err)
		assert.Equal(t, service, existingService)
	})

	t.Run("when no service exists, a new one is created", func(t *testing.T) {
		service, err := createService(naisrequest.Deploy{Environment: environment, Application: otherAppName, Version: version}, otherTeamName, clientset)

		assert.NoError(t, err)
		assert.Equal(t, createObjectName(otherAppName, environment), service.ObjectMeta.Name)
		assert.Equal(t, otherTeamName, service.ObjectMeta.Labels["team"])
		assert.Equal(t, DefaultPortName, service.Spec.Ports[0].TargetPort.StrVal)
		assert.Equal(t, "http", service.Spec.Ports[0].Name)
		assert.Equal(t, map[string]string{"app": otherAppName, "environment": environment}, service.Spec.Selector)
	})
	t.Run("when service exists, nothing happens", func(t *testing.T) {
		nilValue, err := createService(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, teamName, clientset)
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
	resource2KeyMapping := "MY_KEY2"
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
			1,
			resource1Name,
			resource1Type,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{resource1Key: resource1Value},
			map[string]string{},
			map[string]string{secret1Key: secret1Value},
			nil,
			nil,
		},
		{
			1,
			resource2Name,
			resource2Type,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{resource2Key: resource2Value},
			map[string]string{
				resource2Key: resource2KeyMapping,
			},
			map[string]string{secret2Key: secret2Value},
			nil,
			nil,
		},
		{
			1,
			"resource3",
			"applicationproperties",
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{
				"key1": "value1",
			},
			map[string]string{},
			map[string]string{},
			nil,
			nil,
		},
		{
			1,
			"resource4",
			"applicationproperties",
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{
				"key2.Property": "dc=preprod,dc=local",
			},
			map[string]string{},
			map[string]string{},
			nil,
			nil,
		},
		{
			1,
			invalidlyNamedResourceNameDot,
			invalidlyNamedResourceTypeDot,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{invalidlyNamedResourceKeyDot: invalidlyNamedResourceValueDot},
			map[string]string{},
			map[string]string{invalidlyNamedResourceSecretKeyDot: invalidlyNamedResourceSecretValueDot},
			nil,
			nil,
		},
		{
			1,
			invalidlyNamedResourceNameColon,
			invalidlyNamedResourceTypeColon,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{invalidlyNamedResourceKeyColon: invalidlyNamedResourceValueColon},
			map[string]string{},
			map[string]string{invalidlyNamedResourceSecretKeyColon: invalidlyNamedResourceSecretValueColon},
			nil,
			nil,
		},
	}

	naisCertResources := []NaisResource{
		{
			1,
			resource1Name,
			"certificate",
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{resource1Key: resource1Value},
			map[string]string{},
			map[string]string{secret1Key: secret1Value},
			map[string][]byte{cert1Key: cert1Value},
			nil,
		},
		{
			1,
			resource2Name,
			resource2Type,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{resource2Key: resource2Value},
			map[string]string{
				resource2Key: resource2KeyMapping,
			},
			map[string]string{secret2Key: secret2Value},
			map[string][]byte{cert2Key: cert2Value},
			nil,
		},
	}

	deployment, err := createDeploymentDef(naisResources, newDefaultManifest(), naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, nil, false)

	assert.Nil(t, err)

	deployment.ObjectMeta.ResourceVersion = resourceVersion

	clientset := fake.NewSimpleClientset(deployment)

	t.Run("Nonexistant deployment yields empty string and no error", func(t *testing.T) {
		nilValue, err := getExistingDeployment("nonexisting", teamName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing deployment yields def and no error", func(t *testing.T) {
		id, err := getExistingDeployment(createObjectName(appName, environment), teamName, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, id.ObjectMeta.ResourceVersion)
	})

	t.Run("when no deployment exists, it's created", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.Istio.Enabled = true
		deployment, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: otherAppName, Version: version, FasitEnvironment: fasitEnvironment}, manifest, naisResources, true, clientset)
		otherObjectName := createObjectName(otherAppName, environment)

		assert.NoError(t, err)
		assert.Equal(t, otherObjectName, deployment.Name)
		assert.Equal(t, "", deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, otherObjectName, deployment.Spec.Template.Name)

		containers := deployment.Spec.Template.Spec.Containers

		assert.Len(t, containers, 1, "Simple check for no sidecar containers")

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
		assert.Equal(t, int32(3), deployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds)
		assert.Equal(t, int32(2), deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds)
		assert.Equal(t, k8score.Lifecycle{}, *deployment.Spec.Template.Spec.Containers[0].Lifecycle)

		ptr := func(p resource.Quantity) *resource.Quantity {
			return &p
		}

		assert.Equal(t, memoryRequest, ptr(container.Resources.Requests["memory"]).String())
		assert.Equal(t, memoryLimit, ptr(container.Resources.Limits["memory"]).String())
		assert.Equal(t, cpuRequest, ptr(container.Resources.Requests["cpu"]).String())
		assert.Equal(t, cpuLimit, ptr(container.Resources.Limits["cpu"]).String())
		assert.Equal(t, map[string]string{
			"prometheus.io/scrape":    "true",
			"prometheus.io/path":      "/path",
			"prometheus.io/port":      "http",
			"sidecar.istio.io/inject": "true",
		}, deployment.Spec.Template.Annotations)

		env := container.Env
		assert.Equal(t, 14, len(env))
		assert.Equal(t, "APP_NAME", env[0].Name)
		assert.Equal(t, otherAppName, env[0].Value)
		assert.Equal(t, "APP_VERSION", env[1].Name)
		assert.Equal(t, version, env[1].Value)
		assert.Equal(t, environment, env[2].Value)
		assert.Equal(t, "FASIT_ENVIRONMENT_NAME", env[3].Name)
		assert.Equal(t, fasitEnvironment, env[3].Value)
		assert.Equal(t, resource2KeyMapping, env[6].Name)
		assert.Equal(t, "value2", env[6].Value)

		assert.Equal(t, strings.ToUpper(resource2Name+"_"+secret2Key), env[7].Name)
		assert.Equal(t, createSecretRef(createObjectName(otherAppName, environment), secret2Key, resource2Name), env[7].ValueFrom)

		assert.Equal(t, "KEY1", env[8].Name)
		assert.Equal(t, "KEY2_PROPERTY", env[9].Name)
		assert.Equal(t, "DOTS_ARE_NOT_ALLOWED_KEY", env[10].Name)
		assert.Equal(t, "DOTS_ARE_NOT_ALLOWED_SECRETKEY", env[11].Name)
		assert.Equal(t, "COLON_ARE_NOT_ALLOWED_KEY", env[12].Name)
		assert.Equal(t, "COLON_ARE_NOT_ALLOWED_SECRETKEY", env[13].Name)
		assert.False(t, manifest.LeaderElection, "LeaderElection should default to false")
		assert.False(t, manifest.Redis, "Redis should default to false")
	})

	t.Run("when fasit is skipped, FAIST_ENVIRONMENT_NAME is not set", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.Istio.Enabled = true
		deployment, _ := createOrUpdateDeployment(naisrequest.Deploy{
			Environment: environment,
			Application: appName,
			Version:     version,
			SkipFasit:   true,
		}, manifest, []NaisResource{}, false, clientset)

		containers := deployment.Spec.Template.Spec.Containers
		container := containers[0]

		env := container.Env
		assert.Equal(t, 3, len(env))
		assert.Equal(t, "APP_NAME", env[0].Name)
		assert.Equal(t, appName, env[0].Value)
		assert.Equal(t, "APP_VERSION", env[1].Name)
		assert.Equal(t, version, env[1].Value)
		assert.False(t, manifest.LeaderElection, "LeaderElection should default to false")
		assert.False(t, manifest.Redis, "Redis should default to false")
	})

	t.Run("when a deployment exists, its updated", func(t *testing.T) {
		updatedDeployment, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: newVersion}, newDefaultManifest(), naisResources, false, clientset)
		assert.NoError(t, err)

		assert.Equal(t, resourceVersion, deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, createObjectName(appName, environment), updatedDeployment.Name)
		assert.Equal(t, createObjectName(appName, environment), updatedDeployment.Spec.Template.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].Name)
		assert.Equal(t, image+":"+newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, int32(port), updatedDeployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Env[1].Value)
	})

	t.Run("when leaderElection is true, extra container exists", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.LeaderElection = true
		deployment, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
		assert.NoError(t, err)

		containers := deployment.Spec.Template.Spec.Containers
		assert.Len(t, containers, 2, "Simple check to see if leader-elector has been added")

		container := getSidecarContainer(containers, "elector")
		assert.NotNil(t, container)
	})

	t.Run("Prometheus annotations are updated on an existing deployment", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.Prometheus.Path = "/newPath"
		manifest.Prometheus.Enabled = false

		updatedDeployment, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
		assert.NoError(t, err)

		assert.Equal(t, map[string]string{
			"prometheus.io/scrape": "false",
			"prometheus.io/path":   "/newPath",
			"prometheus.io/port":   "http",
		}, updatedDeployment.Spec.Template.Annotations)
	})

	t.Run("when logformat and logtransform is set, annotations exists", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.Logformat = "accesslog"
		manifest.Logtransform = "dns_loglevel"

		updateDeployment, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"prometheus.io/scrape": "true",
			"prometheus.io/path":   "/path",
			"prometheus.io/port":   "http",
			"nais.io/logformat":    "accesslog",
			"nais.io/logtransform": "dns_loglevel",
		}, updateDeployment.Spec.Template.Annotations)
	})

	t.Run("Container lifecycle is set correctly", func(t *testing.T) {
		path := "/stop"

		manifest := newDefaultManifest()
		manifest.PreStopHookPath = path

		d, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
		assert.NoError(t, err)
		assert.Equal(t, path, d.Spec.Template.Spec.Containers[0].Lifecycle.PreStop.HTTPGet.Path)
		assert.Equal(t, intstr.FromString(DefaultPortName), d.Spec.Template.Spec.Containers[0].Lifecycle.PreStop.HTTPGet.Port)

	})

	t.Run("File secrets are mounted correctly for an updated deployment", func(t *testing.T) {
		updatedCertKey := "updatedkey"
		updatedCertValue := []byte("updatedCertValue")

		updatedResource := []NaisResource{
			{
				1,
				resource1Name,
				resource1Type,
				Scope{"u", "u1", constant.ZONE_FSS},
				nil,
				nil,
				nil,
				map[string][]byte{updatedCertKey: updatedCertValue},
				nil,
			},
		}

		updatedDeployment, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), updatedResource, false, clientset)
		assert.NoError(t, err)

		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Volumes))
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Volumes[0].Name)
		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Volumes[0].Secret.Items))
		assert.Equal(t, resource1Name+"_"+updatedCertKey, updatedDeployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Key)

		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts))
		assert.Equal(t, "/var/run/secrets/naisd.io/", updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	})

	t.Run("File secrets are mounted correctly for a new deployment", func(t *testing.T) {
		deployment, _ := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), naisCertResources, false, clientset)

		assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Volumes))
		assert.Equal(t, appName, deployment.Spec.Template.Spec.Volumes[0].Name)
		assert.Equal(t, 2, len(deployment.Spec.Template.Spec.Volumes[0].Secret.Items))
		assert.Equal(t, resource1Name+"_"+cert1Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Key)
		assert.Equal(t, resource1Name+"_"+cert1Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Path)
		assert.Equal(t, resource2Name+"_"+cert2Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[1].Key)
		assert.Equal(t, resource2Name+"_"+cert2Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[1].Path)

		assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts))
		assert.Equal(t, "/var/run/secrets/naisd.io/", deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
		assert.Equal(t, appName, deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)

	})

	t.Run("Env variable is created for file secrets ", func(t *testing.T) {
		deployment, _ := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), naisCertResources, false, clientset)

		envVars := deployment.Spec.Template.Spec.Containers[0].Env

		assert.Equal(t, 10, len(envVars))
		assert.Equal(t, "R1_CERT1KEY", envVars[6].Name)
		assert.Equal(t, "/var/run/secrets/naisd.io/r1_cert1key", envVars[6].Value)
		assert.Equal(t, "R2_CERT2KEY", envVars[9].Name)
		assert.Equal(t, "/var/run/secrets/naisd.io/r2_cert2key", envVars[9].Value)

	})

	t.Run("No volume or volume mounts are added when application does not depende on a Fasit Certificate", func(t *testing.T) {
		resources := []NaisResource{
			{
				1,
				resource1Name,
				resource1Type,
				Scope{"u", "u1", constant.ZONE_FSS},
				nil,
				nil,
				nil,
				nil,
				nil,
			},
		}

		deployment, err := createOrUpdateDeployment(naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), resources, false, clientset)

		assert.NoError(t, err)

		spec := deployment.Spec.Template.Spec
		assert.Empty(t, spec.Volumes, "Unexpected volumes")
		assert.Empty(t, spec.Containers[0].VolumeMounts, "Unexpected volume mounts.")

	})

	t.Run("duplicate environment variables should error", func(t *testing.T) {
		resource1 := NaisResource{
			name:         "srvapp",
			resourceType: "credential",
			properties:   map[string]string{},
			secret: map[string]string{
				"password": "foo",
			},
		}
		resource2 := NaisResource{
			name:         "srvapp",
			resourceType: "certificate",
			properties:   map[string]string{},
			secret: map[string]string{
				"password": "bar",
			},
		}

		deploymentRequest := naisrequest.Deploy{
			Environment: "default",
			Application: "myapp",
			Version:     "1",
		}

		_, err := createOrUpdateDeployment(deploymentRequest, newDefaultManifest(), []NaisResource{resource1, resource2}, false, clientset)

		assert.NotNil(t, err)
		assert.Equal(t, "unable to create deployment: found duplicate environment variable SRVAPP_PASSWORD when adding password for srvapp (certificate)"+
			" Change the Fasit alias or use propertyMap to create unique variable names", err.Error())
	})
	t.Run("Injects envoy sidecar based on settings", func(t *testing.T) {
		deploymentRequest := naisrequest.Deploy{
			Environment: "default",
			Application: "myapp",
			Version:     "1",
		}

		istioDisabledManifest := NaisManifest{Istio: IstioConfig{Enabled: false}}
		istioEnabledManifest := NaisManifest{Istio: IstioConfig{Enabled: true}}

		assert.Equal(t, createPodObjectMetaWithAnnotations(deploymentRequest, istioDisabledManifest, false).Annotations["sidecar.istio.io/inject"], "")
		assert.Equal(t, createPodObjectMetaWithAnnotations(deploymentRequest, istioEnabledManifest, false).Annotations["sidecar.istio.io/inject"], "")
		assert.Equal(t, createPodObjectMetaWithAnnotations(deploymentRequest, istioDisabledManifest, true).Annotations["sidecar.istio.io/inject"], "")
		assert.Equal(t, createPodObjectMetaWithAnnotations(deploymentRequest, istioEnabledManifest, true).Annotations["sidecar.istio.io/inject"], "true")
	})
}

func TestIngress(t *testing.T) {
	appName := "appname"
	environment := "default"
	subDomain := "example.no"
	istioCertSecretName := "istio-ingress-certs"
	ingress := createIngressDef(appName, environment, teamName)
	ingress.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(ingress)

	t.Run("Nonexistant ingress yields nil and no error", func(t *testing.T) {
		ingress, err := getExistingIngress("nonexisting", teamName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, ingress)
	})

	t.Run("Existing ingress yields def and no error", func(t *testing.T) {
		existingIngress, err := getExistingIngress(createObjectName(appName, environment), teamName, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingIngress.ObjectMeta.ResourceVersion)
	})

	t.Run("when no ingress exists, a default ingress is created", func(t *testing.T) {
		ingress, err := createOrUpdateIngress(naisrequest.Deploy{Environment: environment, Application: otherAppName}, otherTeamName, subDomain, []NaisResource{}, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, ingress.ObjectMeta.Name)
		assert.Equal(t, otherTeamName, ingress.ObjectMeta.Labels["team"])
		assert.Equal(t, 1, len(ingress.Spec.Rules))
		assert.Equal(t, otherAppName+"."+subDomain, ingress.Spec.Rules[0].Host)
		assert.Equal(t, otherAppName, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
		assert.Equal(t, istioCertSecretName, ingress.Spec.TLS[0].SecretName)
	})

	t.Run("when ingress is created in non-default environment, hostname is postfixed with environment", func(t *testing.T) {
		environment := "nondefault"
		ingress, err := createOrUpdateIngress(naisrequest.Deploy{Environment: environment, Application: otherAppName}, teamName, subDomain, []NaisResource{}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, otherAppName+"-"+environment+"."+subDomain, ingress.Spec.Rules[0].Host)
	})

	t.Run("Nais ingress resources are added", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(ingress) //Avoid interfering with other tests in suite.
		naisResources := []NaisResource{
			{
				resourceType: "LoadBalancerConfig",
				ingresses: map[string]string{
					"app.adeo.no": "context",
				},
			},
			{
				resourceType: "LoadBalancerConfig",
				ingresses: map[string]string{
					"app2.adeo.no": "context2",
				},
			},
		}
		ingress, err := createOrUpdateIngress(naisrequest.Deploy{Environment: environment, Application: otherAppName}, teamName, subDomain, naisResources, clientset)

		assert.NoError(t, err)
		assert.Equal(t, 3, len(ingress.Spec.Rules))

		assert.Equal(t, "app.adeo.no", ingress.Spec.Rules[1].Host)
		assert.Equal(t, 1, len(ingress.Spec.Rules[1].HTTP.Paths))
		assert.Equal(t, "/context", ingress.Spec.Rules[1].HTTP.Paths[0].Path)

		assert.Equal(t, "app2.adeo.no", ingress.Spec.Rules[2].Host)
		assert.Equal(t, 1, len(ingress.Spec.Rules[1].HTTP.Paths))
		assert.Equal(t, "/context2", ingress.Spec.Rules[2].HTTP.Paths[0].Path)

	})

	t.Run("sbs ingress are added", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(ingress) //Avoid interfering with other tests in suite.
		var naisResources []NaisResource

		ingress, err := createOrUpdateIngress(naisrequest.Deploy{Environment: environment, Application: "testapp", Zone: constant.ZONE_SBS, FasitEnvironment: "testenv"}, teamName, subDomain, naisResources, clientset)
		rules := ingress.Spec.Rules

		assert.NoError(t, err)
		assert.Equal(t, 2, len(rules))

		firstRule := rules[0]
		assert.Equal(t, "testapp.example.no", firstRule.Host)
		assert.Equal(t, 1, len(firstRule.HTTP.Paths))
		assert.Equal(t, "/", firstRule.HTTP.Paths[0].Path)

		secondRule := rules[1]
		assert.Equal(t, "tjenester-testenv.nav.no", secondRule.Host)
		assert.Equal(t, 1, len(secondRule.HTTP.Paths))
		assert.Equal(t, "/testapp", secondRule.HTTP.Paths[0].Path)
	})

}

func TestCreateOrUpdateSecret(t *testing.T) {
	appName := "appname"
	environment := "environment"
	resource1Name := "r1.alias"
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
		{
			1,
			resource1Name,
			resource1Type,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{resource1Key: resource1Value},
			map[string]string{},
			map[string]string{secret1Key: secret1Value},
			files1,
			nil,
		}, {
			1,
			resource2Name,
			resource2Type,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{resource2Key: resource2Value},
			map[string]string{},
			map[string]string{secret2Key: secret2Value},
			files2,
			nil,
		},
	}

	secret := createSecretDef(naisResources, nil, appName, environment, teamName)
	secret.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(secret)

	t.Run("Nonexistant secret yields nil and no error", func(t *testing.T) {
		nilValue, err := getExistingSecret("nonexisting", teamName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing secret yields def and no error", func(t *testing.T) {
		existingSecret, err := getExistingSecret(createObjectName(appName, environment), teamName, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingSecret.ObjectMeta.ResourceVersion)
	})

	t.Run("when no secret exists, a new one is created", func(t *testing.T) {
		secret, err := createOrUpdateSecret(naisrequest.Deploy{Environment: environment, Application: otherAppName}, naisResources, clientset, otherTeamName)
		assert.NoError(t, err)
		assert.Equal(t, "", secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, createObjectName(otherAppName, environment), secret.ObjectMeta.Name)
		assert.Equal(t, otherTeamName, secret.ObjectMeta.Labels["team"])
		assert.Equal(t, 4, len(secret.Data))
		assert.Equal(t, []byte(secret1Value), secret.Data[naisResources[0].ToResourceVariable(secret1Key)])
		assert.Equal(t, []byte(secret2Value), secret.Data[naisResources[1].ToResourceVariable(secret2Key)])
		assert.Equal(t, fileValue1, secret.Data[naisResources[0].ToResourceVariable(fileKey1)])
		assert.Equal(t, fileValue2, secret.Data[naisResources[1].ToResourceVariable(fileKey2)])
	})

	t.Run("when a secret exists, it's updated", func(t *testing.T) {
		updatedSecretValue := "newsecret"
		updatedFileValue := []byte("newfile")
		secret, err := createOrUpdateSecret(naisrequest.Deploy{Environment: environment, Application: appName}, []NaisResource{
			{
				1,
				resource1Name,
				resource1Type,
				Scope{"u", "u1", constant.ZONE_FSS},
				nil,
				map[string]string{},
				map[string]string{secret1Key: updatedSecretValue},
				map[string][]byte{fileKey1: updatedFileValue},
				nil,
			},
		}, clientset, teamName)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, teamName, secret.ObjectMeta.Namespace)
		assert.Equal(t, createObjectName(appName, environment), secret.ObjectMeta.Name)
		assert.Equal(t, teamName, secret.ObjectMeta.Labels["team"])
		assert.Equal(t, environment, secret.ObjectMeta.Labels["environment"])
		assert.Equal(t, []byte(updatedSecretValue), secret.Data["r1_alias_password"])
		assert.Equal(t, updatedFileValue, secret.Data["r1_alias_filekey1"])
	})
}

func TestCreateOrUpdateAutoscaler(t *testing.T) {
	autoscaler := createOrUpdateAutoscalerDef(1, 2, 3, nil, appName, environment, teamName)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler)

	t.Run("nonexistant autoscaler yields empty string and no error", func(t *testing.T) {
		nonExistingAutoscaler, err := getExistingAutoscaler("nonexisting", teamName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistingAutoscaler)
	})

	t.Run("existing autoscaler yields id and no error", func(t *testing.T) {
		existingAutoscaler, err := getExistingAutoscaler(createObjectName(appName, environment), teamName, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingAutoscaler.ObjectMeta.ResourceVersion)
	})

	t.Run("when no autoscaler exists, a new one is created", func(t *testing.T) {
		autoscaler, err := createOrUpdateAutoscaler(naisrequest.Deploy{Environment: environment, Application: otherAppName}, NaisManifest{Replicas: Replicas{Max: 1, Min: 2, CpuThresholdPercentage: 69}, Team: otherTeamName}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, int32(1), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, int32p(2), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32p(69), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, otherTeamName, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, createObjectName(otherAppName, environment), autoscaler.ObjectMeta.Name)
		assert.Equal(t, otherTeamName, autoscaler.ObjectMeta.Labels["team"])
		assert.Equal(t, environment, autoscaler.ObjectMeta.Labels["environment"])
		assert.Equal(t, createObjectName(otherAppName, environment), autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})

	t.Run("when autoscaler exists, it's updated", func(t *testing.T) {
		cpuThreshold := 69
		minReplicas := 6
		maxReplicas := 9
		autoscaler, err := createOrUpdateAutoscaler(naisrequest.Deploy{Environment: environment, Application: appName}, NaisManifest{Replicas: Replicas{CpuThresholdPercentage: cpuThreshold, Min: minReplicas, Max: maxReplicas}, Team: teamName}, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, autoscaler)
		assert.Equal(t, resourceVersion, autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, teamName, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, createObjectName(appName, environment), autoscaler.ObjectMeta.Name)
		assert.Equal(t, teamName, autoscaler.ObjectMeta.Labels["team"])
		assert.Equal(t, environment, autoscaler.ObjectMeta.Labels["environment"])
		assert.Equal(t, int32p(int32(cpuThreshold)), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, int32p(int32(minReplicas)), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32(maxReplicas), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, createObjectName(appName, environment), autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})
}

func TestDNS1123ValidResourceNames(t *testing.T) {
	name := "key_underscore_Upper"
	naisResource := []NaisResource{
		{
			1,
			"name",
			"resourcrType",
			Scope{"u", "u1", constant.ZONE_FSS},
			nil,
			nil,
			nil,
			map[string][]byte{"key": []byte("value")},
			nil,
		},
	}

	t.Run("Generate valid volume mount name", func(t *testing.T) {
		volumeMount := createCertificateVolumeMount(naisrequest.Deploy{Environment: environment, Application: name}, naisResource)
		assert.Equal(t, "key-underscore-upper", volumeMount.Name)

	})

	t.Run("Generate valid volume name", func(t *testing.T) {
		volume := createCertificateVolume(naisrequest.Deploy{Environment: environment, Application: name}, naisResource)
		assert.Equal(t, "key-underscore-upper", volume.Name)

	})

}

func TestCreateK8sResources(t *testing.T) {
	deploymentRequest := naisrequest.Deploy{
		Application:      appName,
		Version:          version,
		FasitEnvironment: environment,
		ManifestUrl:      "http://repo.com/app",
		Zone:             "zone",
		Environment:      environment,
	}

	manifest := NaisManifest{
		Image:   image,
		Port:    port,
		Team:    teamName,
		Ingress: Ingress{Disabled: false},
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
		{
			1,
			"resourceName",
			"resourceType",
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{"resourceKey": "resource1Value"},
			nil,
			map[string]string{"secretKey": "secretValue"},
			nil,
			nil,
		},
	}

	service := createServiceDef(appName, environment, teamName)

	autoscaler := createOrUpdateAutoscalerDef(6, 9, 6, nil, appName, environment, teamName)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler, service)

	t.Run("creates all resources", func(t *testing.T) {
		deploymentResult, err := createOrUpdateK8sResources(deploymentRequest, manifest, naisResources, "nais.example.yo", false, clientset)
		assert.NoError(t, err)

		assert.NotEmpty(t, deploymentResult.Secret)
		assert.Nil(t, deploymentResult.Service, "nothing happens to service if it already exists")
		assert.NotEmpty(t, deploymentResult.Deployment)
		assert.NotEmpty(t, deploymentResult.Ingress)
		assert.NotEmpty(t, deploymentResult.Autoscaler)
		assert.NotEmpty(t, deploymentResult.ServiceAccount)

		assert.Equal(t, resourceVersion, deploymentResult.Autoscaler.ObjectMeta.ResourceVersion, "autoscaler should have same id as the preexisting")
		assert.Equal(t, "", deploymentResult.Secret.ObjectMeta.ResourceVersion, "secret should not have any id set")
	})

	naisResourcesNoSecret := []NaisResource{
		{
			1,
			"resourceName",
			"resourceType",
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{"resourceKey": "resource1Value"},
			map[string]string{},
			map[string]string{},
			nil,
			nil,
		},
	}

	t.Run("omits secret creation when no secret resources ex", func(t *testing.T) {
		deploymentResult, err := createOrUpdateK8sResources(deploymentRequest, manifest, naisResourcesNoSecret, "nais.example.yo", false, fake.NewSimpleClientset())
		assert.NoError(t, err)

		assert.Empty(t, deploymentResult.Secret)
		assert.NotEmpty(t, deploymentResult.Service)
	})

	t.Run("omits ingress creation when disabled", func(t *testing.T) {
		manifest.Ingress.Disabled = true

		deploymentResult, err := createOrUpdateK8sResources(deploymentRequest, manifest, naisResourcesNoSecret, "nais.example.yo", false, fake.NewSimpleClientset())
		assert.NoError(t, err)

		assert.Empty(t, deploymentResult.Ingress)
	})

}

func TestCheckForDuplicates(t *testing.T) {
	t.Run("duplicate fasitEnvironment variables should error", func(t *testing.T) {
		resource1 := NaisResource{
			name:         "srvapp",
			resourceType: "credential",
			properties:   map[string]string{},
			secret: map[string]string{
				"password": "foo",
			},
		}
		resource2 := NaisResource{
			name:         "srvapp",
			resourceType: "certificate",
			properties:   map[string]string{},
			secret: map[string]string{
				"password": "bar",
			},
		}

		deploymentRequest := naisrequest.Deploy{
			Application: "myapp",
			Version:     "1",
		}

		_, err := createEnvironmentVariables(deploymentRequest, NaisManifest{}, []NaisResource{resource1, resource2})

		assert.NotNil(t, err)
		assert.Equal(t, "found duplicate environment variable SRVAPP_PASSWORD when adding password for srvapp (certificate)"+
			" Change the Fasit alias or use propertyMap to create unique variable names", err.Error())
	})

	t.Run("duplicate secret key ref should error", func(t *testing.T) {
		envVar1 := k8score.EnvVar{
			Name: "MY_PASSWORD",
			ValueFrom: &k8score.EnvVarSource{
				SecretKeyRef: &k8score.SecretKeySelector{
					Key: "my_password",
				},
			},
		}
		envVar2 := k8score.EnvVar{
			Name: "OTHER_PASSWORD",
			ValueFrom: &k8score.EnvVarSource{
				SecretKeyRef: &k8score.SecretKeySelector{
					Key: "my_password",
				},
			},
		}
		resource2 := NaisResource{
			name:         "other",
			resourceType: "credential",
			properties:   map[string]string{},
		}

		err := checkForDuplicates([]k8score.EnvVar{envVar1}, envVar2, "password", resource2)

		assert.NotNil(t, err)
		assert.Equal(t, "found duplicate secret key ref my_password between MY_PASSWORD and OTHER_PASSWORD when adding password for other (credential)"+
			" Change the Fasit alias or use propertyMap to create unique variable names", err.Error())
	})
}

func TestInjectProxySettings(t *testing.T) {
	t.Run("proxy settings not be injected in the pod unless requested through manifest", func(t *testing.T) {
		deploymentRequest := naisrequest.Deploy{
			Application: "myapp",
			Version:     "1",
		}

		manifest := NaisManifest{
			WebProxy: false,
		}

		os.Setenv("HTTP_PROXY", "foo")
		os.Setenv("HTTPS_PROXY", "bar")
		os.Setenv("NO_PROXY", "baz")

		env, err := createEnvironmentVariables(deploymentRequest, manifest, []NaisResource{})

		assert.Nil(t, err)
		assert.NotContains(t, env, k8score.EnvVar{Name: "HTTP_PROXY", Value: "foo"})
		assert.NotContains(t, env, k8score.EnvVar{Name: "HTTPS_PROXY", Value: "bar"})
		assert.NotContains(t, env, k8score.EnvVar{Name: "NO_PROXY", Value: "baz"})
	})

	t.Run("proxy settings should be injected in the pod if requested through manifest", func(t *testing.T) {
		deploymentRequest := naisrequest.Deploy{
			Application: "myapp",
			Version:     "1",
		}

		manifest := NaisManifest{
			WebProxy: true,
		}

		os.Setenv("HTTP_PROXY", "foo")
		os.Setenv("https_proxy", "bar")
		os.Setenv("NO_PROXY", "baz")

		env, err := createEnvironmentVariables(deploymentRequest, manifest, []NaisResource{})

		assert.Nil(t, err)
		assert.Contains(t, env, k8score.EnvVar{Name: "HTTP_PROXY", Value: "foo"})
		assert.Contains(t, env, k8score.EnvVar{Name: "HTTPS_PROXY", Value: "bar"})
		assert.Contains(t, env, k8score.EnvVar{Name: "NO_PROXY", Value: "baz"})
		assert.Contains(t, env, k8score.EnvVar{Name: "http_proxy", Value: "foo"})
		assert.Contains(t, env, k8score.EnvVar{Name: "https_proxy", Value: "bar"})
		assert.Contains(t, env, k8score.EnvVar{Name: "no_proxy", Value: "baz"})
	})
}

func TestCreateSBSPublicHostname(t *testing.T) {

	t.Run("p", func(t *testing.T) {
		assert.Equal(t, "tjenester.nav.no", createSBSPublicHostname(naisrequest.Deploy{FasitEnvironment: "p"}))
		assert.Equal(t, "tjenester-t6.nav.no", createSBSPublicHostname(naisrequest.Deploy{FasitEnvironment: "t6"}))
		assert.Equal(t, "tjenester-q6.nav.no", createSBSPublicHostname(naisrequest.Deploy{FasitEnvironment: "q6"}))
	})
}

func TestCreateObjectMeta(t *testing.T) {
	t.Run("Test required metadata field values", func(t *testing.T) {
		objectMeta := generateObjectMeta(appName, environment, teamName)

		assert.Equal(t, teamName, objectMeta.Labels["team"], "Team label should be equal to team name.")
		assert.Equal(t, appName, objectMeta.Labels["app"], "App label should be equal to app name.")
		assert.Equal(t, createObjectName(appName, environment), objectMeta.Name, "Resource name should equal app name.")
		assert.Equal(t, teamName, objectMeta.Namespace, "Resource environment should equal environment.")
	})

	t.Run("Test creating objectmeta without team name", func(t *testing.T) {
		objectMetaWithoutTeamName := generateObjectMeta(appName, environment, "")
		_, ok := objectMetaWithoutTeamName.Labels["team"]
		assert.False(t, ok, "Team label should not be set when team name is empty.")
	})
}

func TestMergeObjectMeta(t *testing.T) {
	t.Run("Test merging objectmeta", func(t *testing.T) {
		existingObjectMeta := generateObjectMeta(appName, environment, teamName)
		existingObjectMeta.ResourceVersion = "asd"

		newObjectMeta := generateObjectMeta(otherAppName, environment, otherTeamName)

		mergedObjectMeta := mergeObjectMeta(existingObjectMeta, newObjectMeta)

		assert.Equal(t, otherTeamName, mergedObjectMeta.Labels["team"], "Team label should be equal to team name.")
		assert.Equal(t, otherAppName, mergedObjectMeta.Labels["app"], "App label should be equal to app name.")
		assert.Equal(t, createObjectName(otherAppName, environment), mergedObjectMeta.Name, "Resource name should equal app name.")
		assert.Equal(t, otherTeamName, mergedObjectMeta.Namespace, "Resource environment should equal environment.")
		assert.Equal(t, "asd", mergedObjectMeta.ResourceVersion, "Resource version should be preserved when merging")
	})
}

func TestTeamNamespaceMultipleDeploys(t *testing.T) {
	naisResources := []NaisResource{
		{
			1,
			"resourceName",
			"resourceType",
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{"resourceKey": "resource1Value"},
			nil,
			map[string]string{"secretKey": "secretValue"},
			nil,
			nil,
		},
	}
	manifest := NaisManifest{
		Team:  "team",
		Image: "image",
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
	}

	clientset := fake.NewSimpleClientset()

	t.Run("Test deploying same application to different environments", func(t *testing.T) {
		deploymentRequest1 := naisrequest.Deploy{
			Environment: "t0",
			Application: "application",
			Version:     "1",
		}

		response1, err1 := createOrUpdateK8sResources(deploymentRequest1, manifest, naisResources, "nais.unittest.no", false, clientset)

		deploymentRequest2 := naisrequest.Deploy{
			Environment: "t1",
			Application: "application",
			Version:     "1",
		}

		response2, err2 := createOrUpdateK8sResources(deploymentRequest2, manifest, naisResources, "nais.unittest.no", false, clientset)

		assert.NoError(t, err1)
		assert.Equal(t, response1.Autoscaler.ObjectMeta.Name, createObjectName("application", "t0"))
		assert.Equal(t, response1.Deployment.ObjectMeta.Name, createObjectName("application", "t0"))
		assert.Equal(t, response1.Ingress.ObjectMeta.Name, createObjectName("application", "t0"))
		assert.Equal(t, response1.Secret.ObjectMeta.Name, createObjectName("application", "t0"))
		assert.Equal(t, response1.Service.ObjectMeta.Name, createObjectName("application", "t0"))
		assert.Equal(t, response1.ServiceAccount.ObjectMeta.Name, createObjectName("application", "t0"))

		assert.Equal(t, response1.Autoscaler.ObjectMeta.Labels["environment"], "t0")
		assert.Equal(t, response1.Deployment.ObjectMeta.Labels["environment"], "t0")
		assert.Equal(t, response1.Ingress.ObjectMeta.Labels["environment"], "t0")
		assert.Equal(t, response1.Secret.ObjectMeta.Labels["environment"], "t0")
		assert.Equal(t, response1.Service.ObjectMeta.Labels["environment"], "t0")
		assert.Equal(t, response1.ServiceAccount.ObjectMeta.Labels["environment"], "t0")

		assert.Equal(t, response1.Autoscaler.ObjectMeta.Namespace, "team")
		assert.Equal(t, response1.Deployment.ObjectMeta.Namespace, "team")
		assert.Equal(t, response1.Ingress.ObjectMeta.Namespace, "team")
		assert.Equal(t, response1.Secret.ObjectMeta.Namespace, "team")
		assert.Equal(t, response1.Service.ObjectMeta.Namespace, "team")
		assert.Equal(t, response1.ServiceAccount.ObjectMeta.Namespace, "team")

		assert.NoError(t, err2)
		assert.Equal(t, response2.Autoscaler.ObjectMeta.Name, createObjectName("application", "t1"))
		assert.Equal(t, response2.Deployment.ObjectMeta.Name, createObjectName("application", "t1"))
		assert.Equal(t, response2.Ingress.ObjectMeta.Name, createObjectName("application", "t1"))
		assert.Equal(t, response2.Secret.ObjectMeta.Name, createObjectName("application", "t1"))
		assert.Equal(t, response2.Service.ObjectMeta.Name, createObjectName("application", "t1"))
		assert.Equal(t, response2.ServiceAccount.ObjectMeta.Name, createObjectName("application", "t1"))

		assert.Equal(t, response2.Autoscaler.ObjectMeta.Labels["environment"], "t1")
		assert.Equal(t, response2.Deployment.ObjectMeta.Labels["environment"], "t1")
		assert.Equal(t, response2.Ingress.ObjectMeta.Labels["environment"], "t1")
		assert.Equal(t, response2.Secret.ObjectMeta.Labels["environment"], "t1")
		assert.Equal(t, response2.Service.ObjectMeta.Labels["environment"], "t1")
		assert.Equal(t, response2.ServiceAccount.ObjectMeta.Labels["environment"], "t1")

		assert.Equal(t, response2.Autoscaler.ObjectMeta.Namespace, "team")
		assert.Equal(t, response2.Deployment.ObjectMeta.Namespace, "team")
		assert.Equal(t, response2.Ingress.ObjectMeta.Namespace, "team")
		assert.Equal(t, response2.Secret.ObjectMeta.Namespace, "team")
		assert.Equal(t, response2.Service.ObjectMeta.Namespace, "team")
		assert.Equal(t, response2.ServiceAccount.ObjectMeta.Namespace, "team")
	})
}

func createSecretRef(appName string, resKey string, resName string) *k8score.EnvVarSource {
	return &k8score.EnvVarSource{
		SecretKeyRef: &k8score.SecretKeySelector{
			LocalObjectReference: k8score.LocalObjectReference{
				Name: appName,
			},
			Key: resName + "_" + resKey,
		},
	}
}

func getSidecarContainer(containers []k8score.Container, sidecarName string) *k8score.Container {
	for _, v := range containers {
		if v.Name == sidecarName {
			return &v
		}
	}

	return nil
}
