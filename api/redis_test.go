package api

import (
	"github.com/nais/naisd/api/constant"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"testing"
)

func TestRedisResource(t *testing.T) {
	t.Run("Replicas should always be 3", func(t *testing.T) {
		deploymentRequest := naisrequest.Deploy{
			Application:      "redisTest",
			FasitEnvironment: constant.ENVIRONMENT_P,
			Namespace:        "default",
		}
		redisFailoverDef := createRedisFailoverDef(deploymentRequest, "teamBeam")
		assert.Equal(t, int32(3), redisFailoverDef.Spec.Redis.Replicas, "")
	})

	t.Run("REDIS_HOST env var should be sed when redis: true", func(t *testing.T) {
		deploymentRequest := naisrequest.Deploy{
			Application:      "redisTest",
			FasitEnvironment: constant.ENVIRONMENT_P,
			Namespace:        "default",
		}

		manifest := NaisManifest{Redis: true}
		env, err := createEnvironmentVariables(deploymentRequest, manifest, []NaisResource{})

		assert.NoError(t, err)
		assert.Contains(t, env, v1.EnvVar{Name: "REDIS_HOST", Value: "rfs-" + deploymentRequest.Application})
	})
}
