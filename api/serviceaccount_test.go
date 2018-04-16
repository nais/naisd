package api

import (
	"testing"
	"k8s.io/client-go/kubernetes/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func TestCreateOrUpdateServiceAccount(t *testing.T) {
	var name, namespace, team = "name", "namespace", "team"

	t.Run("If no service account exists one is created", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		serviceAccount, e := NewServiceAccountInterface(clientset).CreateOrUpdate(name, namespace, team)

		assert.NoError(t, e)
		assert.NotNil(t, serviceAccount)
		assert.Equal(t, serviceAccount.Name, name)
		assert.Equal(t, serviceAccount.Namespace, namespace)

	})

	t.Run("If a service account exists it is updated", func(t *testing.T) {
		existingServiceAccount := createServiceAccountDef(name, namespace, team)
		existingServiceAccount.SetUID("uuid")
		clientset := fake.NewSimpleClientset(existingServiceAccount)

		serviceAccount, e := NewServiceAccountInterface(clientset).CreateOrUpdate(name, namespace, team)
		assert.NoError(t, e)
		assert.NotNil(t, serviceAccount)
		assert.Equal(t, serviceAccount.Name, "name")
		assert.NotEqual(t, existingServiceAccount, serviceAccount)

	})
}

func TestDeleteServiceAccount(t *testing.T) {
	var name, namespace, team = "name", "namespace", "team"

	t.Run("Delete a service account given service account name and namespace ", func(t *testing.T) {

		existingServiceAccount := createServiceAccountDef(name, namespace, team)
		existingServiceAccount.SetUID("uuid")
		clientset := fake.NewSimpleClientset(existingServiceAccount)
		serviceAccountInterface := NewServiceAccountInterface(clientset)

		e2 := serviceAccountInterface.Delete(existingServiceAccount.Name, existingServiceAccount.Namespace)
		assert.NoError(t, e2)

		sa, e3 := clientset.CoreV1().ServiceAccounts(namespace).Get(name, v1.GetOptions{})
		assert.Nil(t, sa)
		assert.True(t, errors.IsNotFound(e3))

	})

	t.Run("Deleting a non existant service account is a noop", func(t *testing.T) {
		assert.Nil(t, NewServiceAccountInterface(fake.NewSimpleClientset()).Delete(name, namespace))

	})
}
