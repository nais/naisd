package api

import (
	"github.com/nais/naisd/api/constant"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestDeleteK8sResouces(t *testing.T) {
	resourceName := "r1"
	resourceType := "db"
	resourceKey := "key"
	resourceValue := "value"
	secretKey := "password"
	secretValue := "secret"

	serviceDef := createServiceDef(appName, namespace, teamName)
	naisResources := []NaisResource{
		{
			1,
			resourceName,
			resourceType,
			Scope{"u", "u1", constant.ZONE_FSS},
			map[string]string{resourceKey: resourceValue},
			map[string]string{},
			map[string]string{secretKey: secretValue},
			nil,
			nil,
		},
	}

	naisDeploymentRequest := naisrequest.Deploy{Namespace: namespace, Application: appName, Version: version}
	deploymentDef, _ := createDeploymentDef(naisResources, newDefaultManifest(), naisDeploymentRequest, nil, false)
	secretDef := createSecretDef(naisResources, nil, appName, namespace, teamName)
	secretDef.ObjectMeta.ResourceVersion = resourceVersion
	configMapDef := createConfigMapDef(AlertsConfigMapName, AlertsConfigMapNamespace, teamName)
	configMapDef.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(serviceDef, deploymentDef, secretDef, configMapDef)
	createService(naisrequest.Deploy{Namespace: namespace, Application: appName, Version: version}, teamName, clientset)

	t.Run("Deleting non-existing app should return no error", func(t *testing.T) {
		_, err := deleteK8sResouces("nonexisting", appName, clientset)
		assert.NoError(t, err)
	})

	t.Run("Deleting existing app should return no error", func(t *testing.T) {
		_, err := deleteK8sResouces(namespace, appName, clientset)
		assert.NoError(t, err)
	})

	t.Run("Deployment should be deleted", func(t *testing.T) {
		nilValue, err := getExistingDeployment(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Service should be deleted", func(t *testing.T) {
		nilValue, err := getExistingService(namespace, appName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Secret should be deleted", func(t *testing.T) {
		nilValue, err := getExistingSecret(AlertsConfigMapNamespace, appName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})
}

func TestDeleteAutoscaler(t *testing.T) {
	autoscaler := createOrUpdateAutoscalerDef(1, 2, 3, nil, appName, namespace, teamName)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler)

	t.Run("no error when autoscaler not existant", func(t *testing.T) {
		_, err := deleteAutoscaler(namespace, "nonexisting", clientset)
		assert.NoError(t, err)
		autoscaler, err = getExistingAutoscaler(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, autoscaler)
	})

	t.Run("no error when deleting existant autoscaler", func(t *testing.T) {
		_, err := deleteAutoscaler(namespace, appName, clientset)
		assert.NoError(t, err)
	})

	t.Run("no autoscaler for app in cluster after deletion", func(t *testing.T) {
		autoscaler, err := getExistingAutoscaler(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, autoscaler)
	})
}

func TestDeleteIngress(t *testing.T) {
	ingress := createIngressDef(appName, namespace, teamName)
	ingress.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(ingress)

	t.Run("No error when ingress not present", func(t *testing.T) {
		_, err := deleteIngress(namespace, "nonexisting", clientset)
		assert.NoError(t, err)
		ingress, err := getExistingIngress(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, ingress)
	})

	t.Run("No error when deleting existant ingress", func(t *testing.T) {
		_, err := deleteIngress(namespace, appName, clientset)
		assert.NoError(t, err)
		ingress, err := getExistingIngress(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, ingress)
	})
}

func TestDeleteConfigMapRules(t *testing.T) {
	configmap := createConfigMapDef(AlertsConfigMapName, AlertsConfigMapNamespace, teamName)
	configmap.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(configmap)

	t.Run("No error when deleting nonexistant app from alerts configmap", func(t *testing.T) {
		_, err := deleteConfigMapRules(appName, "nonexisting", clientset)
		assert.NoError(t, err)
	})

	t.Run("No error when deleting alerts configmap for existing configmap", func(t *testing.T) {
		_, err := deleteConfigMapRules(namespace, appName, clientset)
		assert.NoError(t, err)
	})

	t.Run("No alert for appName existant after deletion", func(t *testing.T) {
		configmap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, configmap)
		alert, _ := configmap.Data[appName+namespace+".yml"]
		assert.Empty(t, alert)
	})
}
