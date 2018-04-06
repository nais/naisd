package naisresource

import (
	"fmt"
	k8sautoscaling "k8s.io/api/autoscaling/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

func CreateAutoscalerDef(meta k8smeta.ObjectMeta) *k8sautoscaling.HorizontalPodAutoscaler {
	return &k8sautoscaling.HorizontalPodAutoscaler{
		TypeMeta: k8smeta.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v1",
		},
		ObjectMeta: meta,
	}
}

func CreateAutoscalerSpec(min, max, cpuTargetPercentage int, application string) k8sautoscaling.HorizontalPodAutoscalerSpec {
	return k8sautoscaling.HorizontalPodAutoscalerSpec{
		MinReplicas:                    int32p(int32(min)),
		MaxReplicas:                    int32(max),
		TargetCPUUtilizationPercentage: int32p(int32(cpuTargetPercentage)),
		ScaleTargetRef: k8sautoscaling.CrossVersionObjectReference{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
			Name:       application,
		},
	}
}

func createOrUpdateAutoscalerResource(autoscalerSpec *k8sautoscaling.HorizontalPodAutoscaler, namespace string, k8sClient k8s.Interface) (*k8sautoscaling.HorizontalPodAutoscaler, error) {
	if autoscalerSpec.ObjectMeta.ResourceVersion != "" {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(autoscalerSpec)
	} else {
		return k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(autoscalerSpec)
	}
}

func CreateOrUpdateAutoscaler(meta k8smeta.ObjectMeta, minReplicas, maxReplicas, cpuThresholdPercentage int, k8sClient k8s.Interface) (*k8sautoscaling.HorizontalPodAutoscaler, error) {
	autoscaler, err := getExistingAutoscaler(meta.Name, meta.Namespace, k8sClient)

	if err != nil {
		return nil, fmt.Errorf("unable to get existing autoscaler: %s", err)
	}

	if autoscaler == nil {
		autoscaler = CreateAutoscalerDef(meta)
	}

	autoscaler.Spec = CreateAutoscalerSpec(minReplicas, maxReplicas, cpuThresholdPercentage, meta.Name)

	return createOrUpdateAutoscalerResource(autoscaler, meta.Namespace, k8sClient)
}

func getExistingAutoscaler(application string, namespace string, k8sClient k8s.Interface) (*k8sautoscaling.HorizontalPodAutoscaler, error) {
	autoscalerClient := k8sClient.AutoscalingV1().HorizontalPodAutoscalers(namespace)
	autoscaler, err := autoscalerClient.Get(application, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return autoscaler, err
	case k8serrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}
