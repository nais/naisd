package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestRedisResource(t *testing.T) {
	t.Run("Replicas should be 1 when not prod", func(t *testing.T) {
		deploymentRequest := NaisDeploymentRequest{
			Application: "redisTest",
			FasitEnvironment: "t",
			Namespace: "default",
		}
		redisFailoverDef := createRedisFailoverDef(deploymentRequest, "teamBeam")
		assert.Equal(t, int32(1), redisFailoverDef.Spec.Redis.Replicas, "")
	})

	t.Run("Replicas should be 3 when prod", func(t *testing.T) {
		deploymentRequest := NaisDeploymentRequest{
			Application: "redisTest",
			FasitEnvironment: ENVIRONMENT_P,
			Namespace: "default",
		}
		redisFailoverDef := createRedisFailoverDef(deploymentRequest, "teamBeam")
		assert.Equal(t, int32(3), redisFailoverDef.Spec.Redis.Replicas, "")
	})
}