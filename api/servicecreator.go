package api

import (
	"k8s.io/client-go/kubernetes"
	"fmt"
	k8score "k8s.io/api/core/v1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/api/errors"
)

func createService(deploymentRequest NaisDeploymentRequest, manifest NaisManifest, k8sClient kubernetes.Interface) (*k8score.Service, error) {
	existingService, err := getExistingService(deploymentRequest.Application, deploymentRequest.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing service: %s", err)
	}

	if existingService != nil {
		return nil, nil // we have done nothing
	}

	objectMeta := createObjectMeta(deploymentRequest.Application, deploymentRequest.Namespace, manifest.Team)
	serviceDef := createServiceDef(objectMeta)

	return createServiceResource(serviceDef, k8sClient)
}

func createServiceDef(meta k8smeta.ObjectMeta) *k8score.Service {
	return &k8score.Service{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: meta,
		Spec: k8score.ServiceSpec{
			Type:     k8score.ServiceTypeClusterIP,
			Selector: map[string]string{"app": meta.Name},
			Ports: []k8score.ServicePort{
				{
					Name:     "http",
					Protocol: k8score.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: DefaultPortName,
					},
				},
			},
		},
	}
}

func getExistingService(application string, namespace string, k8sClient kubernetes.Interface) (*k8score.Service, error) {
	serviceClient := k8sClient.CoreV1().Services(namespace)
	service, err := serviceClient.Get(application, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return service, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func createServiceResource(serviceSpec *k8score.Service, k8sClient kubernetes.Interface) (*k8score.Service, error) {
	return k8sClient.CoreV1().Services(serviceSpec.Namespace).Create(serviceSpec)
}
