package naisresource

import (
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateObjectMeta(applicationName, namespace, teamName string) k8smeta.ObjectMeta {
	labels := map[string]string{"app": applicationName}

	if teamName != "" {
		labels["team"] = teamName
	}

	return k8smeta.ObjectMeta{
		Name:      applicationName,
		Namespace: namespace,
		Labels:    labels,
	}
}

func annotateObjectMeta(meta k8smeta.ObjectMeta, annotations map[string]string) k8smeta.ObjectMeta {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}

	for k, v := range annotations {
		meta.Annotations[k] = v
	}

	return meta
}
