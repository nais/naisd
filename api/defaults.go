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
			PropertyMap:  nil,
		},
	}
}

func GetDefaultAppConfig(application string) NaisAppConfig {

	defaultAppConfig := NaisAppConfig{
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
		Healthcheck: Healthcheck{
			Liveness: Probe{
				Path:             "isAlive",
				InitialDelay:     20,
				PeriodSeconds:    10,
				FailureThreshold: 3,
			},
			Readiness: Probe{
				Path:             "isReady",
				InitialDelay:     20,
				PeriodSeconds:    10,
				FailureThreshold: 3,
			},
		},
		Ingress: Ingress{Enabled: true},
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
	}
	defaultAppConfig.Image = "docker.adeo.no:5000/" + application

	return defaultAppConfig
}
