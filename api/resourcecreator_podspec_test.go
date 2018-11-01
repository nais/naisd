package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/nais/naisd/internal/vault"
	"github.com/nais/naisd/pkg/test"
	"github.com/stretchr/testify/assert"
	"testing"
)

var properEnabledVaultEnv = map[string]string{
	vault.EnvVaultAuthPath:      "authpath",
	vault.EnvVaultKVPath:        "kvpath",
	vault.EnvInitContainerImage: "image",
	vault.EnvVaultAddr:          "adr",
	vault.EnvVaultEnabled:       "true",
}

func TestPodSpecWithInitContainer(t *testing.T) {
	spec := app.Spec{
		Application: "app",
		Namespace:   "evn",
		Team:        "team",
	}
	var naisResources []NaisResource
	deploy := naisrequest.Deploy{
		Application: "app",
		Version:     "version",
	}
	manifest := GetDefaultManifest("app")

	t.Run("Does not add init container if feature is disabled",
		test.EnvWrapper(map[string]string{vault.EnvVaultEnabled: "false"},
			func(t *testing.T) {
				podSpec, e := createPodSpec(spec, deploy, GetDefaultManifest("app"), naisResources)

				assert.NoError(t, e)
				assert.NotEmpty(t, podSpec)
				assert.Empty(t, podSpec.Volumes)
				assert.Empty(t, podSpec.InitContainers)
				assert.Equal(t, len(podSpec.Containers), 1)
				assert.Empty(t, podSpec.Containers[0].VolumeMounts)

			}))

	t.Run("Error if secrets required, vault enabled but vault config missing",
		test.EnvWrapper(map[string]string{vault.EnvVaultEnabled: "true"},
			func(t *testing.T) {
				manifest.Secrets = true
				podSpec, e := createPodSpec(spec, deploy, manifest, naisResources)

				assert.Error(t, e)
				assert.Empty(t, podSpec)
			}))

	t.Run("Does not add init container if feature enabled but secrets not set in in manifest",
		test.EnvWrapper(properEnabledVaultEnv,
			func(t *testing.T) {
				manifest.Secrets = false
				podSpec, e := createPodSpec(spec, deploy, manifest, naisResources)

				assert.NoError(t, e)
				assert.NotEmpty(t, podSpec)
				assert.Empty(t, podSpec.Volumes)
				assert.Empty(t, podSpec.InitContainers)
				assert.Equal(t, len(podSpec.Containers), 1)
				assert.Empty(t, podSpec.Containers[0].VolumeMounts)

			}))

	t.Run("Add initcontainer if vault enabled and secrets is true in manifest",
		test.EnvWrapper(properEnabledVaultEnv,
			func(t *testing.T) {
				manifest.Secrets = true
				podSpec, e := createPodSpec(spec, deploy, manifest, naisResources)

				assert.NoError(t, e)
				assert.NotEmpty(t, podSpec.Volumes)
				assert.NotEmpty(t, podSpec.InitContainers)
				assert.Equal(t, len(podSpec.Containers), 1)
				assert.NotEmpty(t, podSpec.Containers[0].VolumeMounts)

			}))
}
