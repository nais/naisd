package api

const (
	DefaultPortName = "http"
)

func GetDefaultAppConfig(deploymentRequest NaisDeploymentRequest) NaisAppConfig {

	defaultAppConfig := NaisAppConfig{
		Replicas: Replicas{
			Min:                    2,
			Max:                    4,
			CpuThresholdPercentage: 50,
		},
		Port: 8080,
		InitialDelay: 20,
		Prometheus: PrometheusConfig{
			Enabled: false,
			Port:    DefaultPortName,
			Path:    "/metrics",
		},

		Healthcheck: Healthcheck{
			Liveness: Probe{
				Path: "isAlive",
			},
			Readiness: Probe{
				Path: "isReady",
			},
		},
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
	defaultAppConfig.Image = "docker.adeo.no:5000/" + deploymentRequest.Application + ":" + deploymentRequest.Version

	return defaultAppConfig
}
