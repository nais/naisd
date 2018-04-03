package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateService(t *testing.T) {
	objectMeta := CreateObjectMeta(appName, namespace, teamName)
	service := createServiceDef(objectMeta)
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
		service, err := createService(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName, Version: version}, NaisManifest{Team: otherTeamName}, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, service.ObjectMeta.Name)
		assert.Equal(t, otherTeamName, service.ObjectMeta.Labels["team"])
		assert.Equal(t, DefaultPortName, service.Spec.Ports[0].TargetPort.StrVal)
		assert.Equal(t, "http", service.Spec.Ports[0].Name)
		assert.Equal(t, map[string]string{"app": otherAppName}, service.Spec.Selector)
	})

	t.Run("when service exists, nothing happens", func(t *testing.T) {
		nilValue, err := createService(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, NaisManifest{Team: teamName}, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})
}
