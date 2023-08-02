package foundation

// Component describes an interface for all components in the Foundation framework.
// This could be a service, a database, a cache, etc.
type Component interface {
	// Health returns the health of the component
	Health() error
	// Name returns the name of the component
	Name() string
	// Start starts the component
	Start() error
	// Stop stops the component
	Stop() error
}

// GetComponent returns the component with the given name.
func (app *Application) GetComponent(name string) Component {
	for _, component := range app.Components {
		if component.Name() == name {
			return component
		}
	}

	return nil
}
