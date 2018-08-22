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
		redisFailoverDef := createRedisFailoverDef(spec)
		assert.Equal(t, int32(3), redisFailoverDef.Spec.Redis.Replicas, "")
	})

	t.Run("REDIS_HOST env var should be set when redis: true", func(t *testing.T) {
		spec := app.Spec{Application: appName, Namespace: namespace, Team: "teamBeam"}
		manifest := NaisManifest{Redis: true}
		env, err := createEnvironmentVariables(spec, naisrequest.Deploy{}, manifest, []NaisResource{})

		assert.NoError(t, err)
		assert.Contains(t, env, v1.EnvVar{Name: "REDIS_HOST", Value: "rfs-" + spec.ResourceName()})
	})
}
