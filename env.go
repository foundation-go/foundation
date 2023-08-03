package foundation

import (
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
)

// Env represents the application environment name (development, production, etc).
type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
	EnvTest        Env = "test"
)

// FoundationEnv returns the application environment name.
func FoundationEnv() Env {
	return Env(GetEnvOrString("FOUNDATION_ENV", string(EnvDevelopment)))
}

// IsProductionEnv returns true if the application is running in production mode.
func IsProductionEnv() bool {
	return FoundationEnv() == EnvProduction
}

// IsDevelopmentEnv returns true if the application is running in development mode.
func IsDevelopmentEnv() bool {
	return FoundationEnv() == EnvDevelopment
}

// IsTestEnv returns true if the application is running in test mode.
func IsTestEnv() bool {
	return FoundationEnv() == EnvTest
}

// GetEnvOrBool returns the value of the environment variable named by the key
// argument, or defaultValue if there is no such variable set or it is empty.
func GetEnvOrBool(key string, defaultValue bool) bool {
	value, err := strconv.ParseBool(os.Getenv(key))

	if err != nil {
		return defaultValue
	}

	return value
}

// GetEnvOrInt returns the value of the environment variable named by the key
// argument, or defaultValue if there is no such variable set or it is empty.
func GetEnvOrInt(key string, defaultValue int) int {
	value, err := strconv.Atoi(os.Getenv(key))

	if err != nil {
		return defaultValue
	}

	return value
}

// GetEnvOrString returns the value of the environment variable named by the key
// argument, or defaultValue if there is no such variable set or it is empty.
func GetEnvOrString(key string, defaultValue string) string {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	return value
}
