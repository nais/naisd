package api

import (
	"testing"
)

func TestCorrectResourceName(t *testing.T) {
	appConfig := AppConfig{}

	req := DeploymentRequest{
		Application: "app",
		Version: "",
		Environment: ""}

	r := ResourceCreator{appConfig, req}

	service := *r.CreateService()

	if service.Name != "app" {
		t.Error("Service name does not match application name")
	}
}