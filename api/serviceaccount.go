package api

import (
	"k8s.io/client-go/kubernetes"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"github.com/golang/glog"
)

type clientHolder struct {
	client kubernetes.Interface
}

type ServiceAccountInterface interface {
	CreateIfNotExist(name, namespace, team string) (*v1.ServiceAccount, error)
	Delete(name, namespace string) error
}

func NewServiceAccountInterface(client kubernetes.Interface) ServiceAccountInterface {
	return clientHolder{
		client: client,
	}
}

func (c clientHolder) Delete(name, namespace string) error {
	if e := c.client.CoreV1().ServiceAccounts(namespace).Delete(name, &k8smeta.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
		return e
	} else {
		return nil
	}

}

func (c clientHolder) CreateIfNotExist(name, namespace, team string) (*v1.ServiceAccount, error) {
	serviceAccountInterface := c.client.CoreV1().ServiceAccounts(namespace)

	if account, err := serviceAccountInterface.Get(name, k8smeta.GetOptions{}); err ==  nil {
		glog.Infof("Skipping service account creation. All ready exist for application: %s in namespace: %s", name, namespace)
		return account, nil

	}
	return serviceAccountInterface.Create(createServiceAccountDef(name, namespace, team))
}

func createServiceAccountDef(applicationName string, namespace string, team string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: createObjectMeta(applicationName, namespace, team),
	}
}
