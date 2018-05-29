package api

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/nais/naisd/api/naisrequest"
	"gopkg.in/yaml.v2"
	k8score "k8s.io/api/core/v1"
)

type PrometheusAlertGroups struct {
	Groups []PrometheusAlertGroup
}

type PrometheusAlertGroup struct {
	Name  string
	Rules []PrometheusAlertRule
}

type PrometheusAlertRule struct {
	Alert       string
	Expr        string
	For         string
	Labels      map[string]string
	Annotations map[string]string
}

func prefixAlertName(prefix, alertName string) string {
	return fmt.Sprintf("%s_%s", prefix, alertName)
}

func prefixAlertNames(alertRules []PrometheusAlertRule, prefix string) {
	for i := range alertRules {
		alertRules[i].Alert = prefixAlertName(prefix, alertRules[i].Alert)
	}

	return
}

func addTeamLabel(alertRules []PrometheusAlertRule, teamName string) {
	if teamName != "" {
		for i := range alertRules {
			if alertRules[i].Labels == nil {
				alertRules[i].Labels = make(map[string]string)
			}

			alertRules[i].Labels["team"] = teamName
		}
	}

	return
}

func createDeploymentPrefix(namespace string, deployName string) string {
	return namespace + "-" + deployName
}

func addRulesToConfigMap(configMap *k8score.ConfigMap, deploymentRequest naisrequest.Deploy, manifest NaisManifest) (*k8score.ConfigMap, error) {
	deploymentPrefix := createDeploymentPrefix(deploymentRequest.Namespace, deploymentRequest.Application)

	addTeamLabel(manifest.Alerts, manifest.Team)
	prefixAlertNames(manifest.Alerts, deploymentPrefix)

	alertGroup := PrometheusAlertGroup{Name: deploymentPrefix, Rules: manifest.Alerts}
	alertGroups := PrometheusAlertGroups{Groups: []PrometheusAlertGroup{alertGroup}}

	alertGroupYamlBytes, err := yaml.Marshal(alertGroups)
	if err != nil {
		glog.Errorf("Failed to marshal %v to yaml\n", alertGroup)
		return nil, err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	configMap.Data[deploymentPrefix+".yml"] = string(alertGroupYamlBytes)

	return configMap, nil
}

func removeRulesFromConfigMap(configMap *k8score.ConfigMap, deployName string, namespace string) *k8score.ConfigMap {
	if configMap.Data == nil {
		return configMap
	}

	ruleGroupName := createDeploymentPrefix(namespace, deployName)
	delete(configMap.Data, ruleGroupName+".yml")

	return configMap
}

func validateAlertRules(manifest NaisManifest) *ValidationError {
	for _, alertRule := range manifest.Alerts {
		if alertRule.Alert == "" {
			return &ValidationError{
				"Alert must be specified",
				map[string]string{"Alert": alertRule.Alert},
			}
		}
		if alertRule.Expr == "" {
			return &ValidationError{
				"Expr must be specified",
				map[string]string{"Expr": alertRule.Expr},
			}
		}
		if action, exists := alertRule.Annotations["action"]; !exists {
			return &ValidationError{
				"An annotation named action must be specified",
				map[string]string{"annotations[action]": action},
			}
		}
	}

	return nil
}
