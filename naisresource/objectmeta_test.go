package naisresource

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateObjectMeta(t *testing.T) {
	t.Run("Test required metadata field values", func(t *testing.T) {
		objectMeta := CreateObjectMeta(appName, namespace, teamName)

		assert.Equal(t, teamName, objectMeta.Labels["team"], "Team label should be equal to team name.")
		assert.Equal(t, appName, objectMeta.Labels["app"], "App label should be equal to app name.")
		assert.Equal(t, appName, objectMeta.Name, "Resource name should equal app name.")
		assert.Equal(t, namespace, objectMeta.Namespace, "Resource namespace should equal namespace.")
	})

	t.Run("Test team label not set when not specified", func(t *testing.T) {
		objectMetaWithoutTeamName := CreateObjectMeta(appName, namespace, "")

		_, ok := objectMetaWithoutTeamName.Labels["team"]
		assert.False(t, ok, "Team label should not be set when team name is empty.")
	})
}

func TestAnnotateObjectMeta(t *testing.T) {
	objectMeta := CreateObjectMeta(appName, namespace, teamName)

	t.Run("Test annotating ObjectMeta without annotations", func(t *testing.T) {
		key := "key"
		value := "value"

		annotations := map[string]string{key: value}

		annotatedObjectMeta := annotateObjectMeta(objectMeta, annotations)

		assert.NotNil(t, annotatedObjectMeta.Annotations, "Annotations should have been initialized by annotate")
		assert.Equal(t, annotatedObjectMeta.Annotations[key], value)
	})

	t.Run("Test annotating ObjectMeta with annotations", func(t *testing.T) {
		firstKey := "firstKey"
		firstValue := "firstValue"
		secondKey := "secondKey"
		secondValue := "secondValue"

		firstAnnotations := map[string]string{firstKey: firstValue}
		secondAnnotations := map[string]string{secondKey: secondValue}

		firstAnnotatedObjectMeta := annotateObjectMeta(objectMeta, firstAnnotations)
		secondAnnotatedObjectMeta := annotateObjectMeta(firstAnnotatedObjectMeta, secondAnnotations)

		assert.Equal(t, secondAnnotatedObjectMeta.Annotations[firstKey], firstValue)
		assert.Equal(t, secondAnnotatedObjectMeta.Annotations[secondKey], secondValue)
	})
}
