package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestSensuClient(t *testing.T) {
	t.Run("Check generated deploy message", func(t *testing.T) {
		spec := app.Spec{
			Application: "TestApp",
			Namespace:   "nais",
			Team:        "team",
		}
		deploymentRequest := naisrequest.Deploy{
			Application:      spec.Application,
			Version:          "42.0.0",
			FasitEnvironment: "fasitEnvironment",
			Zone:             "zone",
			ManifestUrl:      "manifesturl",
			FasitUsername:    "username",
			FasitPassword:    "password",
			OnBehalfOf:       "onbehalfof",
			Namespace:        spec.Namespace,
		}
		clusterName := "nais-dev"

		message, err := GenerateDeployMessage(spec, &deploymentRequest, &clusterName)
		assert.NoError(t, err)
		expectedMessagePrefix := "{\"name\":\"naisd.deployment\",\"type\":\"metric\",\"handlers\":[\"events_nano\"],\"output\":\"naisd.deployment,application=TestApp,clusterName=nais-dev,namespace=nais version=\\\"42.0.0\\\""
		assert.Equal(t, true, strings.HasPrefix(string(message), expectedMessagePrefix))
	})
}
