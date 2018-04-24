package api

import (
	"github.com/hashicorp/go-multierror"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"testing"
)

func TestManifestUnmarshal(t *testing.T) {
	const repopath = "https://manifest.repo"
	defer gock.Off()

	gock.New(repopath).
		Reply(200).
		File("testdata/nais.yaml")

	manifest, err := GenerateManifest(naisrequest.Deploy{ManifestUrl: repopath})

	assert.NoError(t, err)

	assert.Equal(t, "teamName", manifest.Team)
	assert.Equal(t, 799, manifest.Port)
	assert.Equal(t, "/api", manifest.FasitResources.Exposed[0].Path)
	assert.Equal(t, "datasource", manifest.FasitResources.Used[0].ResourceType)
	assert.Equal(t, "DB_USER", manifest.FasitResources.Used[0].PropertyMap["username"])
	assert.Equal(t, "restservice", manifest.FasitResources.Used[1].ResourceType)
	assert.Nil(t, manifest.FasitResources.Used[1].PropertyMap)
	assert.Equal(t, "isAlive2", manifest.Healthcheck.Liveness.Path)
	assert.Equal(t, "isReady2", manifest.Healthcheck.Readiness.Path)
	assert.Equal(t, 10, manifest.Replicas.Min)
	assert.Equal(t, 20, manifest.Replicas.Max)
	assert.Equal(t, 20, manifest.Replicas.CpuThresholdPercentage)
	assert.True(t, gock.IsDone(), "verifies that the manifestUrl has been called")
	assert.Equal(t, "100m", manifest.Resources.Limits.Cpu)
	assert.Equal(t, "100Mi", manifest.Resources.Limits.Memory)
	assert.Equal(t, "100m", manifest.Resources.Requests.Cpu)
	assert.Equal(t, "100Mi", manifest.Resources.Requests.Memory)
	assert.Equal(t, true, manifest.Prometheus.Enabled)
	assert.Equal(t, DefaultPortName, manifest.Prometheus.Port)
	assert.Equal(t, "/path", manifest.Prometheus.Path)
	assert.Equal(t, true, manifest.Istio.Enabled)
	assert.Equal(t, 79, manifest.Healthcheck.Liveness.InitialDelay)
	assert.Equal(t, 79, manifest.Healthcheck.Readiness.InitialDelay)
	assert.Equal(t, 15, manifest.Healthcheck.Liveness.FailureThreshold)
	assert.Equal(t, 3, manifest.Healthcheck.Readiness.FailureThreshold)
	assert.Equal(t, 69, manifest.Healthcheck.Readiness.Timeout)
	assert.Equal(t, 5, manifest.Healthcheck.Liveness.PeriodSeconds)
	assert.Equal(t, 10, manifest.Healthcheck.Readiness.PeriodSeconds)
	assert.Equal(t, 69, manifest.Healthcheck.Liveness.Timeout)
	assert.Equal(t, "/stop", manifest.PreStopHookPath)
	assert.Equal(t, true, manifest.Ingress.Disabled)
	assert.Equal(t, "Nais-testapp deployed", manifest.Alerts[0].Alert)
	assert.Equal(t, "kube_deployment_status_replicas_unavailable{deployment=\"nais-testapp\"} > 0", manifest.Alerts[0].Expr)
	assert.Equal(t, "5m", manifest.Alerts[0].For)
	assert.Equal(t, "Investigate why nais-testapp can't spawn pods. kubectl describe deployment nais-testapp, kubectl describe pod nais-testapp-*.", manifest.Alerts[0].Annotations["action"])
	assert.Equal(t, "Critical", manifest.Alerts[1].Labels["severity"])
}

