package foundation

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func initLogging() {
	switch AppEnv() {
	case EnvProduction:
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{})
	default:
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	}

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}

	log.SetLevel(logLevel)
}

func logApplicationStartup(mode string) {
	log.Infof("Starting application")
	log.Infof("- Mode:        %s", mode)
	log.Infof("- Environment: %s", AppEnv())
	log.Infof("- Foundation:  v%s", Version)
}
