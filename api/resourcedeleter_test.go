package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/constant"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	k8score "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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

	spec := app.Spec{Application: appName, Namespace: namespace, Team: teamName}
	nonExistingSpec := app.Spec{Application: "nonexisting", Namespace: namespace, Team: teamName}

	serviceDef := createServiceDef(spec)
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
	deploymentDef, _ := createDeploymentDef(spec, naisResources, newDefaultManifest(), naisDeploymentRequest, nil, false)
	secretDef := createSecretDef(spec, naisResources, nil)
	secretDef.ObjectMeta.ResourceVersion = resourceVersion

	configMapDef := &k8score.ConfigMap{ObjectMeta: createObjectMeta(AlertsConfigMapName, AlertsConfigMapNamespace)}
	configMapDef.ObjectMeta.ResourceVersion = resourceVersion
	serviceAccountDef := createServiceAccountDef(spec)
	clientset := fake.NewSimpleClientset(serviceDef, deploymentDef, secretDef, configMapDef, serviceAccountDef)

	t.Run("Deleting non-existing app should return no error", func(t *testing.T) {
		_, err := deleteK8sResouces(nonExistingSpec, clientset)
		assert.NoError(t, err)
	})

	t.Run("Deleting existing app should delete all created resources", func(t *testing.T) {
		result, err := deleteK8sResouces(spec, clientset)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		deployment, err := getExistingAppDeployment(spec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, deployment)

		svc, err := getExistingAppService(spec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, svc)

		secret, err := getExistingSecret(spec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, secret)

		account, e := clientset.CoreV1().ServiceAccounts(spec.Namespace).Get(spec.ResourceName(), v1.GetOptions{})
		assert.Error(t, e)
		assert.True(t, errors.IsNotFound(e))
		assert.Nil(t, account)
	})
}

func TestDeleteAutoscaler(t *testing.T) {
	spec := app.Spec{Application: appName, Namespace: namespace, Team: teamName}
	nonExistingSpec := app.Spec{Application: "nonexisting", Namespace: namespace, Team: teamName}

	autoscaler := createOrUpdateAutoscalerDef(spec, 1, 2, 3, nil)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler)

	t.Run("no error when autoscaler not existant", func(t *testing.T) {
		_, err := deleteAutoscaler(nonExistingSpec, clientset)
		assert.NoError(t, err)
		autoscaler, err = getExistingAutoscaler(spec, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, autoscaler)
	})

	t.Run("no error when deleting existant autoscaler", func(t *testing.T) {
		_, err := deleteAutoscaler(spec, clientset)
		assert.NoError(t, err)
	})

	t.Run("no autoscaler for app in cluster after deletion", func(t *testing.T) {
		autoscaler, err := getExistingAutoscaler(spec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, autoscaler)
	})
}

func TestDeleteIngress(t *testing.T) {
	spec := app.Spec{Application: appName, Namespace: namespace, Team: teamName}
	nonExistingSpec := app.Spec{Application: "nonexisting", Namespace: namespace, Team: teamName}

	ingress := createIngressDef(spec)
	ingress.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(ingress)

	t.Run("No error when ingress not present", func(t *testing.T) {
		_, err := deleteIngress(nonExistingSpec, clientset)
		assert.NoError(t, err)
		ingress, err := getExistingIngress(spec, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, ingress)
	})

	t.Run("No error when deleting existant ingress", func(t *testing.T) {
		_, err := deleteIngress(spec, clientset)
		assert.NoError(t, err)
		ingress, err := getExistingIngress(spec, clientset)
		assert.NoError(t, err)
		assert.Nil(t, ingress)
	})
}

func TestDeleteConfigMapRules(t *testing.T) {
	configMap := &k8score.ConfigMap{ObjectMeta: createObjectMeta(AlertsConfigMapName, AlertsConfigMapNamespace)}
	configMap.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(configMap)

	spec := app.Spec{Application: appName, Namespace: namespace, Team: teamName}
	nonExistingSpec := app.Spec{Application: "nonexisting", Namespace: namespace, Team: teamName}

	t.Run("No error when deleting nonexistant app from alerts configmap", func(t *testing.T) {
		_, err := deleteConfigMapRules(nonExistingSpec, clientset)
		assert.NoError(t, err)
	})

	t.Run("No error when deleting alerts configmap for existing configmap", func(t *testing.T) {
		_, err := deleteConfigMapRules(spec, clientset)
		assert.NoError(t, err)
	})

	t.Run("No alert for appName existant after deletion", func(t *testing.T) {
		configmap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, clientset)
		assert.NoError(t, err)
		assert.NotNil(t, configmap)
		alert, _ := configmap.Data[teamName+appName+namespace+".yml"]
		assert.Empty(t, alert)
	})
}
