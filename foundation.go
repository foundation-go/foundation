package foundation

const Version = "0.0.1"

// Application represents a Foundation application.
type Application struct {
	Name string
}

// Init initializes the Foundation application.
func Init(name string) *Application {
	initLogging()

	return &Application{
		Name: name,
	}
}
