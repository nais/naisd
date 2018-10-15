package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/pkg/test"

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
		Redis:          Redis{Enabled: false},
		Team:           teamName,
	}

	return manifest

}

func TestService(t *testing.T) {
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}
	otherSpec := app.Spec{Application: otherAppName, Environment: environment, Team: otherTeamName, ApplicationNamespaced: true}
	service := createServiceDef(spec)
	fillServiceSpec(spec, &service.Spec)
	service.Spec.ClusterIP = clusterIP
	clientset := fake.NewSimpleClientset(service)

	t.Run("Fetching nonexistant service yields nil and no error", func(t *testing.T) {
		nonExistingSpec := app.Spec{Application: "nonexisting", Environment: environment, Team: teamName, ApplicationNamespaced: true}
		nonExistantService, err := getExistingService(nonExistingSpec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistantService)
	})

	t.Run("Fetching an existing service yields service and no error", func(t *testing.T) {
		existingService, err := getExistingService(spec, clientset)
		assert.NoError(t, err)
		assert.Equal(t, service, existingService)
	})

	t.Run("Creating service with ApplicationNamespaced has correct labels", func(t *testing.T) {
		service := createServiceDef(spec)
		fillServiceSpec(spec, &service.Spec)

		assert.Equal(t, service.Spec.Selector["app"], appName)
		assert.Equal(t, service.Spec.Selector["environment"], environment)
		assert.Empty(t, service.Spec.Selector["teamm"])
	})

	t.Run("Creating service without ApplicationNamespaced has correct labels", func(t *testing.T) {
		oldSpec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: false}

		service := createServiceDef(oldSpec)
		fillServiceSpec(oldSpec, &service.Spec)

		assert.Equal(t, service.Spec.Selector["app"], appName)
		assert.Empty(t, service.Spec.Selector["team"])
		assert.Empty(t, service.Spec.Selector["environment"])
	})

	t.Run("when no service exists, a new one is created", func(t *testing.T) {
		service, err := createOrUpdateService(otherSpec, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherSpec.ResourceName(), service.Name)
		assert.Equal(t, otherTeamName, service.Labels["team"])
		assert.Equal(t, DefaultPortName, service.Spec.Ports[0].TargetPort.StrVal)
		assert.Equal(t, "http", service.Spec.Ports[0].Name)
		assert.Equal(t, map[string]string{"app": otherAppName, "environment": environment}, service.Spec.Selector)
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

	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}
	otherSpec := app.Spec{Application: otherAppName, Environment: environment, Team: otherTeamName, ApplicationNamespaced: true}
	deployment, err := createDeploymentDef(spec, naisResources, newDefaultManifest(), naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, nil, false)

	assert.Nil(t, err)

	deployment.ObjectMeta.ResourceVersion = resourceVersion

	clientset := fake.NewSimpleClientset(deployment)

	t.Run("Nonexistant deployment yields empty string and no error", func(t *testing.T) {
		nonExistingSpec := app.Spec{Application: "nonexisting", Environment: environment, Team: teamName, ApplicationNamespaced: true}
		nilValue, err := getExistingDeployment(nonExistingSpec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing deployment yields def and no error", func(t *testing.T) {
		id, err := getExistingDeployment(spec, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, id.ObjectMeta.ResourceVersion)
	})

	t.Run("when no deployment exists, it's created", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.Istio.Enabled = true
		deployment, err := createOrUpdateDeployment(otherSpec, naisrequest.Deploy{Environment: environment, Application: otherAppName, Version: version, FasitEnvironment: fasitEnvironment}, manifest, naisResources, true, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherSpec.Environment, deployment.Name)
		assert.Equal(t, "", deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, otherSpec.Environment, deployment.Spec.Template.Name)

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
		assert.Equal(t, createSecretRef(otherSpec.ResourceName(), secret2Key, resource2Name), env[7].ValueFrom)

		assert.Equal(t, "KEY1", env[8].Name)
		assert.Equal(t, "KEY2_PROPERTY", env[9].Name)
		assert.Equal(t, "DOTS_ARE_NOT_ALLOWED_KEY", env[10].Name)
		assert.Equal(t, "DOTS_ARE_NOT_ALLOWED_SECRETKEY", env[11].Name)
		assert.Equal(t, "COLON_ARE_NOT_ALLOWED_KEY", env[12].Name)
		assert.Equal(t, "COLON_ARE_NOT_ALLOWED_SECRETKEY", env[13].Name)
		assert.False(t, manifest.LeaderElection, "LeaderElection should default to false")
		assert.False(t, manifest.Redis.Enabled, "Redis should default to false")
	})

	t.Run("when fasit is skipped, FAIST_ENVIRONMENT_NAME is not set", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.Istio.Enabled = true
		deployment, _ := createOrUpdateDeployment(spec, naisrequest.Deploy{
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
		assert.False(t, manifest.Redis.Enabled, "Redis should default to false")
	})

	t.Run("when a deployment exists, its updated", func(t *testing.T) {
		updatedDeployment, err := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: newVersion}, newDefaultManifest(), naisResources, false, clientset)
		assert.NoError(t, err)

		assert.Equal(t, resourceVersion, deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, spec.ResourceName(), updatedDeployment.Name)
		assert.Equal(t, spec.ResourceName(), updatedDeployment.Spec.Template.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].Name)
		assert.Equal(t, image+":"+newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, int32(port), updatedDeployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Env[1].Value)
	})

	t.Run("when leaderElection is true, extra container exists", func(t *testing.T) {
		manifest := newDefaultManifest()
		manifest.LeaderElection = true
		deployment, err := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
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

		updatedDeployment, err := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
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

		updateDeployment, err := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
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

		d, err := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, manifest, naisResources, false, clientset)
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

		updatedDeployment, err := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), updatedResource, false, clientset)
		assert.NoError(t, err)

		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Volumes))
		assert.Equal(t, spec.ResourceName(), updatedDeployment.Spec.Template.Spec.Volumes[0].Name)
		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Volumes[0].Secret.Items))
		assert.Equal(t, resource1Name+"_"+updatedCertKey, updatedDeployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Key)

		assert.Equal(t, 1, len(updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts))
		assert.Equal(t, "/var/run/secrets/naisd.io/", updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
		assert.Equal(t, spec.ResourceName(), updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
	})

	t.Run("File secrets are mounted correctly for a new deployment", func(t *testing.T) {
		deployment, _ := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), naisCertResources, false, clientset)

		assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Volumes))
		assert.Equal(t, spec.ResourceName(), deployment.Spec.Template.Spec.Volumes[0].Name)
		assert.Equal(t, 2, len(deployment.Spec.Template.Spec.Volumes[0].Secret.Items))
		assert.Equal(t, resource1Name+"_"+cert1Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Key)
		assert.Equal(t, resource1Name+"_"+cert1Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[0].Path)
		assert.Equal(t, resource2Name+"_"+cert2Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[1].Key)
		assert.Equal(t, resource2Name+"_"+cert2Key, deployment.Spec.Template.Spec.Volumes[0].Secret.Items[1].Path)

		assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts))
		assert.Equal(t, "/var/run/secrets/naisd.io/", deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
		assert.Equal(t, spec.ResourceName(), deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)

	})

	t.Run("Env variable is created for file secrets ", func(t *testing.T) {
		deployment, _ := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), naisCertResources, false, clientset)

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

		deployment, err := createOrUpdateDeployment(spec, naisrequest.Deploy{Environment: environment, Application: appName, Version: version}, newDefaultManifest(), resources, false, clientset)

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
			Environment: spec.Environment,
			Application: spec.Application,
			Version:     "1",
		}

		_, err := createOrUpdateDeployment(spec, deploymentRequest, newDefaultManifest(), []NaisResource{resource1, resource2}, false, clientset)

		assert.NotNil(t, err)
		assert.Equal(t, "unable to create deployment: found duplicate environment variable SRVAPP_PASSWORD when adding password for srvapp (certificate)"+
			" Change the Fasit alias or use propertyMap to create unique variable names", err.Error())
	})
	t.Run("Injects envoy sidecar based on settings", func(t *testing.T) {
		istioDisabledManifest := NaisManifest{Istio: IstioConfig{Enabled: false}}
		istioEnabledManifest := NaisManifest{Istio: IstioConfig{Enabled: true}}

		assert.Equal(t, createPodObjectMetaWithAnnotations(spec, istioDisabledManifest, false).Annotations["sidecar.istio.io/inject"], "")
		assert.Equal(t, createPodObjectMetaWithAnnotations(spec, istioEnabledManifest, false).Annotations["sidecar.istio.io/inject"], "")
		assert.Equal(t, createPodObjectMetaWithAnnotations(spec, istioDisabledManifest, true).Annotations["sidecar.istio.io/inject"], "")
		assert.Equal(t, createPodObjectMetaWithAnnotations(spec, istioEnabledManifest, true).Annotations["sidecar.istio.io/inject"], "true")
	})

}

