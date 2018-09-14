package api

import (
	"github.com/nais/naisd/api/app"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	"testing"
)

func TestValidatePrometheusAlertRules(t *testing.T) {
	invalidManifest := NaisManifest{
		Alerts: []PrometheusAlertRule{{Alert: "Name"}},
	}

	invalidManifestNoAction := NaisManifest{
		Alerts: []PrometheusAlertRule{
			{
				Alert: "Name",
				For:   "For",
				Expr:  "Expression",
			},
		},
	}

	validManifest := NaisManifest{
		Alerts: []PrometheusAlertRule{
			{
				Alert: "Name",
				Expr:  "Expression",
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
				For:   "For",
				Expr:  "Expression",
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
	assert.Equal(t, "Expr must be specified", err.ErrorMessage)

	err2 := validateAlertRules(invalidManifestNoAction)
	assert.Equal(t, "An annotation named action must be specified", err2.ErrorMessage)

	noErr := validateAlertRules(validManifest)
	assert.Nil(t, noErr)

	noErr2 := validateAlertRules(validManifestWithLabels)
	assert.Nil(t, noErr2)
}

func TestAddRulesToConfigMap(t *testing.T) {
	spec := app.Spec{Application: appName, Namespace: namespace, Team: teamName}
	deploymentPrefix := createDeploymentPrefix(spec)
	rulesGroupFilename := deploymentPrefix + ".yml"

	alertRule := PrometheusAlertRule{
		Alert: "alertName",
		For:   "5m",
		Expr:  "expr",
		Annotations: map[string]string{
			"action": "alertAction",
		},
	}

	configMap := &v1.ConfigMap{
		ObjectMeta: generateObjectMeta(spec),
		Data: map[string]string{
			"asd-environment-otherAppName.yml": "not touched",
		},
	}

	manifest := NaisManifest{
		Team:   teamName,
		Alerts: []PrometheusAlertRule{alertRule},
	}

	resultingConfigMap, err := addRulesToConfigMap(spec, configMap, manifest)

	resultingAlertGroups := PrometheusAlertGroups{}
	err = yaml.Unmarshal([]byte(resultingConfigMap.Data[rulesGroupFilename]), &resultingAlertGroups)
	resultingAlertGroup := resultingAlertGroups.Groups[0]
	resultingAlertRule := resultingAlertGroup.Rules[0]

	assert.Nil(t, err)
	assert.Equal(t, prefixAlertName(deploymentPrefix, alertRule.Alert), resultingAlertRule.Alert)
	assert.Equal(t, alertRule.Expr, resultingAlertRule.Expr)
	assert.Equal(t, alertRule.For, resultingAlertRule.For)
	assert.Equal(t, alertRule.Annotations["action"], resultingAlertRule.Annotations["action"])
	assert.Equal(t, teamName, resultingAlertRule.Labels["team"])

	assert.Equal(t, deploymentPrefix, resultingAlertGroup.Name)
	assert.Len(t, resultingAlertGroup.Rules, 1)
	assert.Len(t, resultingAlertGroups.Groups, 1)

	assert.Equal(t, "not touched", resultingConfigMap.Data["asd-environment-otherAppName.yml"])
	assert.Len(t, resultingConfigMap.Data, 2)
}

func TestAddTeamLabel(t *testing.T) {
	alerts := []PrometheusAlertRule{
		{
			Alert: "Name",
			For:   "For",
			Expr:  "Expression",
			Annotations: map[string]string{
				"action": "action",
			},
		},
	}

	addTeamLabel(alerts, teamName)

	assert.NotNil(t, alerts[0], "Alerts should not be nil.")
	assert.NotNil(t, alerts[0].Labels, "addTeamLabel should have added the Labels map to the alert.")
	assert.NotNil(t, alerts[0].Labels["team"], "addTeamLabel should have added the 'team' key to the map.")

	assert.Equal(t, teamName, alerts[0].Labels["team"])
}

func TestPrefixAlertNames(t *testing.T) {
	prefix := "prefixed"
	alert1 := "alert1"
	alert2 := "alert2"

	alerts := []PrometheusAlertRule{
		{
			Alert: alert1,
			For:   "For",
			Expr:  "Expression",
			Annotations: map[string]string{
				"action": "action",
			},
		},
		{
			Alert: alert2,
			For:   "For",
			Expr:  "Expression",
			Annotations: map[string]string{
				"action": "action",
			},
		},
	}

	prefixAlertNames(alerts, prefix)

	assert.NotNil(t, alerts[0], "Alerts should not be nil.")

	assert.Equal(t, prefixAlertName(prefix, alert1), alerts[0].Alert)
	assert.Equal(t, prefixAlertName(prefix, alert2), alerts[1].Alert)
}

func TestNamespaceSubstitution(t *testing.T) {
	alerts := []PrometheusAlertRule{
		{
			Alert: "alert1",
			For:   "For",
			Expr:  "up{kubernetes_namespace=\"$namespace\"} > 0",
			Annotations: map[string]string{
				"action": "action",
			},
		},
	}

	assert.NotNil(t, alerts[0], "Alerts should not be nil.")

	substituteNamespaceVariables(alerts,"q1")
	assert.Equal(t, alerts[0].Expr, "up{kubernetes_namespace=\"q1\"} > 0")
}
