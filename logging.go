package foundation

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func initLogger(appName string) *log.Entry {
	logger := log.New()

	switch FoundationEnv() {
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

func (s *Service) logStartup() {
	s.Logger.Infof("Starting service `%s`", s.Name)
	s.Logger.Infof(" - Mode:        %s", s.ModeName)
	s.Logger.Infof(" - Environment: %s", FoundationEnv())
	s.Logger.Infof(" - Foundation:  v%s", Version)
}
