package naisresource

import (
	"fmt"
	k8sapps "k8s.io/api/apps/v1"
	k8score "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

func CreateDeploymentDef(meta k8smeta.ObjectMeta) *k8sapps.Deployment {
	return &k8sapps.Deployment{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: meta,
	}
}

func CreateDeploymentSpec(meta k8smeta.ObjectMeta, annotations map[string]string, spec k8score.PodSpec) k8sapps.DeploymentSpec {
	return k8sapps.DeploymentSpec{
		Replicas: int32p(1),
		Strategy: k8sapps.DeploymentStrategy{
			Type: k8sapps.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &k8sapps.RollingUpdateDeployment{
				MaxUnavailable: &intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: int32(0),
				},
				MaxSurge: &intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: int32(1),
				},
			},
		},
		ProgressDeadlineSeconds: int32p(300),
		RevisionHistoryLimit:    int32p(10),
		Template: k8score.PodTemplateSpec{
			ObjectMeta: annotateObjectMeta(meta, annotations),
			Spec:       spec,
		},
	}
}

func GetExistingDeployment(application string, namespace string, k8sClient kubernetes.Interface) (*k8sapps.Deployment, error) {
	deploymentClient := k8sClient.AppsV1().Deployments(namespace)
	deployment, err := deploymentClient.Get(application, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return deployment, err
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func CreateContainerSpec(meta k8smeta.ObjectMeta, taggedImage string, port int, liveness, readiness k8score.Probe, lifecycle k8score.Lifecycle, resourceLimits k8score.ResourceRequirements, envVars []k8score.EnvVar, volumeMounts []k8score.VolumeMount) k8score.Container {
	return k8score.Container{
		Name:  meta.Name,
		Image: taggedImage,
		Ports: []k8score.ContainerPort{
			{ContainerPort: int32(port), Protocol: k8score.ProtocolTCP, Name: DefaultPortName},
		},
		Resources:       resourceLimits,
		LivenessProbe:   &liveness,
		ReadinessProbe:  &readiness,
		Env:             envVars,
		ImagePullPolicy: k8score.PullIfNotPresent,
		Lifecycle:       &lifecycle,
		VolumeMounts:    volumeMounts,
	}
}

func CreatePodSpec(container k8score.Container, sidecars []k8score.Container, volumes []k8score.Volume) k8score.PodSpec {
	return k8score.PodSpec{
		Containers:    append([]k8score.Container{container}, sidecars...),
		RestartPolicy: k8score.RestartPolicyAlways,
		DNSPolicy:     k8score.DNSClusterFirst,
		Volumes:       volumes,
	}
}

func CreateLeaderElectionContainer(appName string) k8score.Container {
	return k8score.Container{
		Name:            "elector",
		Image:           "gcr.io/google_containers/leader-elector:0.5",
		ImagePullPolicy: k8score.PullIfNotPresent,
		Resources: k8score.ResourceRequirements{
			Requests: k8score.ResourceList{
				k8score.ResourceCPU: k8sresource.MustParse("100m"),
			},
		},
		Ports: []k8score.ContainerPort{
			{ContainerPort: 4040, Protocol: k8score.ProtocolTCP},
		},
		Args: []string{"--election=" + appName, "--http=localhost:4040", "--election-namespace=election"},
	}
}

func CreateLifeCycle(path string) k8score.Lifecycle {
	if len(path) > 0 {
		return k8score.Lifecycle{
			PreStop: &k8score.Handler{
				HTTPGet: &k8score.HTTPGetAction{
					Path: path,
					Port: intstr.FromString(DefaultPortName),
				},
			},
		}
	}

	return k8score.Lifecycle{}
}

func CreateResourceLimits(requestsCpu string, requestsMemory string, limitsCpu string, limitsMemory string) k8score.ResourceRequirements {
	return k8score.ResourceRequirements{
		Requests: k8score.ResourceList{
			k8score.ResourceCPU:    k8sresource.MustParse(requestsCpu),
			k8score.ResourceMemory: k8sresource.MustParse(requestsMemory),
		},
		Limits: k8score.ResourceList{
			k8score.ResourceCPU:    k8sresource.MustParse(limitsCpu),
			k8score.ResourceMemory: k8sresource.MustParse(limitsMemory),
		},
	}
}

func CreateProbe(path string, initialDelay, timeout, period, failureThreshold int32) k8score.Probe {
	return k8score.Probe{
		Handler: k8score.Handler{
			HTTPGet: &k8score.HTTPGetAction{
				Path: path,
				Port: intstr.FromString(DefaultPortName),
			},
		},
		InitialDelaySeconds: initialDelay,
		TimeoutSeconds:      timeout,
		PeriodSeconds:       period,
		FailureThreshold:    failureThreshold,
	}
}

func CreateEnvVar(name, value string) k8score.EnvVar {
	return k8score.EnvVar{
		Name:  name,
		Value: value,
	}
}
