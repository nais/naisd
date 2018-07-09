package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestCreateRoleBinding(t *testing.T) {
	existingSpec := app.Spec{Application: "application", Environment: "environment", Team: "team"}
	nonExistingSpec := app.Spec{Application: "nonExistingApplication", Environment: "environment", Team: "team"}
	roleRef := createRoleRef("clusterrole", "serviceaccount-in-app-namespace")

	fakeClient := fake.NewSimpleClientset(createRoleBindingDef(existingSpec, roleRef))
	client := clientHolder{fakeClient}

	t.Run("Ensure not err when role already exists", func(t *testing.T) {
		r, err := client.createOrUpdateRoleBinding(existingSpec, roleRef)

		assert.NoError(t, err)
		assert.Equal(t, r.Name, existingSpec.ResourceName())
		assert.Equal(t, r.RoleRef, roleRef)
	})

	t.Run("Ensure role created if it does not already exist", func(t *testing.T) {
		r, err := client.createOrUpdateRoleBinding(nonExistingSpec, roleRef)

		assert.NoError(t, err)
		assert.Equal(t, r.Name, nonExistingSpec.ResourceName())
		assert.Equal(t, r.RoleRef, roleRef)
	})
}
