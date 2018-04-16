package api

import (
	"github.com/nais/naisd/api/constant"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRedisResource(t *testing.T) {
	t.Run("Replicas should be 1 when not prod", func(t *testing.T) {
		deploymentRequest := naisrequest.Deploy{
			Application:      "redisTest",
			FasitEnvironment: "t",
			Namespace:        "default",
		}
		redisFailoverDef := createRedisFailoverDef(deploymentRequest, "teamBeam")
		assert.Equal(t, int32(1), redisFailoverDef.Spec.Redis.Replicas, "")
	})

	t.Run("Replicas should be 3 when prod", func(t *testing.T) {
		deploymentRequest := naisrequest.Deploy{
			Application:      "redisTest",
			FasitEnvironment: constant.ENVIRONMENT_P,
			Namespace:        "default",
		}
		redisFailoverDef := createRedisFailoverDef(deploymentRequest, "teamBeam")
		assert.Equal(t, int32(3), redisFailoverDef.Spec.Redis.Replicas, "")
	})
}
