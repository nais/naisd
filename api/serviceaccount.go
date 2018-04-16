package api

import (
	"k8s.io/client-go/kubernetes"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type serviceAcccountClient struct {
	client kubernetes.Interface
}

type ServiceAccountInterface interface {
	CreateOrUpdate(name, namespace, team string) (*v1.ServiceAccount, error)
	Delete(name, namespace string) error
}

func NewServiceAccountInterface(client kubernetes.Interface) ServiceAccountInterface {
	return serviceAcccountClient{
		client: client,
	}
}

func (c serviceAcccountClient) Delete(name, namespace string) error {
	if e := c.client.CoreV1().ServiceAccounts(namespace).Delete(name, &k8smeta.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
		return e
	} else {
		return nil
	}

}

func (c serviceAcccountClient) CreateOrUpdate(name, namespace, team string) (*v1.ServiceAccount, error) {
	serviceAccountInterface := c.client.CoreV1().ServiceAccounts(namespace)

	account, e := serviceAccountInterface.Get(name, k8smeta.GetOptions{})
	if e != nil && !errors.IsNotFound(e) {
		return nil, fmt.Errorf("unexpected error: %s", e)
	}

	serviceAccountDef := createServiceAccountDef(name, namespace, team)

	if account != nil {
		return serviceAccountInterface.Update(serviceAccountDef)
	} else {
		return serviceAccountInterface.Create(serviceAccountDef)
	}
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
