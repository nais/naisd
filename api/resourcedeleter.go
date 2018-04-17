package api

import (
	"fmt"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func deleteK8sResouces(namespace string, deployName string, k8sClient kubernetes.Interface) (results []string, e error) {
	var result []string

	res, err := deleteService(namespace, deployName, k8sClient)
	result = append(result, res)
	if err != nil {
		return result, err
	}

	res, err = deleteDeployment(namespace, deployName, k8sClient)
	result = append(result, res)
	if err != nil {
		return result, err
	}

	res, err = deleteRedisFailover(namespace, deployName, k8sClient)
	result = append(result, res)
	if err != nil {
		result = append(result, err.Error())
	}

	res, err = deleteSecret(namespace, deployName, k8sClient)
	result = append(result, res)
	if err != nil {
		return result, err
	}

	res, err = deleteIngress(namespace, deployName, k8sClient)
	result = append(result, res)
	if err != nil {
		return result, err
	}

	res, err = deleteAutoscaler(namespace, deployName, k8sClient)
	result = append(result, res)
	if err != nil {
		return result, err
	}

	res, err = deleteConfigMapRules(namespace, deployName, k8sClient)
	result = append(result, res)
	if err != nil {
		return result, err
	}

	return result, nil
}

func deleteService(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Services(namespace).Delete(deployName, &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound(fmt.Sprintf("service: "), err)
	}
	return "service: OK", nil
}

func deleteDeployment(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	deploymentDeleteOption := k8smeta.DeletePropagationForeground
	if err := k8sClient.ExtensionsV1beta1().Deployments(namespace).Delete(deployName, &k8smeta.DeleteOptions{PropagationPolicy: &deploymentDeleteOption}); err != nil {
		return filterNotFound("deployment: ", err)
	}
	return "deployment: ", nil
}

func deleteSecret(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Secrets(namespace).Delete(deployName, &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound("secret: ", err)
	}
	return "secret: OK", nil
}

func deleteConfigMapRules(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	configMap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, k8sClient)
	if err != nil {
		return "app alerts: FAIL", fmt.Errorf("unable to get existing configmap: %s", err)
	}

	configMap = removeRulesFromConfigMap(configMap, deployName, namespace)
	configMap, err = createOrUpdateConfigMapResource(configMap, AlertsConfigMapNamespace, k8sClient)

	if err != nil {
		return filterNotFound("app alerts: ", err)
	}
	return "alert rules: OK", nil
}

func deleteAutoscaler(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	autoscaler, err := getExistingAutoscaler(deployName, namespace, k8sClient)
	if autoscaler != nil {
		err = k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound("autoscaler: ", err)
	}

	return "autoscaler: OK", nil
}

func deleteIngress(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	ingress, err := getExistingIngress(deployName, namespace, k8sClient)
	if ingress != nil {
		err = k8sClient.ExtensionsV1beta1().Ingresses(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound("ingress: ", err)
	}

	return "ingress OK", nil
}

func deleteRedisFailover(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	svc, err := getExistingService("rfs-"+deployName, namespace, k8sClient)
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

	failoverInterface := redisclient.RedisFailoversGetter(client).RedisFailovers(namespace)
	failover, err := failoverInterface.Get(deployName, k8smeta.GetOptions{})
	if failover != nil {
		err = failoverInterface.Delete(deployName, &k8smeta.DeleteOptions{})
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
