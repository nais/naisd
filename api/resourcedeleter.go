package api

import (
	"k8s.io/client-go/kubernetes"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	"k8s.io/client-go/rest"
	"k8s.io/apimachinery/pkg/api/errors"
)

func deleteK8sResouces(namespace string, deployName string, k8sClient kubernetes.Interface) (results []string, e error) {
	var result []string

	if res, err := deleteService(namespace, deployName, k8sClient); res != "" {
		result = append(result, res)
		if err != nil {
			return result, err
		}
	}

	if res, err := deleteDeployment(namespace, deployName, k8sClient); res != "" {
		result = append(result, res)
		if err != nil {
			return result, err
		}
	}

	if err := deleteRedisFailover(namespace, deployName, k8sClient); err != nil {
		result = append(result, err.Error())
	}

	if res, err := deleteSecret(namespace, deployName, k8sClient); res != "" {
		result = append(result, res)
		if err != nil {
			return result, err
		}
	}

	if res, err := deleteIngress(namespace, deployName, k8sClient); res != "" {
		result = append(result, res)
		if err != nil {
			return result, err
		}
	}

	if res, err := deleteAutoscaler(namespace, deployName, k8sClient); res != "" {
		result = append(result, res)
		if err != nil {
			return result, err
		}
	}

	if res, err := deleteConfigMapRules(namespace, deployName, k8sClient); res != "" {
		result = append(result, res)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func deleteService(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Services(namespace).Delete(deployName, &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound(fmt.Sprintf("could not delete service: %s in namespace: %s: %s", deployName, namespace, err), err)
	}
	return "", nil
}

func deleteDeployment(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	deploymentDeleteOption := k8smeta.DeletePropagationForeground
	if err := k8sClient.ExtensionsV1beta1().Deployments(namespace).Delete(deployName, &k8smeta.DeleteOptions{ PropagationPolicy: &deploymentDeleteOption, }); err != nil {
		return filterNotFound(fmt.Sprintf("could not delete deployment: %s in namespace: %s: %s", deployName, namespace, err), err)
	}
	return "", nil
}

func deleteSecret(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Secrets(namespace).Delete(deployName, &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound(fmt.Sprintf("could not delete secret for: %s in namespace: %s: %s", deployName, namespace, err), err)
	}
	return "", nil
}

func deleteConfigMapRules(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	configMap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, k8sClient)
	if err != nil {
		return fmt.Sprintf("unable to get existing configmap: %s", err), err
	}

	configMap = removeRulesFromConfigMap(configMap, deployName, namespace)
	configMap, err = createOrUpdateConfigMapResource(configMap, AlertsConfigMapNamespace, k8sClient)

	if err != nil {
		return filterNotFound(fmt.Sprintf("failed to remove alert rules from configmap: %s", err), err)
	}

	return "", nil
}

func deleteAutoscaler(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	autoscaler, err := getExistingAutoscaler(deployName, namespace, k8sClient)
	if autoscaler != nil {
		err = k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound(fmt.Sprintf("could not delete autoscaler for %s in namespace %s: %s", deployName, namespace, err), err)
	}

	return "", nil
}

func deleteIngress(namespace string, deployName string, k8sClient kubernetes.Interface) (result string, e error) {
	ingress, err := getExistingIngress(deployName, namespace, k8sClient)
	if ingress != nil {
		err = k8sClient.ExtensionsV1beta1().Ingresses(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound(fmt.Sprintf("could not delete ingress for %s: %s", deployName, err), err)
	}

	return "", nil
}

func deleteRedisFailover(namespace string, deployName string, k8sClient kubernetes.Interface) error {
	svc, err := getExistingService("rfs-" + deployName, namespace, k8sClient)
	if svc == nil {
		return nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("can't create InClusterConfig: %s", err)
	}

	client, err := redisclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("can't create new Redis client for InClusterConfig: %s", err)
	}

	failoverInterface := redisclient.RedisFailoversGetter(client).RedisFailovers(namespace)
	failover, err := failoverInterface.Get(deployName, k8smeta.GetOptions{})
	if  failover != nil {
		err = failoverInterface.Delete(deployName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return fmt.Errorf("failed while deleting Redis sentinel cluster: %s", err)
	}

	return nil
}

func filterNotFound(res string, err error) (result string, e error) {
	if errors.IsNotFound(err) {
		return res, nil
	}

	return res, err
}
