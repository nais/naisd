package api

import (
	"k8s.io/client-go/kubernetes"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
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

	err = deleteRedisFailover(namespace, deployName, deploymentDeleteOption, k8sClient)
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
		return fmt.Errorf("did not find config map: %s", err)
	}

	configMap, err = removeRulesFromConfigMap(configMap, deployName, namespace)
	if err != nil {
		return fmt.Errorf("could not remove rules from config map: %s", err)
	}

	configMap, err = createOrUpdateConfigMapResource(configMap, alertsConfigMapNamespace, k8sClient)

	if err != nil {
		return fmt.Errorf("could not update config map: %s", err)
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

	return err
}

func deleteRedisFailover(namespace string, deployName string, deleteOption k8smeta.DeletionPropagation, k8sClient kubernetes.Interface) error {
	deploymentPrefix := "rfs-"
	deployment, err := getExistingDeployment(deploymentPrefix + deployName, namespace, k8sClient)
	if deployment != nil {
		err = k8sClient.ExtensionsV1beta1().Deployments(namespace).Delete(deploymentPrefix + deployName, &k8smeta.DeleteOptions{
			PropagationPolicy: &deleteOption,
		})
	}

	if err != nil {
		return fmt.Errorf("could not delete redis failover for %s in namespace %s: %s", deployName, namespace, err)
	}

	return nil
}