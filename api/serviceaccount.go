package api

import (
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type clientHolder struct {
	client kubernetes.Interface
}

type ServiceAccountInterface interface {
	CreateOrUpdate(name, environment, team string) (*v1.ServiceAccount, error)
	Delete(name, environment, team string) error
}

func NewServiceAccountInterface(client kubernetes.Interface) ServiceAccountInterface {
	return clientHolder{
		client: client,
	}
}

func (c clientHolder) Delete(name, environment, namespace string) error {
	if e := c.client.CoreV1().ServiceAccounts(namespace).Delete(createObjectName(name, environment), &k8smeta.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
		return e
	} else {
		return nil
	}
}

func (c clientHolder) CreateOrUpdate(name, environment, team string) (*v1.ServiceAccount, error) {
	serviceAccountInterface := c.client.CoreV1().ServiceAccounts(team)

	objectName := createObjectName(name, environment)
	account, e := serviceAccountInterface.Get(objectName, k8smeta.GetOptions{})
	if e != nil && !errors.IsNotFound(e) {
		return nil, fmt.Errorf("unexpected error: %s", e)
	}

	serviceAccountDef := createServiceAccountDef(name, environment, team)

	if account != nil && account.ResourceVersion != "" {
		return serviceAccountInterface.Update(serviceAccountDef)
	} else {
		return serviceAccountInterface.Create(serviceAccountDef)
	}
}

func createServiceAccountDef(applicationName string, environment string, team string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: generateObjectMeta(applicationName, environment, team),
	}
}
