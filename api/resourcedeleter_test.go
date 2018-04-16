package api

import (
	"testing"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sapps "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/fake"
	"github.com/stretchr/testify/assert"
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
			Scope{"u", "u1", ZONE_FSS},
			map[string]string{resourceKey: resourceValue},
			map[string]string{},
			map[string]string{secretKey: secretValue},
			nil,
			nil,
		},
	}

	naisDeploymentRequest := NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}
	deploymentDef, _ := createDeploymentDef(naisResources, newDefaultManifest(), naisDeploymentRequest, nil, false)
	secretDef := createSecretDef(naisResources, nil, appName, namespace, teamName)
	secretDef.ObjectMeta.ResourceVersion = resourceVersion
	configMapDef := createConfigMapDef(AlertsConfigMapName, AlertsConfigMapNamespace, teamName)
	configMapDef.ObjectMeta.ResourceVersion = resourceVersion
	replicaSetDef := &k8sapps.ReplicaSet{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "v1",
		},
		ObjectMeta: createObjectMeta(appName, namespace, teamName),
	}

	clientset := fake.NewSimpleClientset(serviceDef, deploymentDef, secretDef, configMapDef, replicaSetDef)
	createService(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, teamName, clientset)

	t.Run("Deleting non-existing app should return no error and not nil result", func(t *testing.T) {
		res, err := deleteK8sResouces("nonexisting", appName, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Deleting existing app should return no error and nil result", func(t *testing.T) {
		res, err := deleteK8sResouces(namespace, appName, clientset)
		assert.NoError(t, err)
		assert.Nil(t, res)
	})

	t.Run("Deployment should be deleted", func(t *testing.T) {
		nilValue, err := getExistingDeployment(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("ReplicaSet should be deleted", func(t *testing.T) {
		replicasets, err := clientset.AppsV1().ReplicaSets(namespace).List(k8smeta.ListOptions{LabelSelector:"app="+appName})
		assert.NoError(t, err)
		assert.Empty(t, replicasets.Items)
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
		res, err := deleteAutoscaler(namespace, "nonexisting", clientset)
		assert.NoError(t, err)
		assert.Empty(t, res)
		autoscaler, err = getExistingAutoscaler(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, autoscaler)
	})

	t.Run("no error when deleting existant autoscaler", func(t *testing.T) {
		res, err := deleteAutoscaler(namespace, appName, clientset)
		assert.Empty(t, res)
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
		res, err := deleteIngress(namespace,"nonexisting", clientset)
		assert.NoError(t, err)
		assert.Empty(t, res)
		ingress, err := getExistingIngress(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, ingress)
	})

	t.Run("No error when deleting existant ingress", func(t *testing.T) {
		res, err := deleteIngress(namespace, appName, clientset)
		assert.NoError(t, err)
		assert.Empty(t, res)
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
		res, err := deleteConfigMapRules(appName, "nonexisting", clientset)
		assert.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("No error when deleting alerts configmap for existing configmap", func(t *testing.T) {
		res, err := deleteConfigMapRules(namespace, appName, clientset)
		assert.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("No alert for appName existant after deletion", func(t *testing.T) {
		configmap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, configmap)
		alert, _ := configmap.Data[appName + namespace + ".yml" ]
		assert.Empty(t, alert)
	})
}
