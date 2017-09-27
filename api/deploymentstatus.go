package api

import (
	"net/http"
	"net/url"
	"k8s.io/client-go/kubernetes"
	"path"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"fmt"
)

func deploymentStatusHandler(clientset kubernetes.Interface) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		namespace, appName := parseURLPath(r.URL)
		if dep, err := clientset.ExtensionsV1beta1().Deployments(namespace).Get(appName); err != nil {
			_, deployFinished, err := isDeploymentFinished(dep)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if deployFinished {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusAccepted)
			}

		}

	})

}
func isDeploymentFinished(deployment *v1beta1.Deployment) (string, bool, error) {
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		if deploymentExceededProgressDeadline(deployment.Status) {
			return "", false, fmt.Errorf("deployment %q exceeded its progress deadline", deployment.Name)
		}
		if deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas have been updated.", deployment.Status.UpdatedReplicas, deployment.Spec.Replicas), false, nil
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for rollout to finish: %d old replicas are pending termination.", deployment.Status.Replicas-deployment.Status.UpdatedReplicas), false, nil
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for rollout to finish: %d of %d updated replicas are available.", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas), false, nil
		}
		return fmt.Sprintf("deployment %q successfully rolled out.", deployment.Name), true, nil
	}
	return fmt.Sprintf("Waiting for deployment spec update to be observed."), false, nil
}

func deploymentExceededProgressDeadline(status v1beta1.DeploymentStatus) bool {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == v1beta1.DeploymentProgressing && c.Reason == "ProgressDeadlineExceeded"{
			return true
		}
	}
	return false
}

func parseURLPath(url *url.URL) (namespace string, appName string) {
	dir, file := path.Split(url.Path)
	return path.Base(dir), file
}
