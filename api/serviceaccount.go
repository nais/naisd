package api

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"github.com/golang/glog"
)

type clientHolder struct {
	client kubernetes.Interface
}

type ServiceAccountInterface interface {
	CreateIfNotExist(name, environment, team string) (*v1.ServiceAccount, error)
	Delete(name, environment, team string) error
}

func NewServiceAccountInterface(client kubernetes.Interface) ServiceAccountInterface {
	return clientHolder{
		client: client,
	}
}

func (c clientHolder) Delete(name, environment, team string) error {
	if e := c.client.CoreV1().ServiceAccounts(team).Delete(createObjectName(name, environment), &k8smeta.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
		return e
	} else {
		return nil
	}

}

func (c clientHolder) CreateIfNotExist(name, environment, team string) (*v1.ServiceAccount, error) {
	serviceAccountInterface := c.client.CoreV1().ServiceAccounts(team)

	objectName := createObjectName(name, environment)
	if account, err := serviceAccountInterface.Get(objectName, k8smeta.GetOptions{}); err ==  nil {
		glog.Infof("Skipping service account creation. All ready exist for application: %s in namespace: %s", name, team)
		return account, nil
	}

	return serviceAccountInterface.Create(createServiceAccountDef(name, environment, team))
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
