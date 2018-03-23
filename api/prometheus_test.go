package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestValidatePrometheusAlertRules(t *testing.T) {
	invalidManifest := NaisManifest{
		Alerts: []PrometheusAlertRule{{Alert: "Name"}},
	}

	invalidManifestNoAction := NaisManifest{
		Alerts: []PrometheusAlertRule{
			{
				Alert: "Name",
				For: "For",
				Expr: "Expression",
			},
		},
	}

	validManifest := NaisManifest{
		Alerts: []PrometheusAlertRule{
			{
				Alert: "Name",
				For: "For",
				Expr: "Expression",
				Annotations: map[string]string{
					"action": "action",
				},
			},
		},
	}

	validManifestWithLabels := NaisManifest{
		Alerts: []PrometheusAlertRule{
			{
				Alert: "Name",
				For: "For",
				Expr: "Expression",
				Annotations: map[string]string{
					"action": "action",
				},
				Labels: map[string]string{
					"label": "label",
				},
			},
		},
	}

	err := validateAlertRules(invalidManifest)
	assert.Equal(t,"Expr must be specified", err.ErrorMessage)

	err2 := validateAlertRules(invalidManifestNoAction)
	assert.Equal(t,"An annotation named action must be specified", err2.ErrorMessage)

	noErr := validateAlertRules(validManifest)
	assert.Nil(t, noErr)

	noErr2 := validateAlertRules(validManifestWithLabels)
	assert.Nil(t, noErr2)
}
