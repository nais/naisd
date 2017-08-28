package api

func GetDefaultAppConfig() NaisAppConfig {

	return NaisAppConfig{
		Port: Port{
			Name:       "http",
			Port:       80,
			TargetPort: 8080,
			Protocol:   "http",
		}, Healthcheck: Healthcheck{
			Liveness: Probe{
				Path: "isAlive",
			},
			Readiness: Probe{
				Path: "isReady",
			},
		},
	}
}
