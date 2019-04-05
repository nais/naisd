package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"testing"
)

func TestRedisResource(t *testing.T) {
	t.Run("Replicas should always be 3", func(t *testing.T) {
		spec := app.Spec{Application: appName, Namespace: namespace, Team: "teamBeam"}
		manifest := NaisManifest{Redis: Redis{Enabled: true}}
		redisFailoverDef, err := createRedisFailoverDef(spec, manifest.Redis)
		assert.NoError(t, err)
		assert.Equal(t, int32(3), redisFailoverDef.Spec.Redis.Replicas)
	})

	t.Run("Custom resources", func(t *testing.T) {
		spec := app.Spec{Application: appName, Team: "teamBeam"}
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
		redisFailoverDef, err := createRedisFailoverDef(spec, manifest.Redis)
		assert.NoError(t, err)
		assert.Equal(t, "1337m", redisFailoverDef.Spec.Redis.Resources.Limits.CPU)
		assert.Equal(t, "128Mi", redisFailoverDef.Spec.Redis.Resources.Limits.Memory)
		assert.Equal(t, "100m", redisFailoverDef.Spec.Redis.Resources.Requests.CPU)
		assert.Equal(t, "512Mi", redisFailoverDef.Spec.Redis.Resources.Requests.Memory)
	})

	t.Run("REDIS_HOST env var should be set when redis: true", func(t *testing.T) {
		spec := app.Spec{Application: appName, Namespace: namespace, Team: "teamBeam"}
		manifest := NaisManifest{Redis: Redis{Enabled: true}}
		env, err := createEnvironmentVariables(spec, naisrequest.Deploy{}, manifest, []NaisResource{})

		assert.NoError(t, err)
		assert.Contains(t, env, v1.EnvVar{Name: "REDIS_HOST", Value: "rfs-" + spec.ResourceName()})
	})
}
