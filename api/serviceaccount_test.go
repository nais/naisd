package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestCreateOrUpdateServiceAccount(t *testing.T) {
	var name, environment, team = "name", "environment", "team"
	spec := app.Spec{Application: name, Environment: environment, Team: team}

	t.Run("If no service account exists one is created", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		serviceAccount, e := NewServiceAccountInterface(clientset).CreateServiceAccountIfNotExist(spec)

		assert.NoError(t, e)
		assert.NotNil(t, serviceAccount)

		sa, err := clientset.CoreV1().ServiceAccounts(spec.Namespace()).Get(spec.ResourceName(), v1.GetOptions{})
		assert.NotNil(t, sa)
		assert.NoError(t, err)
		assert.Equal(t, spec.ResourceName(), sa.Name)
	})

	t.Run("If a service account exists do nothing ", func(t *testing.T) {
		existingServiceAccount := createServiceAccountDef(spec)
		clientset := fake.NewSimpleClientset(existingServiceAccount)

		newServiceAccount, e := NewServiceAccountInterface(clientset).CreateServiceAccountIfNotExist(spec)
		assert.NoError(t, e)
		assert.NotNil(t, newServiceAccount)

		assert.Equal(t, existingServiceAccount, newServiceAccount)

	})
}

func TestDeleteServiceAccount(t *testing.T) {
	var name, environment, team = "name", "environment", "team"
	spec := app.Spec{Application: name, Environment: environment, Team: team}

	t.Run("Delete a service account given service account name and environment ", func(t *testing.T) {
		existingServiceAccount := createServiceAccountDef(spec)
		existingServiceAccount.SetUID("uuid")
		clientset := fake.NewSimpleClientset(existingServiceAccount)
		serviceAccountInterface := NewServiceAccountInterface(clientset)

		e2 := serviceAccountInterface.DeleteServiceAccount(spec)
		assert.NoError(t, e2)

		sa, e3 := clientset.CoreV1().ServiceAccounts(team).Get(name, v1.GetOptions{})
		assert.Nil(t, sa)
		assert.True(t, errors.IsNotFound(e3))
	})

	t.Run("Deleting a non existant service account is a noop", func(t *testing.T) {
		assert.Nil(t, NewServiceAccountInterface(fake.NewSimpleClientset()).DeleteServiceAccount(spec))
	})
}
