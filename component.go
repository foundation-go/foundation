package foundation

// Component describes an interface for all components in the Foundation framework.
// This could be an external service, a database, a cache, etc.
type Component interface {
	// Health returns the health of the component
	Health() error
	// Name returns the name of the component
	Name() string
	// Start runs the component
	Start() error
	// Stop stops the component
	Stop() error
}

// GetComponent returns the component with the given name.
func (s *Service) GetComponent(name string) Component {
	for _, component := range s.Components {
		if component.Name() == name {
			return component
		}
	}

	return nil
}
