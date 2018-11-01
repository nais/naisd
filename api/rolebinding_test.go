package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestCreateRoleBinding(t *testing.T) {
	existingSpec := app.Spec{Application: "application", Namespace: "default", Team: "team"}
	nonExistingSpec := app.Spec{Application: "nonExistingApplication", Namespace: "default", Team: "team"}
	roleRef := createRoleRef("clusterrole", "serviceaccount-in-app-namespace")

	fakeClient := fake.NewSimpleClientset(createRoleBindingDef(existingSpec, roleRef))
	client := clientHolder{fakeClient}

	t.Run("Ensure not err when RoleBinding already exists", func(t *testing.T) {
		r, err := client.createOrUpdateRoleBinding(existingSpec, roleRef)

		assert.NoError(t, err)
		assert.Equal(t, r.Name, existingSpec.ResourceName())
		assert.Equal(t, r.RoleRef, roleRef)
	})

	t.Run("Ensure RoleBinding created if it does not already exist", func(t *testing.T) {
		r, err := client.createOrUpdateRoleBinding(nonExistingSpec, roleRef)

		assert.NoError(t, err)
		assert.Equal(t, r.Name, nonExistingSpec.ResourceName())
		assert.Equal(t, r.RoleRef, roleRef)
	})

	t.Run("Ensure deleting RoleBindings give no error and removes resources", func(t *testing.T) {
		newSpec := app.Spec{Application: appName, Namespace: namespace, Team: teamName}
		newNonExistingSpec := app.Spec{Application: "nothing", Namespace: namespace, Team: teamName}

		_, err := client.createOrUpdateRoleBinding(newSpec, roleRef)
		assert.NoError(t, err)

		err = client.deleteRoleBinding(newSpec)
		assert.NoError(t, err)

		err = client.deleteRoleBinding(newNonExistingSpec)
		assert.NoError(t, err)

		rolebinding, err := client.client.RbacV1().RoleBindings(newSpec.Namespace).Get(newSpec.ResourceName(), v1.GetOptions{})
		assert.Nil(t, rolebinding)
		assert.True(t, errors.IsNotFound(err))

		rolebinding, err = client.client.RbacV1().RoleBindings(newNonExistingSpec.Namespace).Get(newNonExistingSpec.ResourceName(), v1.GetOptions{})
		assert.Nil(t, rolebinding)
		assert.True(t, errors.IsNotFound(err))
	})
}
