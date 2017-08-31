package api

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
	"testing"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	appName         = "appname"
	namespace       = "namespace"
	image           = "docker.hub/app"
	port            = 6900
	portName        = "http"
	resourceVersion = "12369"
	version         = "13"
	targetPort      = 234
	livenessPath    = "isAlive"
	readinessPath   = "isReady"
)

func TestService(t *testing.T) {
	t.Run("AValidDeploymentRequestAndAppConfigCreatesANewService", func(t *testing.T) {
		service := createServiceDef(targetPort, "", appName, namespace)

		assert.Equal(t, appName, service.ObjectMeta.Name)
		assert.Equal(t, int32(targetPort), service.Spec.Ports[0].TargetPort.IntVal)
		assert.Equal(t, map[string]string{"app": appName}, service.Spec.Selector)
	})
	t.Run("AValidServiceCanBeUpdated", func(t *testing.T) {
		service := createServiceDef(targetPort, resourceVersion, appName, namespace)

		assert.Equal(t, appName, service.ObjectMeta.Name)
		assert.Equal(t, resourceVersion, service.ObjectMeta.ResourceVersion)
		assert.Equal(t, int32(targetPort), service.Spec.Ports[0].TargetPort.IntVal)
		assert.Equal(t, map[string]string{"app": appName}, service.Spec.Selector)
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

	resource2Name := "r2"
	resource2Type := "db"
	resource2Key := "key2"
	resource2Value := "value2"
	secret2Key := "password"
	secret2Value := "anothersecret"

	invalidlyNamedResourceName := "dots.are.not.allowed"
	invalidlyNamedResourceType := "restservice"
	invalidlyNamedResourceKey := "key"
	invalidlyNamedResourceValue := "value"
	invalidlyNamedResourceSecretKey := "secretkey"
	invalidlyNamedResourceSecretValue := "secretvalue"

	naisResources := []NaisResource{
		{
			resource1Name,
			resource1Type,
			map[string]string{resource1Key: resource1Value},
			map[string]string{secret1Key: secret1Value},
		},
		{
			resource2Name,
			resource2Type,
			map[string]string{resource2Key: resource2Value},
			map[string]string{secret2Key: secret2Value},
		},
		{
			invalidlyNamedResourceName,
			invalidlyNamedResourceType,
			map[string]string{invalidlyNamedResourceKey: invalidlyNamedResourceValue},
			map[string]string{invalidlyNamedResourceSecretKey: invalidlyNamedResourceSecretValue},
		},
	}

	t.Run("AValidDeploymentRequestAndAppConfigCreatesANewDeployment", func(t *testing.T) {

		deployment := createDeploymentDef(naisResources, image, version, port, livenessPath, readinessPath, "", appName, namespace)

		assert.Equal(t, appName, deployment.Name)
		assert.Equal(t, "", deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, appName, deployment.Spec.Template.Name)

		containers := deployment.Spec.Template.Spec.Containers
		container := containers[0]
		assert.Equal(t, appName, container.Name)
		assert.Equal(t, image+":"+version, container.Image)
		assert.Equal(t, int32(port), container.Ports[0].ContainerPort)
		assert.Equal(t, livenessPath, container.LivenessProbe.HTTPGet.Path)
		assert.Equal(t, readinessPath, container.ReadinessProbe.HTTPGet.Path)

		env := container.Env
		assert.Equal(t, 7, len(env))
		assert.Equal(t, version, env[0].Value)
		assert.Equal(t, resource1Name+"_"+resource1Key, env[1].Name)
		assert.Equal(t, "value1", env[1].Value)
		assert.Equal(t, resource1Name+"_"+secret1Key, env[2].Name)
		assert.Equal(t, createSecretRef(appName, secret1Key, resource1Name), env[2].ValueFrom)
		assert.Equal(t, resource2Name+"_"+resource2Key, env[3].Name)
		assert.Equal(t, "value2", env[3].Value)
		assert.Equal(t, resource2Name+"_"+secret2Key, env[4].Name)
		assert.Equal(t, createSecretRef(appName, secret2Key, resource2Name), env[4].ValueFrom)
		assert.Equal(t, "dots_are_not_allowed_key", env[5].Name)
		assert.Equal(t, "dots_are_not_allowed_secretkey", env[6].Name)

	})

	t.Run("AValidDeploymentCanBeUpdated", func(t *testing.T) {
		updatedDeployment := createDeploymentDef(naisResources, image, newVersion, port, livenessPath, readinessPath, resourceVersion, appName, namespace)

		assert.Equal(t, appName, updatedDeployment.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].Name)
		assert.Equal(t, image+":"+newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, int32(port), updatedDeployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Env[0].Value)
	})
}

func TestIngress(t *testing.T) {
	t.Run("AValidDeploymentRequestAndAppConfigCreatesANewIngress", func(t *testing.T) {
		ingress := createIngressDef("nais.example.com", "", appName, namespace)

		assert.Equal(t, appName, ingress.ObjectMeta.Name)
		assert.Equal(t, appName+".nais.example.com", ingress.Spec.Rules[0].Host)
		assert.Equal(t, appName, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
	})

	t.Run("AValidIngressCanBeUpdated", func(t *testing.T) {
		updatedIngress := createIngressDef("subdomain", resourceVersion, appName, namespace)

		assert.Equal(t, appName, updatedIngress.ObjectMeta.Name)
		assert.Equal(t, resourceVersion, updatedIngress.ObjectMeta.ResourceVersion)
		assert.Equal(t, appName+".subdomain", updatedIngress.Spec.Rules[0].Host)
		assert.Equal(t, appName, updatedIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), updatedIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
	})
}

func TestSecret(t *testing.T) {
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

	t.Run("ADeploymentRequestContainingSecretsCreatesANewSecret", func(t *testing.T) {
		naisResources := []NaisResource{
			{resource1Name, resource1Type, map[string]string{resource1Key: resource1Value}, map[string]string{secret1Key: secret1Value}},
			{resource2Name, resource2Type, map[string]string{resource2Key: resource2Value}, map[string]string{secret2Key: secret2Value}}}

		secret := createSecretDef(naisResources, resourceVersion, appName, namespace)

		assert.Equal(t, appName+"-secrets", secret.ObjectMeta.Name)
		assert.Equal(t, 2, len(secret.Data))
		assert.Equal(t, []byte(secret1Value), secret.Data[resource1Name+"_"+secret1Key])
		assert.Equal(t, []byte(secret2Value), secret.Data[resource2Name+"_"+secret2Key])
		assert.Equal(t, resourceVersion, secret.ObjectMeta.ResourceVersion)
	})
}

func TestAutoscaler(t *testing.T) {
	autoscaler := createAutoscalerDef(10, 20, 30, "", appName, namespace)

	t.Run("CreatesValidAutoscaler", func(t *testing.T) {
		assert.Equal(t, *autoscaler.Spec.MinReplicas, int32(10))
		assert.Equal(t, autoscaler.Spec.MaxReplicas, int32(20))
		assert.Equal(t, *autoscaler.Spec.TargetCPUUtilizationPercentage, int32(30))
	})

	t.Run("AutoscalerUpdateWorks", func(t *testing.T) {
		const resourceVersion = "resourceId"
		updatedAutoscaler := createAutoscalerDef(100, 200, 300, resourceVersion, appName, namespace)
		assert.Equal(t, *updatedAutoscaler.Spec.MinReplicas, int32(100))
		assert.Equal(t, updatedAutoscaler.Spec.MaxReplicas, int32(200))
		assert.Equal(t, *updatedAutoscaler.Spec.TargetCPUUtilizationPercentage, int32(300))
		assert.Equal(t, updatedAutoscaler.ObjectMeta.ResourceVersion, resourceVersion)
	})
}

func TestCreateOrUpdateAutoscaler(t *testing.T) {
	const resourceId = "id"
	autoscaler := createAutoscalerDef(1, 2, 3, resourceId, appName, namespace)
	clientset := fake.NewSimpleClientset(autoscaler)

	t.Run("nonexistant autoscaler yields empty string and no error", func(t *testing.T) {
		id, err := getExistingAutoscalerId("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", id)
	})

	t.Run("existing autoscaler yields id and no error", func(t *testing.T) {
		id, err := getExistingAutoscalerId(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceId, id)
	})

	t.Run("when no autoscaler exists, a new one is created", func(t *testing.T) {
		autoscaler, err := createOrUpdateAutoscaler(NaisDeploymentRequest{Namespace: "othernamespace", Application: "otherapp"}, NaisAppConfig{Replicas: Replicas{Max: 1, Min: 2, CpuThresholdPercentage: 69}}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, int32(1), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, "othernamespace", autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, "otherapp", autoscaler.ObjectMeta.Name)
	})

	t.Run("when autoscaler exists, resource id is the same as before", func(t *testing.T) {
		autoscaler, err := createOrUpdateAutoscaler(NaisDeploymentRequest{Namespace: namespace, Application: appName}, NaisAppConfig{}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceId, autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, namespace, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, appName, autoscaler.ObjectMeta.Name)
	})
}


func createSecretRef(appName string, resKey string, resName string) *v1.EnvVarSource {
	return &v1.EnvVarSource{
		SecretKeyRef: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: appName + "-secrets",
			},
			Key: resName + "_" + resKey,
		},
	}
}