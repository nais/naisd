package api

import (
	"testing"
	"gopkg.in/h2non/gock.v1"
	"github.com/stretchr/testify/assert"
)

func TestAppConfigUnmarshal(t *testing.T) {
	const repopath = "https://appconfig.repo"

	gock.New(repopath).
		Reply(200).
		File("testdata/nais.yaml")

	appConfig, err := fetchAppConfig(repopath)

	assert.NoError(t, err)

	assert.Equal(t, 79, appConfig.Port.Port)
	assert.Equal(t, 799, appConfig.Port.TargetPort)
	assert.Equal(t, "/api", appConfig.FasitResources.Exposed[0].Path)
	assert.Equal(t, "datasource", appConfig.FasitResources.Used[0].ResourceType)
	assert.Equal(t, "isAlive2", appConfig.Healthcheck.Liveness.Path)
	assert.Equal(t, "isReady2", appConfig.Healthcheck.Readiness.Path)
	assert.Equal(t, 10, appConfig.Replicas.Min)
	assert.Equal(t, 20, appConfig.Replicas.Max)
	assert.Equal(t, 2, appConfig.Replicas.CpuThresholdPercentage)
}

func TestAppConfigUsesDefaultValues(t *testing.T) {
	const repopath = "https://appconfig.repo"

	gock.New(repopath).
		Reply(200).
		File("testdata/nais_minimal.yaml")

	appConfig, err := fetchAppConfig(repopath)

	port := appConfig.Port

	assert.NoError(t, err)
	assert.Equal(t, 80, port.Port)
	assert.Equal(t, 8080, port.TargetPort)
	assert.Equal(t, "isAlive", appConfig.Healthcheck.Liveness.Path)
	assert.Equal(t, "isReady", appConfig.Healthcheck.Readiness.Path)
	assert.Equal(t, 0, len(appConfig.FasitResources.Exposed))
	assert.Equal(t, 0, len(appConfig.FasitResources.Exposed))
	assert.Equal(t, 2, appConfig.Replicas.Min)
	assert.Equal(t, 4, appConfig.Replicas.Max)
	assert.Equal(t, 50, appConfig.Replicas.CpuThresholdPercentage)
}

func TestAppConfigUsesPartialDefaultValues(t *testing.T) {
	const repopath = "https://appconfig.repo"

	gock.New(repopath).
		Reply(200).
		File("testdata/nais_partial.yaml")

	appConfig, err := fetchAppConfig(repopath)

	assert.NoError(t, err)
	assert.Equal(t, 10, appConfig.Replicas.Min)
	assert.Equal(t, 4, appConfig.Replicas.Max)
	assert.Equal(t, 2, appConfig.Replicas.CpuThresholdPercentage)
}