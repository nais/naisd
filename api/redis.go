package api

import (
	"fmt"
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

func redisSentinelClusterExist(failovers []redisapi.RedisFailover, appName string) bool {
	for _, v := range failovers {
		if v.Name == appName {
			return true
		}
	}
	return false
}

func createRedisSentinelCluster(deploymentRequest NaisDeploymentRequest, team string) (*redisapi.RedisFailover, error) {
	failover := createRedisFailoverDef(deploymentRequest, team)

	config, err := k8srest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can't create InClusterConfig: %s", err)
	}

	client, err := redisclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create new Redis client for InClusterConfig: %s", err)
	}

	rfs, err := client.RedisFailovers(deploymentRequest.Namespace).List(k8smeta.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed getting list of redis failovers: %s", err)
	}

	if redisSentinelClusterExist(rfs.Items, deploymentRequest.Application) {
		return nil, nil // redis failover is running, nothing to do
	}

	return redisclient.RedisFailoversGetter(client).RedisFailovers(deploymentRequest.Namespace).Create(failover)
}
