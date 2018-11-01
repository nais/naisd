package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func deleteK8sResouces(spec app.Spec, k8sClient kubernetes.Interface) (results []string, e error) {
	res, err := deleteService(spec, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteDeployment(spec, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteRedisFailover(spec, k8sClient)
	results = append(results, res)
	if err != nil {
		results = append(results, err.Error())
	}

	res, err = deleteSecret(spec, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteIngress(spec, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteAutoscaler(spec, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteConfigMapRules(spec, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	if err := NewServiceAccountInterface(k8sClient).DeleteServiceAccount(spec); err != nil {
		return results, err
	} else {
		results = append(results, "service account: OK")
	}

	return results, nil
}

func deleteService(spec app.Spec, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Services(spec.Namespace).Delete(spec.ResourceName(), &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound(fmt.Sprintf("service: "), err)
	}
	return "service: OK", nil
}

func deleteDeployment(spec app.Spec, k8sClient kubernetes.Interface) (result string, e error) {
	deploymentDeleteOption := k8smeta.DeletePropagationForeground
	if err := k8sClient.ExtensionsV1beta1().Deployments(spec.Namespace).Delete(spec.ResourceName(), &k8smeta.DeleteOptions{PropagationPolicy: &deploymentDeleteOption}); err != nil {
		return filterNotFound("deployment: ", err)
	}
	return "deployment: OK", nil
}

func deleteSecret(spec app.Spec, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Secrets(spec.Namespace).Delete(spec.ResourceName(), &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound("secret: ", err)
	}
	return "secret: OK", nil
}

func deleteConfigMapRules(spec app.Spec, k8sClient kubernetes.Interface) (result string, e error) {
	configMap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, k8sClient)
	if err != nil {
		return "app alerts: FAIL", fmt.Errorf("unable to get existing configmap: %s", err)
	}

	configMap = removeRulesFromConfigMap(configMap, spec)
	configMap, err = createOrUpdateConfigMapResource(configMap, AlertsConfigMapNamespace, k8sClient)

	if err != nil {
		return filterNotFound("app alerts: ", err)
	}
	return "alert rules: OK", nil
}

func deleteAutoscaler(spec app.Spec, k8sClient kubernetes.Interface) (result string, e error) {
	autoscaler, err := getExistingAutoscaler(spec, k8sClient)
	if autoscaler != nil {
		err = k8sClient.AutoscalingV1().HorizontalPodAutoscalers(spec.Namespace).Delete(spec.ResourceName(), &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound("autoscaler: ", err)
	}

	return "autoscaler: OK", nil
}

func deleteIngress(spec app.Spec, k8sClient kubernetes.Interface) (result string, e error) {
	ingress, err := getExistingIngress(spec, k8sClient)
	if ingress != nil {
		err = k8sClient.ExtensionsV1beta1().Ingresses(spec.Namespace).Delete(spec.ResourceName(), &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound("ingress: ", err)
	}

	return "ingress OK", nil
}

func deleteRedisFailover(spec app.Spec, k8sClient kubernetes.Interface) (result string, e error) {

	svc, err := getExistingService(spec, k8sClient)
	if svc == nil {
		return "redis: N/A", nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return "redis: FAIL", fmt.Errorf("failed while deleting redis failover: can't create InClusterConfig: %s", err)
	}

	client, err := redisclient.NewForConfig(config)
	if err != nil {
		return "redis: FAIL", fmt.Errorf("failed while deleting redis failover: can't create new Redis client for InClusterConfig: %s", err)
	}

	failoverInterface := redisclient.RedisFailoversGetter(client).RedisFailovers(spec.Namespace)
	failover, err := failoverInterface.Get(spec.ResourceName(), k8smeta.GetOptions{})
	if failover != nil {
		err = failoverInterface.Delete(spec.ResourceName(), &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return "redis: FAIL", fmt.Errorf("failed while deleting Redis failover: %s", err)
	}

	return "redis: OK", nil
}

func filterNotFound(resultMessage string, err error) (result string, e error) {
	if errors.IsNotFound(err) {
		return resultMessage + "N/A", nil
	}

	return resultMessage + "FAIL", err
}
