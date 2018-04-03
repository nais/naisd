package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCreateObjectMeta(t *testing.T) {
	t.Run("Test required metadata field values", func(t *testing.T) {
		objectMeta := CreateObjectMeta(appName, namespace, teamName)
		objectMetaWithoutTeamName := CreateObjectMeta(appName, namespace, "")

		assert.Equal(t, teamName, objectMeta.Labels["team"], "Team label should be equal to team name.")
		assert.Equal(t, appName, objectMeta.Labels["app"], "App label should be equal to app name.")
		assert.Equal(t, appName, objectMeta.Name, "Resource name should equal app name.")
		assert.Equal(t, namespace, objectMeta.Namespace, "Resource namespace should equal namespace.")

		_, ok := objectMetaWithoutTeamName.Labels["team"]
		assert.False(t, ok, "Team label should not be set when team name is empty.")
	})
}
