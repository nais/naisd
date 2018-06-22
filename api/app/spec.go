package app

// Spec is an identifier for a nais applications kubernetes resources
type Spec struct {
	Application string
	Environment string
	Team        string
}

func (s Spec) Namespace() string {
	return s.Application
}

func (s Spec) ResourceName() string {
	return s.Environment
}
