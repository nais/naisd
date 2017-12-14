package api

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"testing"
)

func TestAppConfigUnmarshal(t *testing.T) {
	const repopath = "https://appconfig.repo"
	defer gock.Off()

	gock.New(repopath).
		Reply(200).
		File("testdata/nais.yaml")

	appConfig, err := GenerateAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath})

	assert.NoError(t, err)

	assert.Equal(t, 799, appConfig.Port)
	assert.Equal(t, "/api", appConfig.FasitResources.Exposed[0].Path)
	assert.Equal(t, "datasource", appConfig.FasitResources.Used[0].ResourceType)
	assert.Equal(t, "DB_USER", appConfig.FasitResources.Used[0].PropertyMap["username"])
	assert.Equal(t, "restservice", appConfig.FasitResources.Used[1].ResourceType)
	assert.Nil(t, appConfig.FasitResources.Used[1].PropertyMap)
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
	assert.Equal(t, 15, appConfig.Healthcheck.Liveness.FailureThreshold)
	assert.Equal(t, 3, appConfig.Healthcheck.Readiness.FailureThreshold)
	assert.Equal(t, 5, appConfig.Healthcheck.Liveness.PeriodSeconds)
	assert.Equal(t, 10, appConfig.Healthcheck.Readiness.PeriodSeconds)
	assert.Equal(t, "/stop", appConfig.PreStopHookPath)
}

func TestAppConfigUsesDefaultValues(t *testing.T) {
	appConfig, err := GenerateAppConfig(NaisDeploymentRequest{NoAppConfig: true})

	assert.NoError(t, err)
	assert.Equal(t, "docker.adeo.no:5000/", appConfig.Image)
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
	assert.Equal(t, true, appConfig.Ingress.Enabled)
	assert.Empty(t, appConfig.PreStopHookPath)

}

func TestAppConfigUsesPartialDefaultValues(t *testing.T) {
	const repopath = "https://appconfig.repo"
	defer gock.Off()
	gock.New(repopath).
		Reply(200).
		File("testdata/nais_partial.yaml")

	appConfig, err := GenerateAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath})

	assert.NoError(t, err)
	assert.Equal(t, 2, appConfig.Replicas.Min)
	assert.Equal(t, 10, appConfig.Replicas.Max)
	assert.Equal(t, 15, appConfig.Replicas.CpuThresholdPercentage)
}

func TestGenerateAppConfigWithoutPassingRepoUrl(t *testing.T) {
	application := "appName"
	version := "42"
	urls := createAppConfigUrls(application, version)
	t.Run("When no manifest found at first or second default URL, the third is called", func(t *testing.T) {
		defer gock.Off()
		gock.New(urls[0]).
			Reply(404)
		gock.New(urls[1]).
			Reply(404)
		gock.New(urls[2]).
			Reply(200).
			JSON(map[string]string{"image": application})

		appConfig, err := GenerateAppConfig(NaisDeploymentRequest{Application: application, Version: version})
		assert.NoError(t, err)
		assert.Equal(t, application, appConfig.Image)
		assert.True(t, gock.IsDone())
	})
	t.Run("When manifest found at first default URL, the second is not called", func(t *testing.T) {
		defer gock.Off()
		gock.New(urls[0]).
			Reply(200).
			JSON(map[string]string{"image": application})
		gock.New(urls[1]).
			Reply(200).
			JSON(map[string]string{"image": "incorrect"})

		appConfig, err := GenerateAppConfig(NaisDeploymentRequest{Application: application, Version: version})
		assert.NoError(t, err)
		assert.Equal(t, application, appConfig.Image)
		assert.True(t, gock.IsPending())
	})
}

func TestNoAppConfigFlagCreatesAppconfigFromDefaults(t *testing.T) {
	image := "docker.adeo.no:5000/" + appName
	const repopath = "https://appconfig.repo"
	defer gock.Off()
	gock.New(repopath).
		Reply(200)

	appConfig, err := GenerateAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath, NoAppConfig: true, Application: appName, Version: version})

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

	_, err := GenerateAppConfig(NaisDeploymentRequest{AppConfigUrl: repopath})
	assert.Error(t, err)
}

func TestMultipleInvalidAppConfigFields(t *testing.T) {
	invalidConfig := NaisAppConfig{
		Image: "myapp:1",
		Replicas: Replicas{
			CpuThresholdPercentage: 5,
			Max: 4,
			Min: 5,
		},
	}
	errors := ValidateAppConfig(invalidConfig)

	assert.Equal(t, 3, len(errors.Errors))
	assert.Equal(t, "Image cannot contain tag", errors.Errors[0].ErrorMessage)
	assert.Equal(t, "Replicas.Min is larger than Replicas.Max.", errors.Errors[1].ErrorMessage)
	assert.Equal(t, "CpuThreshold must be between 10 and 90.", errors.Errors[2].ErrorMessage)
}

func TestInvalidCpuThreshold(t *testing.T) {
	invalidConfig := NaisAppConfig{
		Replicas: Replicas{
			CpuThresholdPercentage: 5,
			Max: 4,
			Min: 5,
		},
	}
	errors := validateCpuThreshold(invalidConfig)
	assert.Equal(t, "CpuThreshold must be between 10 and 90.", errors.ErrorMessage)
}
func TestMinCannotBeZero(t *testing.T) {
	invalidConfig := NaisAppConfig{
		Replicas: Replicas{
			CpuThresholdPercentage: 50,
			Max: 4,
			Min: 0,
		},
	}
	errors := validateReplicasMin(invalidConfig)

	assert.Equal(t, "Replicas.Min is not set", errors.ErrorMessage)
}

func TestValidateImage(t *testing.T) {
	type TestCase struct {
		name  string
		valid bool
	}

	images := []TestCase{
		{"myapp", true},
		{"myapp:1", false},
		{"registry-1.docker.io:5000/myapp", true},
		{"registry-1.docker.io:5000/myapp:1", false},
	}

	for _, v := range images {
		t.Run("test "+v.name, func(t *testing.T) {
			appConfig := NaisAppConfig{
				Image: v.name,
			}

			err := validateImage(appConfig)

			if v.valid {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, "Image cannot contain tag", err.ErrorMessage)
				assert.Equal(t, v.name, err.Fields["Image"])
			}
		})
	}
}
func TestValidateResource(t *testing.T) {
	invalidConfig := NaisAppConfig{
		FasitResources: FasitResources{
			Exposed: []ExposedResource{{Alias: "alias1"}},
			Used:    []UsedResource{{ResourceType: "restService"}},
		},
	}
	invalidConfig2 := NaisAppConfig{
		FasitResources: FasitResources{
			Exposed: []ExposedResource{{ResourceType: "restService"}},
			Used:    []UsedResource{{Alias: "alias1"}},
		},
	}
	validConfig := NaisAppConfig{
		FasitResources: FasitResources{
			Exposed: []ExposedResource{{ResourceType: "restService", Alias: "alias1"}},
			Used:    []UsedResource{{ResourceType: "restService", Alias: "alias2"}},
		},
	}
	err := validateResources(invalidConfig)
	err2 := validateResources(invalidConfig2)
	noErr := validateResources(validConfig)
	assert.Equal(t, "Alias and ResourceType must be specified", err.ErrorMessage)
	assert.Equal(t, "Alias and ResourceType must be specified", err2.ErrorMessage)
	assert.Nil(t, noErr)
}
