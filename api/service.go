package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	"k8s.io/api/core/v1"
)

func (c clientHolder) redirectOldServiceToNewApp(originalService *v1.Service, spec app.Spec) (*v1.Service, error) {
	serviceInterface := c.client.CoreV1().Services(originalService.Namespace)

	externalNameService := &v1.Service{
		ObjectMeta: *originalService.ObjectMeta.DeepCopy(),
		Spec: v1.ServiceSpec{
			Type:         v1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc.nais.local", spec.ResourceName(), spec.Namespace()),
			Ports:        []v1.ServicePort{{Port: 80}},
		},
	}

	return serviceInterface.Update(externalNameService)
}
