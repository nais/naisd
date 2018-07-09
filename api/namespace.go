package api

import (
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c clientHolder) waitForNamespaceReady(namespace *corev1.Namespace) error {
	var err error
	namespaceInterface := c.client.CoreV1().Namespaces()
	glog.Infof("Waiting for namespace '%s' to become ready.", namespace.Name)

	for {
		_, err := namespaceInterface.Get(namespace.Name, k8smeta.GetOptions{})
		if !errors.IsNotFound(err) {
			break
		}
		glog.Infof("Namespace '%s' still not ready, sleeping for 1 second.", namespace.Name)
		time.Sleep(time.Second)
	}

	return err
}

func (c clientHolder) createNamespace(name, team string) (*corev1.Namespace, error) {
	namespaceInterface := c.client.CoreV1().Namespaces()

	namespace, err := namespaceInterface.Get(name, k8smeta.GetOptions{IncludeUninitialized: false})
	if err == nil && namespace != nil {
		glog.Infof("Namespace %s already exists. Aborting.", name)
		return namespace, err
	} else if errors.IsNotFound(err) {
		glog.Infof("Creating namespace %s.", name)
		return namespaceInterface.Create(createNamespaceDef(name, team))
	} else {
		glog.Errorf("Failed while getting existing namespace: unexpected error: %s", err)
		return nil, nil
	}
}

func createNamespaceDef(name, team string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: k8smeta.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"team": team,
			},
		},
	}
}
