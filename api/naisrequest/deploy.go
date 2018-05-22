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
	SkipFasit        bool   `json:"skipFasit,omitempty"`
	FasitEnvironment string `json:"fasitEnvironment,omitempty"`
	FasitUsername    string `json:"fasitUsername,omitempty"`
	FasitPassword    string `json:"fasitPassword,omitempty"`
	OnBehalfOf       string `json:"onbehalfof,omitempty"`
	Namespace        string `json:"namespace"`
}

func (r Deploy) Validate() []error {
	required := map[string]*string{
		"application":      &r.Application,
		"version":          &r.Version,
		"zone":             &r.Zone,
		"namespace":        &r.Namespace,
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

	if r.Zone != constant.ZONE_FSS && r.Zone != constant.ZONE_SBS && r.Zone != constant.ZONE_IAPP {
		errs = append(errs, errors.New("zone can only be fss, sbs or iapp"))
	}

	return errs
}
