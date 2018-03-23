package api

import (
	k8score "k8s.io/api/core/v1"
	"gopkg.in/yaml.v2"
	"github.com/golang/glog"
)

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

func addRulesToConfigMap(configMap *k8score.ConfigMap, deploymentRequest NaisDeploymentRequest, manifest NaisManifest) (*k8score.ConfigMap, error) {
	ruleGroupName := deploymentRequest.Namespace + deploymentRequest.Application
	alertGroup := PrometheusAlertGroup{Name: ruleGroupName, Rules: manifest.Alerts}
	alertGroupYamlBytes, err := yaml.Marshal(alertGroup)
	if err != nil {
		glog.Errorf("Failed to marshal %v to yaml\n", alertGroup)
		return nil, err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	configMap.Data[ruleGroupName + ".yml"] = string(alertGroupYamlBytes)

	return configMap, nil
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
		if alertRule.For == "" {
			return &ValidationError{
				"For must be specified",
				map[string]string{"For": alertRule.For},
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
