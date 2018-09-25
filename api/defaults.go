package api

const (
	DefaultPortName         = "http"
	NavTruststoreFasitAlias = "nav_truststore"
)

func DefaultResourceRequests() []ResourceRequest {
	return []ResourceRequest{
		{
			Alias:        NavTruststoreFasitAlias,
			ResourceType: "certificate",
			PropertyMap:  map[string]string{"keystore": "NAV_TRUSTSTORE_PATH"},
		},
	}
}

func GetDefaultManifest(application string) NaisManifest {

	defaultManifest := NaisManifest{
		Replicas: Replicas{
			Min: 2,
			Max: 4,
			CpuThresholdPercentage: 50,
		},
		Port: 8080,
		Prometheus: PrometheusConfig{
			Enabled: false,
			Port:    DefaultPortName,
			Path:    "/metrics",
		},
		Istio: IstioConfig{
			Enabled: false,
		},
		Healthcheck: Healthcheck{
			Liveness: Probe{
				Path:             "isAlive",
				InitialDelay:     20,
				PeriodSeconds:    10,
				FailureThreshold: 3,
				Timeout:          1,
			},
			Readiness: Probe{
				Path:             "isReady",
				InitialDelay:     20,
				PeriodSeconds:    10,
				FailureThreshold: 3,
				Timeout:          1,
			},
		},
		Ingress: Ingress{Disabled: false},
		Resources: ResourceRequirements{
			Limits: ResourceList{
				Cpu:    "500m",
				Memory: "512Mi",
			},
			Requests: ResourceList{
				Cpu:    "200m",
				Memory: "256Mi",
			},
		},
		LeaderElection: false,
		Redis:          Redis{Enabled: false},
		Secrets:        false,
	}
	defaultManifest.Image = "docker.adeo.no:5000/" + application

	return defaultManifest
}
