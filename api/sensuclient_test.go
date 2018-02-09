package api

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestSensuClient(t *testing.T) {
	t.Run("Check generated deploy message", func(t *testing.T) {
		deploymentRequest := NaisDeploymentRequest{
			"TestApp",
			"42.0.0",
			"environment",
			"zone",
			"manifesturl",
			"username",
			"password",
			"onbehalfof",
			"nais",
		}
		clusterName := "nais-dev"

		message, err := GenerateDeployMessage(&deploymentRequest, &clusterName)
		assert.NoError(t, err)
		expectedMessagePrefix := "{\"name\":\"naisd.deployment\",\"type\":\"metric\",\"handlers\":[\"events_nano\"],\"output\":\"naisd.deployment,application=TestApp,clusterName=nais-dev,namespace=nais version=\\\"42.0.0\\\""
		assert.Equal(t, true, strings.HasPrefix(string(message), expectedMessagePrefix))
	})
}
