package api

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestEnsureNamespaceExists(t *testing.T) {
	nonExistingNamespace := "nonexistingnamespace"
	existingNamespace := "existingnamespace"
	fakeClient := fake.NewSimpleClientset(createNamespaceDef(existingNamespace))
	client := clientHolder{fakeClient}

	t.Run("Ensure not err when namespace already exists", func(t *testing.T) {
		ns, err := client.createNamespace(existingNamespace)

		assert.NoError(t, err)
		assert.Equal(t, ns.ObjectMeta.Name, existingNamespace)
	})

	t.Run("Ensure namespace created if namespace does not exist", func(t *testing.T) {
		ns, err := client.createNamespace(nonExistingNamespace)

		assert.NoError(t, err)
		assert.Equal(t, ns.ObjectMeta.Name, nonExistingNamespace)
	})
}
