package api

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"net/url"
)

func TestAppnNameFromUrl(t *testing.T) {

	u, _ := url.Parse("http://test.url/namespace/appname")
	namespace, appName := parseURLPath(u)

	assert.Equal(t, "appname", appName)
	assert.Equal(t, "namespace", namespace)
}