func TestManifestUsesDefaultValues(t *testing.T) {

	const repopath = "https://manifest.repo"
	defer gock.Off()

	gock.New(repopath).
		Reply(200).
		File("testdata/nais_minimal.yaml")

	manifest, err := GenerateManifest(naisrequest.Deploy{ManifestUrl: repopath})

	assert.NoError(t, err)
	assert.Equal(t, "docker.adeo.no:5000/", manifest.Image)
	assert.Equal(t, 8080, manifest.Port)
	assert.Equal(t, "isAlive", manifest.Healthcheck.Liveness.Path)
	assert.Equal(t, "isReady", manifest.Healthcheck.Readiness.Path)
	assert.Equal(t, 0, len(manifest.FasitResources.Exposed))
	assert.Equal(t, 0, len(manifest.FasitResources.Exposed))
	assert.Equal(t, 2, manifest.Replicas.Min)
	assert.Equal(t, 4, manifest.Replicas.Max)
	assert.Equal(t, 50, manifest.Replicas.CpuThresholdPercentage)
	assert.Equal(t, "500m", manifest.Resources.Limits.Cpu)
	assert.Equal(t, "512Mi", manifest.Resources.Limits.Memory)
	assert.Equal(t, "200m", manifest.Resources.Requests.Cpu)
	assert.Equal(t, "256Mi", manifest.Resources.Requests.Memory)
	assert.Equal(t, false, manifest.Prometheus.Enabled)
	assert.Equal(t, DefaultPortName, manifest.Prometheus.Port)
	assert.Equal(t, "/metrics", manifest.Prometheus.Path)
	assert.Equal(t, false, manifest.Istio.Enabled)
	assert.Equal(t, 20, manifest.Healthcheck.Readiness.InitialDelay)
	assert.Equal(t, 20, manifest.Healthcheck.Liveness.InitialDelay)
	assert.Equal(t, 1, manifest.Healthcheck.Liveness.Timeout)
	assert.Equal(t, 1, manifest.Healthcheck.Readiness.Timeout)
	assert.Equal(t, false, manifest.Ingress.Disabled)
	assert.Empty(t, manifest.PreStopHookPath)
	assert.Empty(t, manifest.Team)
}

func TestManifestUsesPartialDefaultValues(t *testing.T) {
	const repopath = "https://manifest.repo"
	defer gock.Off()
	gock.New(repopath).
		Reply(200).
		File("testdata/nais_partial.yaml")

	manifest, err := GenerateManifest(naisrequest.Deploy{ManifestUrl: repopath})

	assert.NoError(t, err)
	assert.Equal(t, 2, manifest.Replicas.Min)
	assert.Equal(t, 10, manifest.Replicas.Max)
	assert.Equal(t, 15, manifest.Replicas.CpuThresholdPercentage)
}

func TestGenerateManifestWithoutPassingRepoUrl(t *testing.T) {
	application := "appName"
	version := "42"
	urls := createManifestUrl(application, version)
	t.Run("When no manifest found an error is returned", func(t *testing.T) {
		defer gock.Off()
		gock.New(urls[0]).
			Reply(404)
		gock.New(urls[1]).
			Reply(404)
		gock.New(urls[2]).
			Reply(404)

		_, err := GenerateManifest(naisrequest.Deploy{Application: application, Version: version})
		assert.Error(t, err)
		assert.True(t, gock.IsDone())
	})
	t.Run("When no manifest found at first or second default URL, the third is called", func(t *testing.T) {
		defer gock.Off()
		gock.New(urls[0]).
			Reply(404)
		gock.New(urls[1]).
			Reply(404)
		gock.New(urls[2]).
			Reply(200).
			JSON(map[string]string{"image": application})

		manifest, err := GenerateManifest(naisrequest.Deploy{Application: application, Version: version})
		assert.NoError(t, err)
		assert.Equal(t, application, manifest.Image)
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

		manifest, err := GenerateManifest(naisrequest.Deploy{Application: application, Version: version})
		assert.NoError(t, err)
		assert.Equal(t, application, manifest.Image)
		assert.True(t, gock.IsPending())
	})
}

func TestDownLoadManifestErrors(t *testing.T) {
	request := naisrequest.Deploy{
		Application: "appname",
		Version:     "42",
	}
	urls := createManifestUrl(request.Application, request.Version)

	t.Run("Single error is wrapped correctly ", func(t *testing.T) {
		defer gock.Off()
		gock.New(urls[0]).
			Reply(404)

		_, err := downloadManifest(naisrequest.Deploy{ManifestUrl: urls[0]})
		assert.Error(t, err)
		merr, _ := err.(*multierror.Error)
		assert.Equal(t, 1, len(merr.Errors))
	})

	t.Run("Multiple errors are wrapped correctly", func(t *testing.T) {

		defer gock.Off()
		gock.New(urls[0]).
			Reply(404)
		gock.New(urls[1]).
			Reply(404)
		gock.New(urls[2]).
			Reply(200).
			File("testdata/nais_yaml_error.yaml")
		_, err := downloadManifest(request)

		assert.Error(t, err)
		merr, _ := err.(*multierror.Error)
		assert.Equal(t, 3, len(merr.Errors))
	})
}

func TestInvalidReplicasConfigGivesValidationErrors(t *testing.T) {
	const repopath = "https://manifest.repo"
	defer gock.Off()
	gock.New(repopath).
		Reply(200).
		File("testdata/nais_error.yaml")

	_, err := GenerateManifest(naisrequest.Deploy{ManifestUrl: repopath})
	assert.Error(t, err)
}

