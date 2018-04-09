package api

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestSensuClient(t *testing.T) {
	t.Run("Check generated deploy message", func(t *testing.T) {
		deploymentRequest := NaisDeploymentRequest{
			Application: "TestApp",
			Version: "42.0.0",
			FasitEnvironment: "environment",
			Zone: "zone",
			ManifestUrl: "manifesturl",
			FasitUsername: "username",
			FasitPassword: "password",
			OnBehalfOf: "onbehalfof",
			Namespace: "nais",
		}
		clusterName := "nais-dev"

		message, err := GenerateDeployMessage(&deploymentRequest, &clusterName)
		assert.NoError(t, err)
		expectedMessagePrefix := "{\"name\":\"naisd.deployment\",\"type\":\"metric\",\"handlers\":[\"events_nano\"],\"output\":\"naisd.deployment,application=TestApp,clusterName=nais-dev,namespace=nais version=\\\"42.0.0\\\""
		assert.Equal(t, true, strings.HasPrefix(string(message), expectedMessagePrefix))
	})
}
