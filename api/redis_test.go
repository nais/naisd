package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRedisResource(t *testing.T) {
	t.Run("Replicas should always be 3", func(t *testing.T) {
		spec := app.Spec{Application: appName, Environment: environment, Team: "teamBeam"}
		redisFailoverDef := createRedisFailoverDef(spec)
		assert.Equal(t, int32(3), redisFailoverDef.Spec.Redis.Replicas, "")
	})
}
