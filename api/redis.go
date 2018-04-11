package api

import (
	"fmt"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srest "k8s.io/client-go/rest"
	redisapi "github.com/spotahome/redis-operator/api/redisfailover/v1alpha2"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
)

func createRedisFailoverDef(deploymentRequest NaisDeploymentRequest, team string) *redisapi.RedisFailover {
	replicas := int32(3)
	resources := redisapi.RedisFailoverResources{
		Limits:   redisapi.CPUAndMem{Memory: "100Mi"},
		Requests: redisapi.CPUAndMem{CPU: "100m"},
	}
	if deploymentRequest.FasitEnvironment != ENVIRONMENT_P {
		replicas = int32(1)
		resources = redisapi.RedisFailoverResources{
			Limits:   redisapi.CPUAndMem{Memory: "50Mi"},
			Requests: redisapi.CPUAndMem{CPU: "50m"},
		}
	}

	spec := redisapi.RedisFailoverSpec{
		HardAntiAffinity: false,
		Sentinel: redisapi.SentinelSettings{
			Replicas:  replicas,
			Resources: resources,
		},
		Redis: redisapi.RedisSettings{
			Replicas:  replicas,
			Resources: resources,
			Exporter:  true,
		},
	}

	meta := createObjectMeta(deploymentRequest.Application, deploymentRequest.Namespace, team)
	return &redisapi.RedisFailover{Spec: spec, ObjectMeta: meta}
}

func getExistingFailover(failoverInterface redisclient.RedisFailoverInterface, appName string) (*redisapi.RedisFailover, error) {
	failover, err := failoverInterface.Get(appName, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return failover, err
	case k8serrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func updateOrCreateRedisSentinelCluster(deploymentRequest NaisDeploymentRequest, team string) (*redisapi.RedisFailover, error) {
	newFailover := createRedisFailoverDef(deploymentRequest, team)

	config, err := k8srest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can't create InClusterConfig: %s", err)
	}

	client, err := redisclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create new Redis client for InClusterConfig: %s", err)
	}

	existingFailover, err := getExistingFailover(redisclient.RedisFailoversGetter(client).RedisFailovers(deploymentRequest.Namespace), deploymentRequest.Application)
	if err != nil {
		return nil, fmt.Errorf("unable to get existing redis failover: %s", err)
	}

	if existingFailover != nil {
		existingFailover.Spec = newFailover.Spec
		existingFailover.ObjectMeta = mergeObjectMeta(existingFailover.ObjectMeta, newFailover.ObjectMeta)
		return redisclient.RedisFailoversGetter(client).RedisFailovers(deploymentRequest.Namespace).Update(existingFailover)
	}

	return redisclient.RedisFailoversGetter(client).RedisFailovers(deploymentRequest.Namespace).Create(newFailover)
}
