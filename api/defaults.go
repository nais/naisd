package api

func GetDefaultAppConfig() NaisAppConfig {

	return NaisAppConfig{
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
