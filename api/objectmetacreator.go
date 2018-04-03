package api

import (
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createObjectMeta(applicationName, namespace, teamName string) k8smeta.ObjectMeta {
	labels := map[string]string{"app": applicationName,}

	if teamName != "" {
		labels["team"] = teamName
	}

	return k8smeta.ObjectMeta{
		Name:      applicationName,
		Namespace: namespace,
		Labels: labels,
	}
}
