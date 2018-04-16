package naisrequest

import (
	"errors"
	"fmt"
	"github.com/nais/naisd/api/constant"
)

type Deploy struct {
	Application      string `json:"application"`
	Version          string `json:"version"`
	Zone             string `json:"zone"`
	ManifestUrl      string `json:"manifesturl,omitempty"`
	FasitEnvironment string `json:"fasitEnvironment"`
	FasitUsername    string `json:"fasitUsername"`
	FasitPassword    string `json:"fasitPassword"`
	OnBehalfOf       string `json:"onbehalfof,omitempty"`
	Namespace        string `json:"namespace"`
}

func (r Deploy) Validate() []error {
	required := map[string]*string{
		"application":      &r.Application,
		"version":          &r.Version,
		"fasitEnvironment": &r.FasitEnvironment,
		"zone":             &r.Zone,
		"fasitUsername":    &r.FasitUsername,
		"fasitPassword":    &r.FasitPassword,
		"namespace":        &r.Namespace,
	}

	var errs []error
	for key, pointer := range required {
		if len(*pointer) == 0 {
			errs = append(errs, fmt.Errorf("%s is required and is empty", key))
		}
	}

	if r.Zone != constant.ZONE_FSS && r.Zone != constant.ZONE_SBS && r.Zone != constant.ZONE_IAPP {
		errs = append(errs, errors.New("zone can only be fss, sbs or iapp"))
	}

	return errs
}
