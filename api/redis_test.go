package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"testing"
)

func TestRedisResource(t *testing.T) {
	redisName := fmt.Sprintf("%s-redis", appName)

	t.Run("Replicas should always be 1", func(t *testing.T) {
		spec := app.Spec{Application: appName, Namespace: namespace, Team: "teamBeam"}
		manifest := NaisManifest{Redis: Redis{Enabled: true}}
		manifest.Redis = updateDefaultRedisValues(manifest.Redis)
		deploymentSpec := createRedisDeploymentSpec(redisName, spec, manifest.Redis)
		expectedReplicas := int32(1)
		assert.Equal(t, &expectedReplicas, deploymentSpec.Replicas)
	})

	t.Run("Custom resources", func(t *testing.T) {
		manifest := NaisManifest{
			Redis: Redis{
				Enabled: true,
				Limits: ResourceList{
					Cpu: "1337m",
				},
				Requests: ResourceList{
					Memory: "512Mi",
				},
			},
		}
		manifest.Redis = updateDefaultRedisValues(manifest.Redis)

		podSpec := createRedisPodSpec(manifest.Redis)
		container := podSpec.Containers[0]
		assert.Equal(t, "redis", container.Name)
		resources := container.Resources
		assert.Equal(t, "1337m", resources.Limits.Cpu().String())
		assert.Equal(t, "128Mi", resources.Limits.Memory().String())
		assert.Equal(t, "100m", resources.Requests.Cpu().String())
		assert.Equal(t, "512Mi", resources.Requests.Memory().String())
	})

	t.Run("REDIS_HOST env var should be set when redis: true", func(t *testing.T) {
		spec := app.Spec{Application: appName, Namespace: namespace, Team: "teamBeam"}
		manifest := NaisManifest{Redis: Redis{Enabled: true}}
		manifest.Redis = updateDefaultRedisValues(manifest.Redis)
		env, err := createEnvironmentVariables(spec, naisrequest.Deploy{}, manifest, []NaisResource{})

		assert.NoError(t, err)
		assert.Contains(t, env, v1.EnvVar{Name: "REDIS_HOST", Value: redisName})
	})
}
