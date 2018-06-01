package api

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestCreateOrUpdateServiceAccount(t *testing.T) {
	var name, environment, team = "name", "environment", "team"

	t.Run("If no service account exists one is created", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		serviceAccount, e := NewServiceAccountInterface(clientset).CreateOrUpdate(name, environment, team)

		assert.NoError(t, e)
		assert.NotNil(t, serviceAccount)

		sa, err := clientset.CoreV1().ServiceAccounts(team).Get(createObjectName(name, environment), v1.GetOptions{})
		assert.NotNil(t, sa)
		assert.NoError(t, err)
		assert.Equal(t, createObjectName(name, environment), sa.Name)
	})

	t.Run("If a service account exists it is updated", func(t *testing.T) {
		existingServiceAccount := createServiceAccountDef(name, environment, team)
		existingServiceAccount.SetResourceVersion("001")
		clientset := fake.NewSimpleClientset(existingServiceAccount)

		_, e := NewServiceAccountInterface(clientset).CreateOrUpdate(name, environment, team)
		assert.NoError(t, e)

		updatedServiceAccount, _ := clientset.CoreV1().ServiceAccounts(team).Get(createObjectName(name, environment), v1.GetOptions{})
		assert.NotNil(t, updatedServiceAccount)
		assert.NotEqual(t, existingServiceAccount, updatedServiceAccount)

	})
}

func TestDeleteServiceAccount(t *testing.T) {
	var application, environment, team = "application", "environment", "team"
	objectName := createObjectName(team, environment)

	t.Run("Delete a service account given service account application and environment ", func(t *testing.T) {
		existingServiceAccount := createServiceAccountDef(application, environment, team)
		existingServiceAccount.SetUID("uuid")
		clientset := fake.NewSimpleClientset(existingServiceAccount)
		serviceAccountInterface := NewServiceAccountInterface(clientset)

		e2 := serviceAccountInterface.Delete(existingServiceAccount.Name, environment, team)
		assert.NoError(t, e2)

		sa, e3 := clientset.CoreV1().ServiceAccounts(team).Get(objectName, v1.GetOptions{})
		assert.Nil(t, sa)
		assert.True(t, errors.IsNotFound(e3))
	})

	t.Run("Deleting a non existant service account is a noop", func(t *testing.T) {
		assert.Nil(t, NewServiceAccountInterface(fake.NewSimpleClientset()).Delete(application, environment, team))
	})
}
