package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestRedirectServiceToNewDeployment(t *testing.T) {
	t.Run("Test redirecing existing service will actualle update it.", func(t *testing.T) {
		existingService := &v1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{
					Protocol:   v1.Protocol("http"),
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				}},
			},
		}

		newTargetSpec := app.Spec{Application: "targetApp", Environment: "targetEnv", Team: "targetTeam"}

		client := fake.NewSimpleClientset(existingService)
		clientHolder := clientHolder{client: client}

		updatedService, err := clientHolder.redirectOldServiceToNewApp(existingService, newTargetSpec)

		assert.NoError(t, err)
		assert.Equal(t, updatedService.Name, existingService.Name)
		assert.Equal(t, updatedService.Spec.ExternalName, fmt.Sprintf("%s.%s.svc.nais.local", newTargetSpec.ResourceName(), newTargetSpec.Namespace()))
	})
}
