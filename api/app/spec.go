package app

// Spec is an identifier for a nais applications kubernetes resources
type Spec struct {
	Application           string
	Namespace             string
	Team                  string
}

// Determine and return the `name` for this resource
func (s Spec) ResourceName() string {
	return s.Application
}
