package api

func GetDefaultAppConfig() NaisAppConfig {

	return NaisAppConfig{
		Replicas: Replicas{
			Min:                    2,
			Max:                    4,
			CpuThresholdPercentage: 50,
		},
		Port: Port{
			Name:       "http",
			Port:       80,
			TargetPort: 8080,
			Protocol:   "http",
		},
		Healthcheck: Healthcheck{
			Liveness: Probe{
				Path: "isAlive",
			},
			Readiness: Probe{
				Path: "isReady",
			},
		},
		Resources: Resources{
			Limits: ContainerResource{
				Cpu: "500m",
				Memory: "512Mi",
			},
			Requests: ContainerResource{
				Cpu: "200m",
				Memory: "256Mi",
			},
		},
	}
}
