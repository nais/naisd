package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"net/url"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/api/v1"
)

func TestAppnNameFromUrl(t *testing.T) {

	u, _ := url.Parse("http://test.url/namespace/appname")
	namespace, appName := parseURLPath(u)

	assert.Equal(t, "appname", appName)
	assert.Equal(t, "namespace", namespace)
}

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

	t.Run("Error when progress dead line is exceeded", func(t *testing.T) {

		deployment.Status = v1beta1.DeploymentStatus{
			ObservedGeneration: deploymentGeneration,
			Conditions: []v1beta1.DeploymentCondition{
				{
					Type:   v1beta1.DeploymentProgressing,
					Reason: "ProgressDeadlineExceeded",
				},
			},
		}

		_, isFinished, err := isDeploymentFinished(deployment)

		assert.Error(t, err)
		assert.Equal(t, false, isFinished)
	})

	t.Run("Test for when a deployment is finished", func(t *testing.T) {
		tests := []struct {
			testDescription string
			deployStatus    v1beta1.DeploymentStatus
			expectedStatus  bool
		}{
			{
				testDescription: "Deploy is not finished when observed generation is less than spec generation.",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration - 1,
				},
				expectedStatus: false,
			},
			{
				testDescription: "Deploy is not finished when updated replicas are less than wanted replicas",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration,
					UpdatedReplicas:    wantedReplicas - 1,
					AvailableReplicas:  wantedReplicas,
				},
				expectedStatus: false,
			},
			{
				testDescription: "Deploy is not finished when there are more replicas  than wanted replicas",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration,
					UpdatedReplicas:    wantedReplicas,
					AvailableReplicas:  wantedReplicas,
					Replicas:           wantedReplicas + 1,
				},
				expectedStatus: false,
			},
			{

				testDescription: "Deploy is not finished when there are less available replicas than wanted replicas",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration,
					UpdatedReplicas:    wantedReplicas,
					AvailableReplicas:  wantedReplicas - 1,
					Replicas:           wantedReplicas,
				},
				expectedStatus: false,
			},
			{

				testDescription: "Deploy is finished when the number of replicas, available, updated and wanted replicas are equal",
				deployStatus: v1beta1.DeploymentStatus{
					ObservedGeneration: deploymentGeneration,
					UpdatedReplicas:    wantedReplicas,
					AvailableReplicas:  wantedReplicas,
					Replicas:           wantedReplicas,
				},
				expectedStatus: true,
			},
		}

		for _, test := range tests {
			deployment.Status = test.deployStatus

			_, actualStatus, _ := isDeploymentFinished(deployment)
			if test.expectedStatus != actualStatus {
				t.Errorf("Failed test: %s\n DeploymentStatus: %+v", test.testDescription, test.deployStatus)
			}
		}

	})

}