func TestIngress(t *testing.T) {
	appName := "appname"
	environment := "app"
	subDomain := "example.no"
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}
	otherSpec := app.Spec{Application: otherAppName, Environment: environment, Team: otherTeamName, ApplicationNamespaced: true}
	nonExistingSpec := app.Spec{Application: "nonexisting", Environment: environment, Team: teamName, ApplicationNamespaced: true}
	naisManifest := NaisManifest{
		Healthcheck: Healthcheck{
			Liveness: Probe{
				Path: "/internal/liveness",
			},
		},
	}

	ingress := createIngressDef(spec)
	ingress.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(ingress)

	t.Run("Nonexistant ingress yields nil and no error", func(t *testing.T) {
		ingress, err := getExistingIngress(nonExistingSpec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, ingress)
	})

	t.Run("Existing ingress yields def and no error", func(t *testing.T) {
		existingIngress, err := getExistingIngress(spec, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingIngress.ObjectMeta.ResourceVersion)
	})

	t.Run("when no ingress exists, a default ingress is created", func(t *testing.T) {
		ingress, err := createOrUpdateIngress(otherSpec, naisManifest, naisrequest.Deploy{Environment: environment, Application: otherAppName, ApplicationNamespaced: true}, subDomain, []NaisResource{}, clientset)

		assert.NoError(t, err)
		assert.Equal(t, spec.ResourceName(), ingress.Name)
		assert.Equal(t, otherTeamName, ingress.Labels["team"])
		assert.Equal(t, 1, len(ingress.Spec.Rules))
		assert.Equal(t, otherAppName+"."+subDomain, ingress.Spec.Rules[0].Host)
		assert.Equal(t, spec.ResourceName(), ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
		assert.Equal(t, "true", ingress.Annotations["prometheus.io/scrape"])
		assert.Equal(t, naisManifest.Healthcheck.Liveness.Path, ingress.Annotations["prometheus.io/path"])
	})

	t.Run("when ingress is created in non-default environment, hostname is postfixed with environment", func(t *testing.T) {
		otherEnvironment := "nondefault"
		otherEnvSpec := app.Spec{Application: otherAppName, Environment: otherEnvironment, Team: otherTeamName, ApplicationNamespaced: true}

		ingress, err := createOrUpdateIngress(otherEnvSpec, naisManifest, naisrequest.Deploy{Environment: otherEnvironment, Namespace: otherEnvironment, Application: otherAppName}, subDomain, []NaisResource{}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, otherAppName+"-"+otherEnvironment+"."+subDomain, ingress.Spec.Rules[0].Host)
	})

	t.Run("Nais ingress resources are added", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(ingress) // Avoid interfering with other tests in suite.
		naisResources := []NaisResource{
			{
				resourceType: "LoadBalancerConfig",
				ingresses: []FasitIngress{
					{Host: "app.adeo.no", Path: "context"},
				},
			},
			{
				resourceType: "LoadBalancerConfig",
				ingresses: []FasitIngress{
					{Host: "app2.adeo.no", Path: "context2"},
				},
			},
		}
		ingress, err := createOrUpdateIngress(otherSpec, naisManifest, naisrequest.Deploy{Environment: environment, Application: otherAppName}, subDomain, naisResources, clientset)

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
		clientset := fake.NewSimpleClientset(ingress) // Avoid interfering with other tests in suite.
		var naisResources []NaisResource

		ingress, err := createOrUpdateIngress(spec, naisManifest, naisrequest.Deploy{Environment: environment, Application: spec.Application, Zone: constant.ZONE_SBS, FasitEnvironment: spec.Environment, ApplicationNamespaced: true}, subDomain, naisResources, clientset)
		rules := ingress.Spec.Rules

		assert.NoError(t, err)
		assert.Equal(t, 2, len(rules))

		firstRule := rules[0]
		assert.Equal(t, "appname.example.no", firstRule.Host)
		assert.Equal(t, 1, len(firstRule.HTTP.Paths))
		assert.Equal(t, "/", firstRule.HTTP.Paths[0].Path)

		secondRule := rules[1]
		assert.Equal(t, fmt.Sprintf("tjenester-%s.nav.no", spec.Environment), secondRule.Host)
		assert.Equal(t, 1, len(secondRule.HTTP.Paths))
		assert.Equal(t, fmt.Sprintf("/%s", spec.Application), secondRule.HTTP.Paths[0].Path)
	})

	t.Run("when no deploying to application namespace, environment 'app' should give url's without environment postfixed.", func(t *testing.T) {
		tests := []struct {
			testDescription       string
			application           string
			environment           string
			namespace             string
			applicationNamespaced bool
			expectedHostname      string
		}{
			{
				testDescription:       "when environment is 'app', namespace is 'default' and applicationNamespaced is 'false', don't postfix environment",
				application:           "application",
				environment:           "app",
				namespace:             "default",
				applicationNamespaced: false,
				expectedHostname:      "application",
			},
			{
				testDescription:       "when environment is 'app', namespace is 'default' and applicationNamespaced is 'true', don't postfix environment",
				application:           "application",
				environment:           "app",
				namespace:             "default",
				applicationNamespaced: true,
				expectedHostname:      "application",
			},
			{
				testDescription:       "when environment is 't1', namespace is 'default' and applicationNamespaced is 'false', don't postfix environment",
				application:           "application",
				environment:           "t1",
				namespace:             "default",
				applicationNamespaced: false,
				expectedHostname:      "application",
			},
			{
				testDescription:       "when environment is 'app', namespace is 't1' and applicationNamespaced is 'true', don't postfix environment",
				application:           "application",
				environment:           "app",
				namespace:             "t1",
				applicationNamespaced: true,
				expectedHostname:      "application",
			},
			{
				testDescription:       "when environment is 't1', namespace is 'default' and applicationNamespaced is 'true', postfix environment",
				application:           "application",
				environment:           "t1",
				namespace:             "default",
				applicationNamespaced: true,
				expectedHostname:      "application-t1",
			},
			{
				testDescription:       "when environment is 'app', namespace is 't1' and applicationNamespaced is 'false', postfix environment",
				application:           "application",
				environment:           "app",
				namespace:             "t1",
				applicationNamespaced: false,
				expectedHostname:      "application-t1",
			},
		}

		for _, test := range tests {
			subDomain = "subdomain.no"
			actualHostname := createIngressHostname(test.application, test.environment, test.namespace, subDomain, test.applicationNamespaced)
			if test.expectedHostname+"."+subDomain != actualHostname {
				t.Errorf("Failed test: %s\nExpected: %+v\nGot: %+v", test.testDescription, test.expectedHostname, actualHostname)
			}
		}
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

	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}
	nonExistingSpec := app.Spec{Application: "nonexisting", Environment: environment, Team: teamName, ApplicationNamespaced: true}
	otherSpec := app.Spec{Application: otherAppName, Environment: environment, Team: otherTeamName, ApplicationNamespaced: true}

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

	secret := createSecretDef(spec, naisResources, nil)
	secret.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(secret)

	t.Run("Nonexistant secret yields nil and no error", func(t *testing.T) {
		nilValue, err := getExistingSecret(nonExistingSpec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing secret yields def and no error", func(t *testing.T) {
		existingSecret, err := getExistingSecret(spec, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingSecret.ObjectMeta.ResourceVersion)
	})

	t.Run("when no secret exists, a new one is created", func(t *testing.T) {
		secret, err := createOrUpdateSecret(otherSpec, naisResources, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, otherSpec.ResourceName(), secret.ObjectMeta.Name)
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
		secret, err := createOrUpdateSecret(spec, []NaisResource{
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
		}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, spec.Namespace(), secret.Namespace)
		assert.Equal(t, spec.ResourceName(), secret.Name)
		assert.Equal(t, teamName, secret.ObjectMeta.Labels["team"])
		assert.Equal(t, environment, secret.ObjectMeta.Labels["environment"])
		assert.Equal(t, []byte(updatedSecretValue), secret.Data["r1_alias_password"])
		assert.Equal(t, updatedFileValue, secret.Data["r1_alias_filekey1"])
	})
}

func TestCreateOrUpdateAutoscaler(t *testing.T) {
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}
	nonExistingSpec := app.Spec{Application: "nonexisting", Environment: environment, Team: teamName, ApplicationNamespaced: true}
	otherSpec := app.Spec{Application: otherAppName, Environment: environment, Team: otherTeamName, ApplicationNamespaced: true}

	autoscaler := createOrUpdateAutoscalerDef(spec, 1, 2, 3, nil)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler)

	t.Run("nonexistant autoscaler yields empty string and no error", func(t *testing.T) {
		nonExistingAutoscaler, err := getExistingAutoscaler(nonExistingSpec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistingAutoscaler)
	})

	t.Run("existing autoscaler yields id and no error", func(t *testing.T) {
		existingAutoscaler, err := getExistingAutoscaler(spec, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingAutoscaler.ObjectMeta.ResourceVersion)
	})

	t.Run("when no autoscaler exists, a new one is created", func(t *testing.T) {
		autoscaler, err := createOrUpdateAutoscaler(otherSpec, NaisManifest{Replicas: Replicas{Max: 1, Min: 2, CpuThresholdPercentage: 69}, Team: otherTeamName}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", autoscaler.ResourceVersion)
		assert.Equal(t, int32(1), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, int32p(2), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32p(69), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, otherSpec.Namespace(), autoscaler.Namespace)
		assert.Equal(t, otherSpec.ResourceName(), autoscaler.Name)
		assert.Equal(t, otherTeamName, autoscaler.Labels["team"])
		assert.Equal(t, environment, autoscaler.Labels["environment"])
		assert.Equal(t, otherSpec.ResourceName(), autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})

	t.Run("when autoscaler exists, it's updated", func(t *testing.T) {
		cpuThreshold := 69
		minReplicas := 6
		maxReplicas := 9
		autoscaler, err := createOrUpdateAutoscaler(spec, NaisManifest{Replicas: Replicas{CpuThresholdPercentage: cpuThreshold, Min: minReplicas, Max: maxReplicas}, Team: teamName}, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, autoscaler)
		assert.Equal(t, resourceVersion, autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, appName, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, spec.ResourceName(), autoscaler.ObjectMeta.Name)
		assert.Equal(t, teamName, autoscaler.ObjectMeta.Labels["team"])
		assert.Equal(t, environment, autoscaler.ObjectMeta.Labels["environment"])
		assert.Equal(t, int32p(int32(cpuThreshold)), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, int32p(int32(minReplicas)), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32(maxReplicas), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, spec.ResourceName(), autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})
}

func TestDNS1123ValidResourceNames(t *testing.T) {
	spec := app.Spec{Application: appName, Environment: "key_underscore_Upper", Team: teamName, ApplicationNamespaced: true}

	naisResource := []NaisResource{
		{
			1,
			"name",
			"resourceType",
			Scope{"u", "u1", constant.ZONE_FSS},
			nil,
			nil,
			nil,
			map[string][]byte{"key": []byte("value")},
			nil,
		},
	}

	t.Run("Generate valid volume mount name", func(t *testing.T) {
		volumeMount := createCertificateVolumeMount(spec, naisResource)
		assert.Equal(t, "key-underscore-upper", volumeMount.Name)

	})

	t.Run("Generate valid volume name", func(t *testing.T) {
		volume := createCertificateVolume(spec, naisResource)
		assert.Equal(t, "key-underscore-upper", volume.Name)

	})

}

func TestCreateK8sResources(t *testing.T) {
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}

	deploymentRequest := naisrequest.Deploy{
		Application:           spec.Application,
		Version:               version,
		FasitEnvironment:      spec.Environment,
		ManifestUrl:           "http://repo.com/app",
		Zone:                  "zone",
		Environment:           spec.Environment,
		ApplicationNamespaced: true,
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

	service := createServiceDef(spec)
	fillServiceSpec(spec, &service.Spec)
	service.ResourceVersion = "abc"

	autoscaler := createOrUpdateAutoscalerDef(spec, 6, 9, 6, nil)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler, service)

	t.Run("creates all resources", func(t *testing.T) {
		deploymentResult, err := createOrUpdateK8sResources(spec, deploymentRequest, manifest, naisResources, "nais.example.yo", false, clientset)
		assert.NoError(t, err)

		assert.NotEmpty(t, deploymentResult.Secret)
		assert.NotEmpty(t, deploymentResult.Service)
		assert.NotEmpty(t, deploymentResult.Deployment)
		assert.NotEmpty(t, deploymentResult.Ingress)
		assert.NotEmpty(t, deploymentResult.Autoscaler)
		assert.NotEmpty(t, deploymentResult.ServiceAccount)

		assert.Equal(t, resourceVersion, deploymentResult.Autoscaler.ResourceVersion, "autoscaler should have same id as the preexisting")
		assert.Equal(t, "", deploymentResult.Secret.ResourceVersion, "secret should not have any id set")
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
		deploymentResult, err := createOrUpdateK8sResources(spec, deploymentRequest, manifest, naisResourcesNoSecret, "nais.example.yo", false, fake.NewSimpleClientset())
		assert.NoError(t, err)

		assert.Empty(t, deploymentResult.Secret)
		assert.NotEmpty(t, deploymentResult.Service)
	})

	t.Run("omits ingress creation when disabled", func(t *testing.T) {
		manifest.Ingress.Disabled = true

		deploymentResult, err := createOrUpdateK8sResources(spec, deploymentRequest, manifest, naisResourcesNoSecret, "nais.example.yo", false, fake.NewSimpleClientset())
		assert.NoError(t, err)

		assert.Empty(t, deploymentResult.Ingress)
	})

}

func TestCheckForDuplicates(t *testing.T) {
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}

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
			Application:           "myapp",
			Version:               "1",
			ApplicationNamespaced: true,
		}

		_, err := createEnvironmentVariables(spec, deploymentRequest, NaisManifest{}, []NaisResource{resource1, resource2})

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
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}
	vars := map[string]string{
		"NAIS_POD_HTTP_PROXY": "http://foo.bar:1234",
		"NAIS_POD_NO_PROXY":   "baz",
	}

	test.EnvWrapper(vars,
		func(t *testing.T) {
			t.Run("proxy settings should be injected in the pod if requested through manifest", func(t *testing.T) {
				deploymentRequest := naisrequest.Deploy{
					Application:           "myapp",
					Version:               "1",
					ApplicationNamespaced: true,
				}

				manifest := NaisManifest{
					Webproxy: true,
				}

				env, err := createEnvironmentVariables(spec, deploymentRequest, manifest, []NaisResource{})

				assert.Nil(t, err)
				assert.Contains(t, env, k8score.EnvVar{Name: "HTTP_PROXY", Value: "http://foo.bar:1234"})
				assert.Contains(t, env, k8score.EnvVar{Name: "HTTPS_PROXY", Value: "http://foo.bar:1234"})
				assert.Contains(t, env, k8score.EnvVar{Name: "NO_PROXY", Value: "baz"})
				assert.Contains(t, env, k8score.EnvVar{Name: "http_proxy", Value: "http://foo.bar:1234"})
				assert.Contains(t, env, k8score.EnvVar{Name: "https_proxy", Value: "http://foo.bar:1234"})
				assert.Contains(t, env, k8score.EnvVar{Name: "no_proxy", Value: "baz"})
				assert.Contains(t, env, k8score.EnvVar{Name: "JAVA_PROXY_OPTIONS", Value: "-Dhttp.proxyHost=foo.bar -Dhttps.proxyHost=foo.bar -Dhttp.proxyPort=1234 -Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts=baz"})
			})

			t.Run("proxy settings should not be injected in the pod unless requested through manifest", func(t *testing.T) {
				deploymentRequest := naisrequest.Deploy{
					Application:           "myapp",
					Version:               "1",
					ApplicationNamespaced: true,
				}

				manifest := NaisManifest{}

				env, err := createEnvironmentVariables(spec, deploymentRequest, manifest, []NaisResource{})

				assert.Nil(t, err)
				assert.NotContains(t, env, k8score.EnvVar{Name: "HTTP_PROXY", Value: "http://foo.bar:1234"})
				assert.NotContains(t, env, k8score.EnvVar{Name: "HTTPS_PROXY", Value: "http://foo.bar:1234"})
				assert.NotContains(t, env, k8score.EnvVar{Name: "NO_PROXY", Value: "baz"})
				assert.NotContains(t, env, k8score.EnvVar{Name: "http_proxy", Value: "http://foo.bar:1234"})
				assert.NotContains(t, env, k8score.EnvVar{Name: "https_proxy", Value: "http://foo.bar:1234"})
				assert.NotContains(t, env, k8score.EnvVar{Name: "no_proxy", Value: "baz"})
				assert.NotContains(t, env, k8score.EnvVar{Name: "JAVA_PROXY_OPTIONS", Value: "-Dhttp.proxyHost=foo.bar -Dhttps.proxyHost=foo.bar -Dhttp.proxyPort=1234 -Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts=baz"})
			})
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
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}

	t.Run("Test required metadata field values", func(t *testing.T) {
		objectMeta := generateObjectMeta(spec)

		assert.Equal(t, teamName, objectMeta.Labels["team"], "Team label should be equal to team name.")
		assert.Equal(t, appName, objectMeta.Labels["app"], "App label should be equal to app name.")
		assert.Equal(t, spec.ResourceName(), objectMeta.Name, "Resource name should equal app name.")
		assert.Equal(t, spec.Namespace(), objectMeta.Namespace, "Resource environment should equal environment.")
	})
}

