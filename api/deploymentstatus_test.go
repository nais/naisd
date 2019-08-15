package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	k8score "k8s.io/api/core/v1"
	k8sapps "k8s.io/api/apps/v1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsDeploymentStatus(t *testing.T) {

	var wantedReplicas int32 = 2
	var deploymentGeneration int64 = 2

	deployment := &k8sapps.Deployment{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:       "appname",
			Namespace:  "default",
			Generation: deploymentGeneration,
		},
		Spec: k8sapps.DeploymentSpec{
			Replicas: int32p(wantedReplicas),
		},
	}

	tests := []struct {
		testDescription string
		deployStatus    k8sapps.DeploymentStatus
		expectedStatus  DeployStatus
	}{
		{
			testDescription: "Deploy is in progress when observed generation is less than spec generation.",
			deployStatus: k8sapps.DeploymentStatus{
				ObservedGeneration: deploymentGeneration - 1,
			},
			expectedStatus: InProgress,
		},
		{
			testDescription: "Deploy is in progress when updated replicas are less than wanted replicas",
			deployStatus: k8sapps.DeploymentStatus{
				ObservedGeneration: deploymentGeneration,
				UpdatedReplicas:    wantedReplicas - 1,
				AvailableReplicas:  wantedReplicas,
			},
			expectedStatus: InProgress,
		},
		{
			testDescription: "Deploy is in progress when there are more replicas  than wanted replicas",
			deployStatus: k8sapps.DeploymentStatus{
				ObservedGeneration: deploymentGeneration,
				UpdatedReplicas:    wantedReplicas,
				AvailableReplicas:  wantedReplicas,
				Replicas:           wantedReplicas + 1,
			},
			expectedStatus: InProgress,
		},
		{

			testDescription: "Deploy is in progress when there are less available replicas than wanted replicas",
			deployStatus: k8sapps.DeploymentStatus{
				ObservedGeneration: deploymentGeneration,
				UpdatedReplicas:    wantedReplicas,
				AvailableReplicas:  wantedReplicas - 1,
				Replicas:           wantedReplicas,
			},
			expectedStatus: InProgress,
		},
		{

			testDescription: "Deploy is finished when the number of replicas, available, updated and wanted replicas are equal",
			deployStatus: k8sapps.DeploymentStatus{
				ObservedGeneration: deploymentGeneration,
				UpdatedReplicas:    wantedReplicas,
				AvailableReplicas:  wantedReplicas,
				Replicas:           wantedReplicas,
			},
			expectedStatus: Success,
		},
	}

	for _, test := range tests {
		deployment.Status = test.deployStatus

		actualStatus, _ := deploymentStatusAndView(*deployment)
		if test.expectedStatus != actualStatus {
			t.Errorf("Failed test: %s\n DeploymentStatus: %+v", test.testDescription, test.deployStatus)
		}
	}
}

func TestDeploymentStatusViewFrom(t *testing.T) {
	deployment := k8sapps.Deployment{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:      "appname",
			Namespace: "default",
		},
		Spec: k8sapps.DeploymentSpec{
			Replicas: int32p(4),
			Template: k8score.PodTemplateSpec{
				Spec: k8score.PodSpec{
					Containers: []k8score.Container{
						{
							Name:  "container",
							Image: "docker.io/nginx",
						},
					},
				},
			},
		},

		Status: k8sapps.DeploymentStatus{
			Replicas:          3,
			UpdatedReplicas:   2,
			AvailableReplicas: 2,
		},
	}
	view := deploymentStatusViewFrom(Success, "reason", deployment)

	assert.Equal(t, view.Status, Success.String())
	assert.Equal(t, view.Reason, "reason")
	assert.Equal(t, view.Name, deployment.Name)
	assert.Equal(t, view.Containers, []string{deployment.Spec.Template.Spec.Containers[0].Name})
	assert.Equal(t, view.Images, []string{deployment.Spec.Template.Spec.Containers[0].Image})
	assert.Equal(t, view.Available, deployment.Status.AvailableReplicas)
	assert.Equal(t, view.UpToDate, deployment.Status.UpdatedReplicas)
	assert.Equal(t, view.Current, deployment.Status.Replicas)
	assert.Equal(t, view.Desired, *deployment.Spec.Replicas)

}

func TestDeploymentExceededProgressDeadline(t *testing.T) {

	t.Run("True if a condition is progress dead line exceeded", func(t *testing.T) {
		assert.True(t, deploymentExceededProgressDeadline(k8sapps.DeploymentStatus{
			Conditions: []k8sapps.DeploymentCondition{
				{
					Type:   k8sapps.DeploymentProgressing,
					Reason: "ProgressDeadlineExceeded",
				},
			},
		}))
	})

	t.Run("False if no condition is progress dead line exceeded", func(t *testing.T) {
		assert.False(t, deploymentExceededProgressDeadline(k8sapps.DeploymentStatus{
			Conditions: []k8sapps.DeploymentCondition{
				{
					Type:   k8sapps.DeploymentProgressing,
					Reason: "Other reason",
				},
			},
		}))
	})

}
