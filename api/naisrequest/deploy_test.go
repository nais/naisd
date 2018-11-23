package naisrequest

import (
	"encoding/json"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStringMethodInDeployNaisRequestShouldHidePasswordAndUsername(t *testing.T) {
	deployRequest := naisrequest.Deploy{
		FasitUsername: "username",
		FasitPassword: "password",
		Namespace:     "app",
	}

	var jsonValue = deployRequest.String()
	assert.Contains(t, jsonValue, "***")
	assert.Contains(t, jsonValue, "fasitPassword")
}

func TestStringMethodInDeployNaisRequestShouldNotHidePasswordAndUsername(t *testing.T) {
	deployRequest := naisrequest.Deploy{
		FasitUsername: "username",
		FasitPassword: "password",
		Namespace:     "app",
	}

	jsonValue, err := json.Marshal(deployRequest)
	if err != nil {
		panic(err)
	}
	assert.Contains(t, string(jsonValue), "password")
	assert.Contains(t, string(jsonValue), "fasitPassword")
}
