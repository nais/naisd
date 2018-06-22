package api

import (
	"github.com/golang/glog"
	"github.com/nais/naisd/api/app"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type clientHolder struct {
	client kubernetes.Interface
}

type ServiceAccountInterface interface {
	CreateIfNotExist(spec app.Spec) (*v1.ServiceAccount, error)
	Delete(spec app.Spec) error
}

func NewServiceAccountInterface(client kubernetes.Interface) ServiceAccountInterface {
	return clientHolder{
		client: client,
	}
}

func (c clientHolder) Delete(spec app.Spec) error {
	serviceAccountInterface := c.client.CoreV1().ServiceAccounts(spec.Application)

	if e := serviceAccountInterface.Delete(spec.ResourceName(), &k8smeta.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
		return e
	} else {
		return nil
	}
}

func (c clientHolder) CreateIfNotExist(spec app.Spec) (*v1.ServiceAccount, error) {
	serviceAccountInterface := c.client.CoreV1().ServiceAccounts(spec.Application)

	if account, err := serviceAccountInterface.Get(spec.ResourceName(), k8smeta.GetOptions{}); err == nil {
		glog.Infof("Skipping service account creation. All ready exist for application: %s in namespace: %s", spec.ResourceName(), spec.Namespace())
		return account, nil
	}

	return serviceAccountInterface.Create(createServiceAccountDef(spec))
}

func createServiceAccountDef(spec app.Spec) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: generateObjectMeta(spec),
	}
}
