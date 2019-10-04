package api_test

import (
	"github.com/nais/naisd/api"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/nais/naisd/pkg/event"
	"testing"

	"github.com/stretchr/testify/assert"
)

type containerImageTest struct {
	name      string
	container deployment.ContainerImage
}

var containerImageTests = []containerImageTest{
	{
		name: "nginx",
		container: deployment.ContainerImage{
			Name: "docker.io/library/nginx",
			Tag:  "latest",
		},
	},
	{
		name: "nginx:latest",
		container: deployment.ContainerImage{
			Name: "docker.io/library/nginx",
			Tag:  "latest",
		},
	},
	{
		name: "nginx:tagged",
		container: deployment.ContainerImage{
			Name: "docker.io/library/nginx",
			Tag:  "tagged",
		},
	},
	{
		name: "organization/repo:0.1.2",
		container: deployment.ContainerImage{
			Name: "docker.io/organization/repo",
			Tag:  "0.1.2",
		},
	},
	{
		name: "nginx@sha256:5c3c0bbb737db91024882667ad5acbe64230ddecaca1d019968d8df2c4adab35",
		container: deployment.ContainerImage{
			Name: "docker.io/library/nginx",
			Hash: "sha256:5c3c0bbb737db91024882667ad5acbe64230ddecaca1d019968d8df2c4adab35",
		},
	},
	{
		name: "internal.repo:12345/foo/bar/image",
		container: deployment.ContainerImage{
			Name: "internal.repo:12345/foo/bar/image",
			Tag:  "latest",
		},
	},
	{
		name: "internal.repo:12345/foo/bar/image:tagged",
		container: deployment.ContainerImage{
			Name: "internal.repo:12345/foo/bar/image",
			Tag:  "tagged",
		},
	},
	{
		name: "internal.repo:12345/foo/bar/image@sha256:5c3c0bbb737db91024882667ad5acbe64230ddecaca1d019968d8df2c4adab35",
		container: deployment.ContainerImage{
			Name: "internal.repo:12345/foo/bar/image",
			Hash: "sha256:5c3c0bbb737db91024882667ad5acbe64230ddecaca1d019968d8df2c4adab35",
		},
	},
}

func TestContainerImage(t *testing.T) {
	for _, test := range containerImageTests {
		container := api.ContainerImage(test.name)
		assert.Equal(t, test.container, container)
	}
}

func TestNewDeploymentEvent(t *testing.T) {

	deploymentRequest := naisrequest.Deploy{
		Application:      "myapplication",
		Version:          "1.2.3",
		FasitEnvironment: "t0",
		FasitUsername:    "A123456",
		Namespace:        "mynamespace",
		Environment:      "whichenvironment",
	}

	manifest := api.NaisManifest{
		Team:  "myteam",
		Image: "org/image",
	}

	t.Run("Event defaults are picked up from Application correctly", func(t *testing.T) {
		event := api.NewDeploymentEvent(deploymentRequest, manifest, "test-cluster")

		assert.Equal(t, deployment.PlatformType_nais, event.GetPlatform().GetType())
		assert.Empty(t, event.GetPlatform().GetVariant())
		assert.Equal(t, deployment.System_naisd, event.GetSource())
		assert.Equal(t, "A123456", event.GetDeployer().GetIdent())
		assert.Equal(t, "myteam", event.GetTeam())
		assert.Equal(t, deployment.RolloutStatus_complete, event.GetRolloutStatus())
		assert.Equal(t, deployment.Environment_development, event.GetEnvironment())
		assert.Equal(t, "mynamespace", event.GetNamespace())
		assert.Equal(t, "test-cluster", event.GetCluster())
		assert.Equal(t, "myapplication", event.GetApplication())
		assert.Equal(t, deploymentRequest.Version, event.GetVersion())

		image := event.GetImage()
		assert.NotEmpty(t, image)
		assert.Equal(t, deployment.ContainerImage{
			Name: "docker.io/org/image",
			Tag:  deploymentRequest.Version,
		}, *image)

		assert.True(t, event.GetTimestampAsTime().Unix() > 0)
		assert.True(t, event.GetTimestampAsTime().UnixNano() > 0)
	})

	t.Run("Production cluster derived from FasitEnvironment=p", func(t *testing.T) {
		deploymentRequest.FasitEnvironment = "p"
		event := api.NewDeploymentEvent(deploymentRequest, manifest, "test-cluster")
		assert.Equal(t, deployment.Environment_production, event.GetEnvironment())
	})

}
