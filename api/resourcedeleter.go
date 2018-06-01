package api

import (
	"fmt"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func deleteK8sResouces(application, environment, team string, k8sClient kubernetes.Interface) (results []string, e error) {
	res, err := deleteService(application, environment, team, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteDeployment(application, environment, team, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteRedisFailover(application, environment, team, k8sClient)
	results = append(results, res)
	if err != nil {
		results = append(results, err.Error())
	}

	res, err = deleteSecret(application, environment, team, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteIngress(application, environment, team, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteAutoscaler(application, environment, team, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	res, err = deleteConfigMapRules(application, environment, team, k8sClient)
	results = append(results, res)
	if err != nil {
		return results, err
	}

	if err := NewServiceAccountInterface(k8sClient).Delete(application, environment, team); err != nil {
		return results, err
	} else {
		results = append(results, "service account: OK")
	}
	return results, nil
}

func deleteService(application, environment, team string, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Services(team).Delete(createObjectName(application, environment), &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound(fmt.Sprintf("service: "), err)
	}
	return "service: OK", nil
}

func deleteDeployment(application, environment, team string, k8sClient kubernetes.Interface) (result string, e error) {
	deploymentDeleteOption := k8smeta.DeletePropagationForeground
	if err := k8sClient.ExtensionsV1beta1().Deployments(team).Delete(createObjectName(application, environment), &k8smeta.DeleteOptions{PropagationPolicy: &deploymentDeleteOption}); err != nil {
		return filterNotFound("deployment: ", err)
	}
	return "deployment: OK", nil
}

func deleteSecret(application, environment, team string, k8sClient kubernetes.Interface) (result string, e error) {
	if err := k8sClient.CoreV1().Secrets(team).Delete(createObjectName(application, environment), &k8smeta.DeleteOptions{}); err != nil {
		return filterNotFound("secret: ", err)
	}
	return "secret: OK", nil
}

func deleteConfigMapRules(application, environment, team string, k8sClient kubernetes.Interface) (result string, e error) {
	configMap, err := getExistingConfigMap(AlertsConfigMapName, AlertsConfigMapNamespace, k8sClient)
	if err != nil {
		return "app alerts: FAIL", fmt.Errorf("unable to get existing configmap: %s", err)
	}

	configMap = removeRulesFromConfigMap(configMap, application, environment, team)
	configMap, err = createOrUpdateConfigMapResource(configMap, AlertsConfigMapNamespace, k8sClient)

	if err != nil {
		return filterNotFound("app alerts: ", err)
	}
	return "alert rules: OK", nil
}

func deleteAutoscaler(application, environment, team string, k8sClient kubernetes.Interface) (result string, e error) {
	objectName := createObjectName(application, environment)
	autoscaler, err := getExistingAutoscaler(objectName, team, k8sClient)
	if autoscaler != nil {
		err = k8sClient.AutoscalingV1().HorizontalPodAutoscalers(team).Delete(objectName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound("autoscaler: ", err)
	}

	return "autoscaler: OK", nil
}

func deleteIngress(application, environment, team string, k8sClient kubernetes.Interface) (result string, e error) {
	objectName := createObjectName(application, environment)
	ingress, err := getExistingIngress(objectName, team, k8sClient)
	if ingress != nil {
		err = k8sClient.ExtensionsV1beta1().Ingresses(team).Delete(objectName, &k8smeta.DeleteOptions{})
	}

	if err != nil {
		return filterNotFound("ingress: ", err)
	}

	return "ingress OK", nil
}

func deleteRedisFailover(application, environment, team string, k8sClient kubernetes.Interface) (result string, e error) {
	objectName := createObjectName(application, environment)
	svc, err := getExistingService("rfs-"+objectName, team, k8sClient)
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

	failoverInterface := redisclient.RedisFailoversGetter(client).RedisFailovers(team)
	failover, err := failoverInterface.Get(objectName, k8smeta.GetOptions{})
	if failover != nil {
		err = failoverInterface.Delete(objectName, &k8smeta.DeleteOptions{})
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
