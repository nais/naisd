package naisrequest

import (
	"encoding/json"
	"fmt"
	"github.com/nais/naisd/api/constant"
	"k8s.io/apimachinery/pkg/util/validation"
)

type Deploy struct {
	Application      string `json:"application"`
	Version          string `json:"version"`
	Zone             string `json:"zone"`
	ManifestUrl      string `json:"manifesturl,omitempty"`
	SkipFasit        bool   `json:"skipFasit,omitempty"`
	FasitEnvironment string `json:"fasitEnvironment,omitempty"`
	FasitUsername    string `json:"fasitUsername,omitempty"`
	FasitPassword    string `json:"fasitPassword,omitempty"`
	OnBehalfOf       string `json:"onbehalfof,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	Environment      string `json:"environment,omitempty"`
}

func (r Deploy) Validate() []error {
	required := map[string]*string{
		"application": &r.Application,
		"version":     &r.Version,
		"zone":        &r.Zone,
		"namespace":   &r.Namespace,
	}

	if !r.SkipFasit {
		required["fasitEnvironment"] = &r.FasitEnvironment
		required["fasitUsername"] = &r.FasitUsername
		required["fasitPassword"] = &r.FasitPassword
	}

	var errs []error
	for key, pointer := range required {
		if len(*pointer) == 0 {
			errs = append(errs, fmt.Errorf("%s is required and is empty", key))
		}
	}

	illegalNamespaces := []string{"kube-system", "istio-system", "k8s-dashboard", "kubernetes-dashboard", "nais-rook", "tpa", "reboot-coordinator"}
	for _, illegalNamespace := range illegalNamespaces {
		if r.Namespace == illegalNamespace {
			errs = append(errs, fmt.Errorf("deploying to system namespaces disallowed"))
		}
	}

	if r.Zone != constant.ZONE_FSS && r.Zone != constant.ZONE_SBS && r.Zone != constant.ZONE_IAPP {
		errs = append(errs, fmt.Errorf("zone can only be fss, sbs or iapp"))
	}

	for _, e := range validation.IsDNS1123Label(r.Application) {
		errs = append(errs, fmt.Errorf("invalid application name: %s", e))
	}

	return errs
}

func (r Deploy) String() string {
	r.FasitPassword = "***"
	r.FasitUsername = "***"
	bytes, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return fmt.Sprintf("failed to marshal struct: %s", err)
	}

	return string(bytes)
}
