package api

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func (c clientHolder) WaitForPodReady(spec app.Spec) error {
	podInterface := c.client.CoreV1().Pods(spec.Namespace())

	for attempts := 60; attempts >= 0; attempts-- {
		pods, err := podInterface.List(v1.ListOptions{LabelSelector: fmt.Sprintf("app=%s,environment=%s", spec.Application, spec.Environment)})
		if err != nil {
			return err
		}

		for _, pod := range pods.Items {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady {
					if condition.Status == corev1.ConditionTrue {
						glog.Info("Pod ready")
						return nil
					} else {
						glog.Info("Pod not ready")
					}
				}
			}
		}

		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("pod did not become ready before timeout")
}

func (c clientHolder) DeleteOldApp(spec app.Spec, deploymentRequest naisrequest.Deploy, manifest NaisManifest) (string, error) {
	service, err := c.client.CoreV1().Services(spec.Environment).Get(spec.Application, v1.GetOptions{})

	if err == nil {
		err := c.WaitForPodReady(spec)
		if err != nil {
			return "  - aborting deletion of old app", err
		}

		_, err = c.redirectOldServiceToNewApp(service, spec)
		if err != nil {
			return "  - failed while forwarding traffic to new service. aborting deletion of old app", err
		}
	} else {
		return "", nil
	}

	if service.Spec.Type == corev1.ServiceTypeExternalName {
		return fmt.Sprintf("  - service already redirected to app-namespace, not deleting anything (this is good). App is at: %s\n", service.Spec.ExternalName), nil
	}

	// This is a "trick" to make it delete the old resources created by the old version naisd prior to app-namespaces.
	appInEnvironmentNamespace := app.Spec{
		Application: spec.Application,
		Environment: spec.Environment,
		Team:        spec.Team,
		ApplicationNamespaced: false,
	}

	result := ""
	result, _ = deleteDeployment(appInEnvironmentNamespace, c.client)
	joinedResult := fmt.Sprintln("  - " + result)

	result, _ = deleteAutoscaler(appInEnvironmentNamespace, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteConfigMapRules(appInEnvironmentNamespace, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteIngress(appInEnvironmentNamespace, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteRedisFailover(appInEnvironmentNamespace, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteSecret(appInEnvironmentNamespace, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	err = c.DeleteServiceAccount(appInEnvironmentNamespace)
	if err != nil {
		joinedResult += fmt.Sprintln("  - service account: OK")
	} else {
		joinedResult += fmt.Sprintln("  - service account: N/A")
	}

	joinedResult += "  - redirected old service to the new service in app-namespace.\n"

	return joinedResult, nil
}
