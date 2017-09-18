package api

import (
	"testing"
	"gopkg.in/h2non/gock.v1"
	"github.com/stretchr/testify/assert"
)

func TestAppConfigUnmarshal(t *testing.T) {
	const repopath = "https://appconfig.repo"
	defer gock.Off()

	gock.New(repopath).
		Reply(200).
		File("testdata/nais.yaml")

	appConfig, err := fetchAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath})

	assert.NoError(t, err)

	assert.Equal(t, 799, appConfig.Port)
	assert.Equal(t, "/api", appConfig.FasitResources.Exposed[0].Path)
	assert.Equal(t, "datasource", appConfig.FasitResources.Used[0].ResourceType)
	assert.Equal(t, "isAlive2", appConfig.Healthcheck.Liveness.Path)
	assert.Equal(t, "isReady2", appConfig.Healthcheck.Readiness.Path)
	assert.Equal(t, 10, appConfig.Replicas.Min)
	assert.Equal(t, 20, appConfig.Replicas.Max)
	assert.Equal(t, 20, appConfig.Replicas.CpuThresholdPercentage)
	assert.True(t, gock.IsDone(), "verifies that the appconfigUrl has been called")
	assert.Equal(t, "100m", appConfig.Resources.Limits.Cpu)
	assert.Equal(t, "100Mi", appConfig.Resources.Limits.Memory)
	assert.Equal(t, "100m", appConfig.Resources.Requests.Cpu)
	assert.Equal(t, "100Mi", appConfig.Resources.Requests.Memory)
	assert.Equal(t, true, appConfig.Prometheus.Enabled)
	assert.Equal(t, DefaultPortName, appConfig.Prometheus.Port)
	assert.Equal(t, "/path", appConfig.Prometheus.Path)
	assert.Equal(t, 79, appConfig.Healthcheck.Liveness.InitialDelay)
	assert.Equal(t, 79, appConfig.Healthcheck.Readiness.InitialDelay)
}

func TestAppConfigUsesDefaultValues(t *testing.T) {
	appConfig, err := fetchAppConfig(NaisDeploymentRequest{NoAppConfig: true})

	assert.NoError(t, err)
	assert.Equal(t, 8080, appConfig.Port)
	assert.Equal(t, "isAlive", appConfig.Healthcheck.Liveness.Path)
	assert.Equal(t, "isReady", appConfig.Healthcheck.Readiness.Path)
	assert.Equal(t, 0, len(appConfig.FasitResources.Exposed))
	assert.Equal(t, 0, len(appConfig.FasitResources.Exposed))
	assert.Equal(t, 2, appConfig.Replicas.Min)
	assert.Equal(t, 4, appConfig.Replicas.Max)
	assert.Equal(t, 50, appConfig.Replicas.CpuThresholdPercentage)
	assert.Equal(t, "500m", appConfig.Resources.Limits.Cpu)
	assert.Equal(t, "512Mi", appConfig.Resources.Limits.Memory)
	assert.Equal(t, "200m", appConfig.Resources.Requests.Cpu)
	assert.Equal(t, "256Mi", appConfig.Resources.Requests.Memory)
	assert.Equal(t, false, appConfig.Prometheus.Enabled)
	assert.Equal(t, DefaultPortName, appConfig.Prometheus.Port)
	assert.Equal(t, "/metrics", appConfig.Prometheus.Path)
	assert.Equal(t, 20, appConfig.Healthcheck.Readiness.InitialDelay)
	assert.Equal(t, 20, appConfig.Healthcheck.Liveness.InitialDelay)
}

func TestAppConfigUsesPartialDefaultValues(t *testing.T) {
	const repopath = "https://appconfig.repo"
	defer gock.Off()
	gock.New(repopath).
		Reply(200).
		File("testdata/nais_partial.yaml")

	appConfig, err := fetchAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath})

	assert.NoError(t, err)
	assert.Equal(t, 2, appConfig.Replicas.Min)
	assert.Equal(t, 10, appConfig.Replicas.Max)
	assert.Equal(t, 15, appConfig.Replicas.CpuThresholdPercentage)
}

func TestNoAppConfigFlagCreatesAppconfigFromDefaults(t *testing.T) {
	image := "docker.adeo.no:5000/" + appName + ":" + version
	const repopath = "https://appconfig.repo"
	defer gock.Off()
	gock.New(repopath).
		Reply(200)

	appConfig, err := fetchAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath, NoAppConfig: true, Application: appName, Version: version})

	assert.NoError(t, err)
	assert.Equal(t, image, appConfig.Image, "If no Image provided, a default is created")
	assert.True(t, gock.IsPending(), "No calls to appConfigUrl registered")
}

func TestInvalidReplicasConfigGivesValidationErrors(t *testing.T) {
	const repopath = "https://appconfig.repo"
	defer gock.Off()
	gock.New(repopath).
		Reply(200).
		File("testdata/nais_error.yaml")

	_, err := fetchAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath})
	assert.Error(t, err)
}

func TestIncalidCouThreshold(t *testing.T) {
	invalidConfig := NaisAppConfig{
		Replicas: Replicas{
			CpuThresholdPercentage: 5,
			Max:                    4,
			Min:                    5,
		},
	}
	errors := validateAppConfig(invalidConfig)
	t.Log(errors)

	assert.Equal(t, 2, len(errors.Errors))
}
func TestMinCannotBeZero(t *testing.T) {
	invalidConfig := NaisAppConfig{
		Replicas: Replicas{
			CpuThresholdPercentage: 50,
			Max:                    4,
			Min:                    0,
		},
	}
	errors := validateAppConfig(invalidConfig)

	assert.Equal(t, 1, len(errors.Errors))
}
