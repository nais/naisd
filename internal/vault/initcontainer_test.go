package vault

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"os"
	"github.com/nais/naisd/api/app"
)

func TestFeatureFlagging(t *testing.T) {
	t.Run("Vault should by default be disabled", func(t *testing.T) {
		assert.False(t, Enabled())
	})

	t.Run("Feature flag is configured through env variables", func(t *testing.T) {
		os.Setenv(envVaultEnabled, "true")

		assert.True(t, Enabled())

		os.Unsetenv(envVaultEnabled)

	})
}

func TestConfigValidation(t *testing.T) {

	t.Run("Validation should fail if one or more config value is missing", enableVault(func(t *testing.T) {

		var tests = []struct {
			config         vaultConfig
			expectedResult bool
		}{
			{vaultConfig{vaultAddr: "addr"}, false},
			{vaultConfig{vaultAddr: "addr", kvPath: "path"}, false},
			{vaultConfig{vaultAddr: "addr", kvPath: "path", authPath: "auth"}, false},
		}

		for _, test := range tests {
			actualResult, err := test.config.validate()
			assert.Equal(t, test.expectedResult, actualResult)
			assert.NotNil(t, err)
		}
	}))

	t.Run("Validation should pass if all config values are present", enableVault(func(t *testing.T) {
		result, err := vaultConfig{vaultAddr: "addr", kvPath: "path", authPath: "auth", initContainerImage: "image"}.validate()
		assert.True(t, result)
		assert.Nil(t, err)

	}))
}

func TestNewInitializer(t *testing.T) {

	t.Run("Initializer is configured through environment variables", enableVault(func(t *testing.T) {
		envVars := map[string]string{
			envVaultAuthPath:      "authpath",
			envVaultKVPath:        "kvpath",
			envInitContainerImage: "image",
			envVaultAddr:          "adr",
		}

		for k, v := range envVars {
			os.Setenv(k, v)
		}

		aInitializer, e := NewInitializer(app.Spec{})
		assert.NoError(t, e)
		assert.NotNil(t, aInitializer)

		initializerStruct, ok := aInitializer.(initializer)
		assert.True(t, ok)

		config := initializerStruct.config
		assert.NotNil(t, config)
		assert.Equal(t, envVars[envVaultAddr], config.vaultAddr)
		assert.Equal(t, envVars[envInitContainerImage], config.initContainerImage)
		assert.Equal(t, envVars[envVaultKVPath], config.kvPath)
		assert.Equal(t, envVars[envVaultAuthPath], config.authPath)

		for k, _ := range envVars {
			os.Unsetenv(k)
		}

	}))

	t.Run("Fail initializer creation if config validation fails", enableVault(func(t *testing.T) {
		_, err := NewInitializer(app.Spec{})
		assert.Error(t, err)
	}))
}

func TestKVPath(t *testing.T) {
	initializer := initializer{
		config: vaultConfig{
			kvPath: "path/kvpath",
		},
		spec: app.Spec{
			Environment: "env",
			Application: "app",
		},
	}

	assert.Equal(t, "path/kvpath/app/env", initializer.kvPath())
}


func enableVault(test func(t *testing.T)) func(t *testing.T) {
	return func(t *testing.T) {
		os.Setenv(envVaultEnabled, "true")
		test(t)
		os.Unsetenv(envVaultEnabled)
	}
}
