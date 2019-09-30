package api

import (
	"github.com/nais/naisd/api/naisrequest"
	"k8s.io/apimachinery/pkg/util/uuid"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/nais/naisd/pkg/event"
	docker "github.com/novln/docker-parser"
)

// NewDeploymentEvent creates a new deployment event based on a naisd deployment request along with its manifest and cluster name.
func NewDeploymentEvent(request naisrequest.Deploy, manifest NaisManifest, clusterName string) deployment.Event {
	image := ContainerImage(manifest.Image)
	ts := convertTimestamp(time.Now())
	id := uuid.NewUUID()

	return deployment.Event{
		CorrelationID: string(id),
		Platform: &deployment.Platform{
			Type: deployment.PlatformType_nais,
		},
		Source: deployment.System_naisd,
		Deployer: &deployment.Actor{
			Ident: request.FasitUsername,
		},
		Team:            manifest.Team,
		RolloutStatus:   deployment.RolloutStatus_unknown,
		Environment:     environment(request),
		SkyaEnvironment: request.FasitEnvironment,
		Namespace:       request.Namespace,
		Cluster:         clusterName,
		Application:     request.Application,
		Version:         image.GetTag(),
		Image:           &image,
		Timestamp:       &ts,
	}
}

func convertTimestamp(t time.Time) timestamp.Timestamp {
	return timestamp.Timestamp{
		Seconds: int64(t.Unix()),
		Nanos:   int32(t.UnixNano()),
	}
}

func environment(request naisrequest.Deploy) deployment.Environment {
	if request.FasitEnvironment == "p" {
		return deployment.Environment_production
	}
	return deployment.Environment_development
}

func hashtag(t string) (hash, tag string) {
	if strings.ContainsRune(t, ':') {
		return t, ""
	}
	return "", t
}

// ContainerImage parses a Docker image name and return it as structured data.
func ContainerImage(imageName string) deployment.ContainerImage {
	ref, err := docker.Parse(imageName)
	if err != nil {
		return deployment.ContainerImage{}
	}
	hash, tag := hashtag(ref.Tag())
	return deployment.ContainerImage{
		Name: ref.Repository(),
		Tag:  tag,
		Hash: hash,
	}
}
