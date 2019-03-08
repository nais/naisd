package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	redisapi "github.com/spotahome/redis-operator/api/redisfailover/v1alpha2"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srest "k8s.io/client-go/rest"
)

// Redis yaml-object to enable and set resources
type Redis struct {
	Enabled          bool
	HardAntiAffinity bool
	Limits           ResourceList
	Requests         ResourceList
}

func createRedisFailoverDef(spec app.Spec, redis Redis) *redisapi.RedisFailover {
	replicas := int32(3)
	resources := redisapi.RedisFailoverResources{
		Limits: redisapi.CPUAndMem{
			CPU:    "100m",
			Memory: "128Mi",
		},
		Requests: redisapi.CPUAndMem{
			CPU:    "100m",
			Memory: "128Mi",
		},
	}

	if len(redis.Limits.Cpu) != 0 {
		resources.Limits.CPU = redis.Limits.Cpu
	}
	if len(redis.Limits.Memory) != 0 {
		resources.Limits.Memory = redis.Limits.Memory
	}
	if len(redis.Requests.Cpu) != 0 {
		resources.Requests.CPU = redis.Requests.Cpu
	}
	if len(redis.Requests.Memory) != 0 {
		resources.Requests.Memory = redis.Requests.Memory
	}

	redisSpec := redisapi.RedisFailoverSpec{
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

	if redis.HardAntiAffinity {
		redisSpec.HardAntiAffinity = true
	}

	meta := generateObjectMeta(spec)
	return &redisapi.RedisFailover{Spec: redisSpec, ObjectMeta: meta}
}

func getExistingFailover(failoverInterface redisclient.RedisFailoverInterface, resourceName string) (*redisapi.RedisFailover, error) {
	failover, err := failoverInterface.Get(resourceName, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return failover, err
	case k8serrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func updateOrCreateRedisSentinelCluster(spec app.Spec, redis Redis) (*redisapi.RedisFailover, error) {
	newFailover := createRedisFailoverDef(spec, redis)

	config, err := k8srest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can't create InClusterConfig: %s", err)
	}

	client, err := redisclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create new Redis client for InClusterConfig: %s", err)
	}

	existingFailover, err := getExistingFailover(redisclient.RedisFailoversGetter(client).RedisFailovers(spec.Namespace), spec.ResourceName())
	if err != nil {
		return nil, fmt.Errorf("unable to get existing redis failover: %s", err)
	}

	if existingFailover != nil {
		existingFailover.Spec = newFailover.Spec
		existingFailover.ObjectMeta = mergeObjectMeta(existingFailover.ObjectMeta, newFailover.ObjectMeta)
		return redisclient.RedisFailoversGetter(client).RedisFailovers(spec.Namespace).Update(existingFailover)
	}

	return redisclient.RedisFailoversGetter(client).RedisFailovers(spec.Namespace).Create(newFailover)
}
