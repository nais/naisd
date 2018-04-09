package api

import (
	"k8s.io/client-go/kubernetes"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	"k8s.io/client-go/rest"
)


func deleteK8sResouces(namespace string, deployName string, k8sClient kubernetes.Interface) error {
	deploymentDeleteOption := k8smeta.DeletePropagationForeground

	err := k8sClient.CoreV1().Services(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("did not find service: %s in namespace: %s: %s", deployName, namespace, err)
	}

	err = k8sClient.ExtensionsV1beta1().Deployments(namespace).Delete(deployName, &k8smeta.DeleteOptions{
		PropagationPolicy: &deploymentDeleteOption,
	})
	if err != nil {
		return fmt.Errorf("did not find deployment: %s in namespace: %s: %s", deployName, namespace, err)
	}

	err = k8sClient.CoreV1().Secrets(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("did not find secret for: %s in namespace: %s: %s", deployName, namespace, err)
	}

	err = deleteRedisFailover(namespace, deployName, k8sClient)
	if err != nil {
		return err
	}

	err = deleteIngress(namespace, deployName, k8sClient)
	if err != nil {
		return err
	}

	err = deleteAutoscaler(namespace, deployName, k8sClient)
	if err != nil {
		return err
	}

	err = deleteConfigMapRules(namespace, deployName, k8sClient)
	if err != nil {
		return err
	}

	return nil
}
func deleteConfigMapRules(namespace string, deployName string, k8sClient kubernetes.Interface) error {
	configMap, err := getExistingConfigMap(alertsConfigMapName, alertsConfigMapNamespace, k8sClient)
	if configMap == nil {
		return fmt.Errorf("unable to get  existing configmap: %s", err)
	}

	configMap = removeRulesFromConfigMap(configMap, deployName, namespace)
	configMap, err = createOrUpdateConfigMapResource(configMap, alertsConfigMapNamespace, k8sClient)

	if err != nil {
		return fmt.Errorf("failed to remove alert rules from configmap: %s", err)
	}

	return nil
}

func deleteAutoscaler(namespace string, deployName string, k8sClient kubernetes.Interface) error {
	autoscaler, err := getExistingAutoscaler(deployName, namespace, k8sClient)
	if autoscaler != nil {
		err = k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return fmt.Errorf("could not delete autoscaler for %s in namespace %s: %s", deployName, namespace, err)
	}

	return nil
}

func deleteIngress(namespace string, deployName string, k8sClient kubernetes.Interface) error {
	ingress, err := getExistingIngress(deployName, namespace, k8sClient)
	if ingress != nil {
		err = k8sClient.ExtensionsV1beta1().Ingresses(namespace).Delete(deployName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return fmt.Errorf("could not delete ingress for %s: %s", deployName, err)
	}

	return nil
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
