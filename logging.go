package foundation

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func initLogger(appName string) *log.Entry {
	logger := log.New()

	switch AppEnv() {
	case EnvProduction:
		logger.SetFormatter(&log.JSONFormatter{})
	default:
		logger.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	}

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}

	logger.SetLevel(logLevel)

	return logger.WithField("app", appName)
}

func (app *Application) logStartup(mode string) {
	app.Logger.Infof("Starting application")
	app.Logger.Infof(" - Mode:        %s", mode)
	app.Logger.Infof(" - Environment: %s", AppEnv())
	app.Logger.Infof(" - Foundation:  v%s", Version)
}
