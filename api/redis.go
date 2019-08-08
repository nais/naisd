package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	v1 "k8s.io/api/core/v1"
	k8sextensions "k8s.io/api/extensions/v1beta1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultRedisPort          = 6379
	defaultRedisExporterPort  = 9121
	defaultRedisExporterImage = "oliver006/redis_exporter:v1.0.4-alpine"
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

func createRedisDeploymentSpec(resourceName string, spec app.Spec, redis Redis) k8sextensions.DeploymentSpec {
	objectMeta := generateObjectMeta(spec)
	objectMeta.Name = resourceName
	objectMeta.Annotations = map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   string(defaultRedisExporterPort),
		"prometheus.io/path":   "/metrics",
	}

	return k8sextensions.DeploymentSpec{
		Replicas: int32p(1),
		Selector: &k8smeta.LabelSelector{
			MatchLabels: createPodSelector(spec),
		},
		Strategy: k8sextensions.DeploymentStrategy{
			Type: k8sextensions.RecreateDeploymentStrategyType,
		},
		ProgressDeadlineSeconds: int32p(300),
		RevisionHistoryLimit:    int32p(10),
		Template: v1.PodTemplateSpec{
			ObjectMeta: objectMeta,
			Spec:       createRedisPodSpec(redis),
		},
	}
}

func createRedisDeploymentDef(resourceName string, spec app.Spec, redis Redis, existingDeployment *k8sextensions.Deployment) *k8sextensions.Deployment {
	deploymentSpec := createRedisDeploymentSpec(resourceName, spec, redis)
	if existingDeployment != nil {
		existingDeployment.ObjectMeta = addLabelsToObjectMeta(existingDeployment.ObjectMeta, spec)
		existingDeployment.Spec = deploymentSpec
		return existingDeployment
	} else {
		return &k8sextensions.Deployment{
			TypeMeta: k8smeta.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1beta1",
			},
			ObjectMeta: generateObjectMeta(spec),
			Spec:       deploymentSpec,
		}
	}
}

func createOrUpdateRedisInstance(spec app.Spec, redis Redis, k8sClient kubernetes.Interface) (*k8sextensions.Deployment, error) {
	redisName := fmt.Sprintf("%s-redis", spec.ResourceName())
	existingDeployment, err := getExistingDeployment(redisName, spec.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing deployment: %s", err)
	}

	deploymentDef := createRedisDeploymentDef(redisName, spec, redis, existingDeployment)
	deploymentDef.Name = fmt.Sprintf("%s-redis", spec.ResourceName())

	return createOrUpdateDeploymentResource(deploymentDef, spec.Namespace, k8sClient)
}

func createRedisServiceDef(spec app.Spec) *v1.Service {
	return &v1.Service{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: generateObjectMeta(spec),
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: createPodSelector(spec),
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
	redisName := fmt.Sprintf("%s-redis", spec.ResourceName())
	service, err := getExistingService(redisName, spec.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing service: %s", err)
	} else if service == nil {
		service = createRedisServiceDef(spec)
		service.Name = redisName
	}

	service.ObjectMeta = addLabelsToObjectMeta(service.ObjectMeta, spec)
	return createOrUpdateServiceResource(service, spec.Namespace, k8sClient)
}
