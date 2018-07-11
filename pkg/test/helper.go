package test

import (
	"os"
	"testing"
)

func EnvWrapper(envVars map[string]string, testFunc func(t *testing.T)) func(t *testing.T) {
	return func(t *testing.T) {
		for k, v := range envVars {
			os.Setenv(k, v)
		}
		testFunc(t)

		for k, _ := range envVars {
			os.Unsetenv(k)
		}
	}
}
