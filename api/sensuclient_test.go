package api

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestSensuClient(t *testing.T) {
	t.Run("Check generated deploy message", func(t *testing.T) {
		application := "TestApp"
		clusterName := "nais-dev"
		namespace := "nais"
		version := "42.0.0"

		message, err := GenerateDeployMessage(&application, &clusterName, &namespace, &version)
		assert.NoError(t, err)
		expected_message_prefix := "{\"name\":\"naisd.deployment\",\"type\":\"metric\",\"handlers\":[\"events_nano\"],\"output\":\"naisd.deployment,application=TestApp,clusterName=nais-dev,namespace=nais version=\\\"42.0.0\\\""
		assert.Equal(t, true, strings.HasPrefix(string(message), expected_message_prefix))
	})
}
