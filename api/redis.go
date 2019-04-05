package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	redisapi "github.com/spotahome/redis-operator/api/redisfailover/v1alpha2"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srest "k8s.io/client-go/rest"
)

type Persistent struct {
	Enabled bool
	Storage string
}

type Redis struct {
	Enabled          bool
	HardAntiAffinity bool
	Limits           ResourceList
	Requests         ResourceList
	Persistent       Persistent
	Image            string
	Version          string
}

func createRedisFailoverDef(spec app.Spec, redis Redis) (*redisapi.RedisFailover, error) {
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
			Image:     "redis",
			Version:   "3.2-alpine",
		},
	}

	if redis.Image != "" {
		redisSpec.Redis.Image = redis.Image
	}
	if redis.Version != "" {
		redisSpec.Redis.Version = redis.Version
	}

	if redis.HardAntiAffinity {
		redisSpec.HardAntiAffinity = true
	}

	if redis.Persistent.Enabled {
		quantity, err := resource.ParseQuantity(redis.Persistent.Storage)
		if err != nil {
			return nil, err
		}

		redisSpec.Redis.Storage = redisapi.RedisStorage{
			PersistentVolumeClaim: &v1.PersistentVolumeClaim{
				ObjectMeta: k8smeta.ObjectMeta{
					Name: fmt.Sprintf("rf-%s", spec.Application),
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						v1.ReadWriteOnce,
					},
					Resources: v1.ResourceRequirements{
						Requests: map[v1.ResourceName]resource.Quantity{
							"storage": quantity,
						},
					},
				},
			},
		}
	}

	meta := generateObjectMeta(spec)
	return &redisapi.RedisFailover{Spec: redisSpec, ObjectMeta: meta}, nil
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
	newFailover, err := createRedisFailoverDef(spec, redis)
	if err != nil {
		return nil, fmt.Errorf("can't create RedisFailoverSpec: %s", err)
	}

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