func TestMultipleInvalidManifestFields(t *testing.T) {
	invalidConfig := NaisManifest{
		Image: "myapp:1",
		Replicas: Replicas{
			CpuThresholdPercentage: 5,
			Max:                    4,
			Min:                    5,
		},
	}
	errors := ValidateManifest(invalidConfig)

	assert.Equal(t, 5, len(errors.Errors))
	assert.Equal(t, "Image cannot contain tag", errors.Errors[0].ErrorMessage)
	assert.Equal(t, "Replicas.Min is larger than Replicas.Max.", errors.Errors[1].ErrorMessage)
	assert.Equal(t, "CpuThreshold must be between 10 and 90.", errors.Errors[2].ErrorMessage)
	assert.Equal(t, "not a valid memory quantity. quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'", errors.Errors[3].ErrorMessage)
	assert.Equal(t, "not a valid memory quantity. quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'", errors.Errors[4].ErrorMessage)
}

func TestInvalidCpuThreshold(t *testing.T) {
	invalidManifest := NaisManifest{
		Replicas: Replicas{
			CpuThresholdPercentage: 5,
			Max:                    4,
			Min:                    5,
		},
	}
	errors := validateCpuThreshold(invalidManifest)
	assert.Equal(t, "CpuThreshold must be between 10 and 90.", errors.ErrorMessage)
}

func TestMinCannotBeZero(t *testing.T) {
	invalidManifest := NaisManifest{
		Replicas: Replicas{
			CpuThresholdPercentage: 50,
			Max:                    4,
			Min:                    0,
		},
	}
	errors := validateReplicasMin(invalidManifest)

	assert.Equal(t, "Replicas.Min is not set", errors.ErrorMessage)
}

func TestMemoryNotation(t *testing.T) {
	newDefaultManifest()
	manifest := NaisManifest{
		Resources: ResourceRequirements{
			Requests: ResourceList{
				Memory: "200Mi",
			},
			Limits: ResourceList{
				Memory: "400Mi",
			},
		},
	}

	errors := validateRequestMemoryQuantity(manifest)
	assert.Nil(t, errors)
	errors = validateLimitsMemoryQuantity(manifest)
	assert.Nil(t, errors)

	manifest.Resources.Limits.Memory = "200"
	errors = validateLimitsMemoryQuantity(manifest)
	assert.Nil(t, errors)

	manifest.Resources.Requests.Memory = "200"
	errors = validateRequestMemoryQuantity(manifest)
	assert.Nil(t, errors)

	manifest.Resources.Limits.Memory = "200M"
	errors = validateLimitsMemoryQuantity(manifest)
	assert.Nil(t, errors)

	manifest.Resources.Requests.Memory = "200M"
	errors = validateRequestMemoryQuantity(manifest)
	assert.Nil(t, errors)

	manifest.Resources.Limits.Memory = "200i"
	errors = validateLimitsMemoryQuantity(manifest)
	assert.Equal(t, "not a valid memory quantity. unable to parse quantity's suffix", errors.ErrorMessage)

	manifest.Resources.Requests.Memory = "200i"
	errors = validateRequestMemoryQuantity(manifest)
	assert.Equal(t, "not a valid memory quantity. unable to parse quantity's suffix", errors.ErrorMessage)
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
			manifest := NaisManifest{
				Image: v.name,
			}

			err := validateImage(manifest)

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
	invalidManifest := NaisManifest{
		FasitResources: FasitResources{
			Exposed: []ExposedResource{{Alias: "alias1"}},
			Used:    []UsedResource{{ResourceType: "restService"}},
		},
	}
	invalidManifest2 := NaisManifest{
		FasitResources: FasitResources{
			Exposed: []ExposedResource{{ResourceType: "restService"}},
			Used:    []UsedResource{{Alias: "alias1"}},
		},
	}
	validManifest := NaisManifest{
		FasitResources: FasitResources{
			Exposed: []ExposedResource{{ResourceType: "restService", Alias: "alias1"}},
			Used:    []UsedResource{{ResourceType: "restService", Alias: "alias2"}},
		},
	}
	err := validateResources(invalidManifest)
	err2 := validateResources(invalidManifest2)
	noErr := validateResources(validManifest)
	assert.Equal(t, "Alias and ResourceType must be specified", err.ErrorMessage)
	assert.Equal(t, "Alias and ResourceType must be specified", err2.ErrorMessage)
	assert.Nil(t, noErr)
}
