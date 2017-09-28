package api

import (
	"testing"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/api/v1"
)


func TestIsDeploymentFinished(t *testing.T) {

	var wantedReplicas int32 = 2
	var deploymentGeneration int64 = 2

	deployment := &v1beta1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:       "appname",
			Namespace:  "default",
			Generation: deploymentGeneration,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32p(wantedReplicas),
		},
	}


	t.Run("Test for when a deployment status ", func(t *testing.T) {
		tests := []struct {
			testDescription string
			deployStatus    v1beta1.DeploymentStatus
			expectedStatus  DeployStatus
		}{
			{
				testDescription: "Deploy is in progress when observed generation is less than spec generation.",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration - 1,
				},
				expectedStatus: InProgress,
			},
			{
				testDescription: "Deploy is in progress when updated replicas are less than wanted replicas",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration,
					UpdatedReplicas:    wantedReplicas - 1,
					AvailableReplicas:  wantedReplicas,
				},
				expectedStatus: InProgress,
			},
			{
				testDescription: "Deploy is in progress when there are more replicas  than wanted replicas",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration,
					UpdatedReplicas:    wantedReplicas,
					AvailableReplicas:  wantedReplicas,
					Replicas:           wantedReplicas + 1,
				},
				expectedStatus: InProgress,
			},
			{

				testDescription: "Deploy is in progress when there are less available replicas than wanted replicas",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration,
					UpdatedReplicas:    wantedReplicas,
					AvailableReplicas:  wantedReplicas - 1,
					Replicas:           wantedReplicas,
				},
				expectedStatus: InProgress,
			},
			{

				testDescription: "Deploy is finished when the number of replicas, available, updated and wanted replicas are equal",
				deployStatus: v1beta1.DeploymentStatus{
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

			actualStatus,_:= deploymentStatusAndView(*deployment)
			if test.expectedStatus != actualStatus {
				t.Errorf("Failed test: %s\n DeploymentStatus: %+v", test.testDescription, test.deployStatus)
			}
		}

	})

}