func TestMergeObjectMeta(t *testing.T) {
	spec := app.Spec{Application: appName, Environment: environment, Team: teamName, ApplicationNamespaced: true}
	otherSpec := app.Spec{Application: otherAppName, Environment: environment, Team: otherTeamName, ApplicationNamespaced: true}

	t.Run("Test merging objectmeta", func(t *testing.T) {
		existingObjectMeta := generateObjectMeta(spec)
		existingObjectMeta.ResourceVersion = "asd"

		newObjectMeta := generateObjectMeta(otherSpec)

		mergedObjectMeta := mergeObjectMeta(existingObjectMeta, newObjectMeta)

		assert.Equal(t, otherTeamName, mergedObjectMeta.Labels["team"], "Team label should be equal to team name.")
		assert.Equal(t, otherAppName, mergedObjectMeta.Labels["app"], "App label should be equal to app name.")
		assert.Equal(t, otherSpec.ResourceName(), mergedObjectMeta.Name, "Resource name should equal app name.")
		assert.Equal(t, otherAppName, mergedObjectMeta.Namespace, "Resource environment should equal environment.")
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
		specT0 := app.Spec{Application: "application", Environment: "t0", Team: "team", ApplicationNamespaced: true}
		deploymentRequest1 := naisrequest.Deploy{
			Environment:           "t0",
			Application:           "application",
			Version:               "1",
			ApplicationNamespaced: true,
		}

		response1, err1 := createOrUpdateK8sResources(specT0, deploymentRequest1, manifest, naisResources, "nais.unittest.no", false, clientset)

		specT1 := app.Spec{Application: "application", Environment: "t1", Team: "team", ApplicationNamespaced: true}
		deploymentRequest2 := naisrequest.Deploy{
			Environment:           "t1",
			Application:           "application",
			Version:               "1",
			ApplicationNamespaced: true,
		}

		response2, err2 := createOrUpdateK8sResources(specT1, deploymentRequest2, manifest, naisResources, "nais.unittest.no", false, clientset)

		assert.NoError(t, err1)
		assert.Equal(t, response1.Autoscaler.Name, specT0.ResourceName())
		assert.Equal(t, response1.Deployment.Name, specT0.ResourceName())
		assert.Equal(t, response1.Ingress.Name, specT0.ResourceName())
		assert.Equal(t, response1.Secret.Name, specT0.ResourceName())
		assert.Equal(t, response1.Service.Name, specT0.ResourceName())
		assert.Equal(t, response1.ServiceAccount.Name, specT0.ResourceName())

		assert.Equal(t, response1.Autoscaler.Labels["environment"], "t0")
		assert.Equal(t, response1.Deployment.Labels["environment"], "t0")
		assert.Equal(t, response1.Ingress.Labels["environment"], "t0")
		assert.Equal(t, response1.Secret.Labels["environment"], "t0")
		assert.Equal(t, response1.Service.Labels["environment"], "t0")
		assert.Equal(t, response1.ServiceAccount.Labels["environment"], "t0")

		assert.Equal(t, response1.Autoscaler.Namespace, "application")
		assert.Equal(t, response1.Deployment.Namespace, "application")
		assert.Equal(t, response1.Ingress.Namespace, "application")
		assert.Equal(t, response1.Secret.Namespace, "application")
		assert.Equal(t, response1.Service.Namespace, "application")
		assert.Equal(t, response1.ServiceAccount.Namespace, "application")

		assert.NoError(t, err2)
		assert.Equal(t, response2.Autoscaler.Name, specT1.ResourceName())
		assert.Equal(t, response2.Deployment.Name, specT1.ResourceName())
		assert.Equal(t, response2.Ingress.Name, specT1.ResourceName())
		assert.Equal(t, response2.Secret.Name, specT1.ResourceName())
		assert.Equal(t, response2.Service.Name, specT1.ResourceName())
		assert.Equal(t, response2.ServiceAccount.Name, specT1.ResourceName())

		assert.Equal(t, response2.Autoscaler.Labels["environment"], "t1")
		assert.Equal(t, response2.Deployment.Labels["environment"], "t1")
		assert.Equal(t, response2.Ingress.Labels["environment"], "t1")
		assert.Equal(t, response2.Secret.Labels["environment"], "t1")
		assert.Equal(t, response2.Service.Labels["environment"], "t1")
		assert.Equal(t, response2.ServiceAccount.Labels["environment"], "t1")

		assert.Equal(t, response2.Autoscaler.Namespace, "application")
		assert.Equal(t, response2.Deployment.Namespace, "application")
		assert.Equal(t, response2.Ingress.Namespace, "application")
		assert.Equal(t, response2.Secret.Namespace, "application")
		assert.Equal(t, response2.Service.Namespace, "application")
		assert.Equal(t, response2.ServiceAccount.Namespace, "application")
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
