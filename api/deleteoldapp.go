package api

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	oldApp := app.Spec{
		Application: spec.Application,
		Environment: spec.Environment,
		Team:        spec.Team,
		ApplicationNamespaced: false,
	}

	service, err := c.client.CoreV1().Services(oldApp.Namespace()).Get(oldApp.ResourceName(), v1.GetOptions{})

	if err == nil {
		err := c.WaitForPodReady(spec)
		if err != nil {
			return "  - aborting deletion of old app", err
		}

		_, err = c.redirectOldServiceToNewApp(service, spec)
		if err != nil {
			return "  - failed while forwarding traffic to new service. aborting deletion of old app", err
		}
	} else if errors.IsNotFound(err) {
		// No old service exists.
		return "", nil
	} else {
		return "failed while fetching existing service", err
	}

	if service.Spec.Type == corev1.ServiceTypeExternalName {
		return fmt.Sprintf("  - service already redirected to app-namespace, not deleting anything (this is good). App is at: %s\n", service.Spec.ExternalName), nil
	}

	result := ""
	result, _ = deleteDeployment(oldApp, c.client)
	joinedResult := fmt.Sprintln("  - " + result)

	result, _ = deleteAutoscaler(oldApp, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteConfigMapRules(oldApp, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteIngress(oldApp, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteRedisFailover(oldApp, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	result, _ = deleteSecret(oldApp, c.client)
	joinedResult += fmt.Sprintln("  - " + result)

	err = c.DeleteServiceAccount(oldApp)
	if err == nil {
		joinedResult += fmt.Sprintln("  - service account: OK")
	} else {
		joinedResult += fmt.Sprintln("  - service account: N/A")
	}

	err = c.deleteRoleBinding(oldApp)
	if err == nil {
		joinedResult += fmt.Sprintln("  - role binding: OK")
	} else {
		joinedResult += fmt.Sprintln("  - role binding: N/A")
	}

	joinedResult += "  - redirected old service to the new service in app-namespace.\n"

	return joinedResult, nil
}
