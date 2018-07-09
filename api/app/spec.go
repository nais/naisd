package app

// Spec is an identifier for a nais applications kubernetes resources
type Spec struct {
	Application string
	Environment string
	Team        string
}

// Determine and return in which `namespace` this resource should reside.
func (s Spec) Namespace() string {
	return s.Application
}

// Determine and return the `name` for this resource
func (s Spec) ResourceName() string {
	if s.Environment == "default" {
		return "app"
	}

	return s.Environment
}
