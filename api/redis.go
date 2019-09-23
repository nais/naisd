package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	k8sapps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"strconv"
)

const (
	defaultRedisPort          = 6379
	defaultRedisExporterPort  = 9121
	defaultRedisExporterImage = "oliver006/redis_exporter:v1.2.0-alpine"
	defaultRedisImage         = "redis:5-alpine"
)

type Redis struct {
	Enabled  bool
	Image    string
	Limits   ResourceList
	Requests ResourceList
}

func updateDefaultRedisValues(redis Redis) Redis {
	if redis.Image == "" {
		redis.Image = defaultRedisImage
	}
	if len(redis.Limits.Cpu) == 0 {
		redis.Limits.Cpu = "100m"
	}
	if len(redis.Limits.Memory) == 0 {
		redis.Limits.Memory = "128Mi"
	}
	if len(redis.Requests.Cpu) == 0 {
		redis.Requests.Cpu = "100m"
	}
	if len(redis.Requests.Memory) == 0 {
		redis.Requests.Memory = "128Mi"
	}
	return redis
}

func createRedisPodSpec(redis Redis) v1.PodSpec {
	return v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "redis",
				Image: redis.Image,
				Resources: createResourceLimits(redis.Requests.Cpu, redis.Requests.Memory,
					redis.Limits.Cpu, redis.Limits.Memory),
				ImagePullPolicy: v1.PullIfNotPresent,
				Ports: []v1.ContainerPort{
					{
						ContainerPort: int32(defaultRedisPort),
						Name:          DefaultPortName,
						Protocol:      v1.ProtocolTCP,
					},
				},
			},
			{
				Name:  "exporter",
				Image: defaultRedisExporterImage,
				Resources: createResourceLimits("100m", "100Mi",
					"100m", "100Mi"),
				ImagePullPolicy: v1.PullIfNotPresent,
				Ports: []v1.ContainerPort{
					{
						ContainerPort: int32(defaultRedisExporterPort),
						Name:          DefaultPortName,
						Protocol:      v1.ProtocolTCP,
					},
				},
			},
		},
	}
}

func createRedisDeploymentSpec(redisSpec app.Spec, redis Redis) k8sapps.DeploymentSpec {
	objectMeta := generateObjectMeta(redisSpec)
	objectMeta.Annotations = map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   strconv.Itoa(defaultRedisExporterPort),
		"prometheus.io/path":   "/metrics",
	}

	return k8sapps.DeploymentSpec{
		Replicas: int32p(1),
		Selector: &k8smeta.LabelSelector{
			MatchLabels: createPodSelector(redisSpec),
		},
		Strategy: k8sapps.DeploymentStrategy{
			Type: k8sapps.RecreateDeploymentStrategyType,
		},
		ProgressDeadlineSeconds: int32p(300),
		RevisionHistoryLimit:    int32p(10),
		Template: v1.PodTemplateSpec{
			ObjectMeta: objectMeta,
			Spec:       createRedisPodSpec(redis),
		},
	}
}

func createRedisDeploymentDef(redisSpec app.Spec, redis Redis, existingDeployment *k8sapps.Deployment) *k8sapps.Deployment {
	deploymentSpec := createRedisDeploymentSpec(redisSpec, redis)
	if existingDeployment != nil {
		existingDeployment.ObjectMeta = addLabelsToObjectMeta(existingDeployment.ObjectMeta, redisSpec)
		existingDeployment.Spec = deploymentSpec
		return existingDeployment
	} else {
		return &k8sapps.Deployment{
			TypeMeta: k8smeta.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: generateObjectMeta(redisSpec),
			Spec:       deploymentSpec,
		}
	}
}

func createOrUpdateRedisInstance(spec app.Spec, redis Redis, k8sClient kubernetes.Interface) (*k8sapps.Deployment, error) {
	redisSpec := app.Spec{
		Application: fmt.Sprintf("%s-redis", spec.ResourceName()),
		Namespace:   spec.Namespace,
		Team:        spec.Team,
	}
	existingDeployment, err := getExistingDeployment(redisSpec.ResourceName(), redisSpec.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing deployment: %s", err)
	}

	deploymentDef := createRedisDeploymentDef(redisSpec, redis, existingDeployment)

	return createOrUpdateDeploymentResource(deploymentDef, redisSpec.Namespace, k8sClient)
}

func createRedisServiceDef(redisSpec app.Spec) *v1.Service {
	return &v1.Service{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: generateObjectMeta(redisSpec),
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": redisSpec.ResourceName(),
			},
			Ports: []v1.ServicePort{
				{
					Name:     DefaultPortName,
					Protocol: v1.ProtocolTCP,
					Port:     6379,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: DefaultPortName,
					},
				},
			},
		},
	}
}

func createOrUpdateRedisService(spec app.Spec, k8sClient kubernetes.Interface) (*v1.Service, error) {
	redisSpec := app.Spec{
		Application: fmt.Sprintf("%s-redis", spec.ResourceName()),
		Namespace:   spec.Namespace,
		Team:        spec.Team,
	}
	service, err := getExistingService(redisSpec.ResourceName(), redisSpec.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing service: %s", err)
	} else if service == nil {
		service = createRedisServiceDef(redisSpec)
	}

	service.ObjectMeta = addLabelsToObjectMeta(service.ObjectMeta, redisSpec)
	return createOrUpdateServiceResource(service, redisSpec.Namespace, k8sClient)
}
