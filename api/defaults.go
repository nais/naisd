package api

func GetDefaultAppConfig() NaisAppConfig {

	return NaisAppConfig{
		Replicas: Replicas{
			Min: 2,
			Max: 4,
			CpuThresholdPercentage: 50,
		},
		Ports: []Port{
			{
				Name:       "http",
				Port:       80,
				TargetPort: 8080,
				Protocol:   "http",
			},
		},
	}
}
