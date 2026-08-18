package foundation

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var loadEnvOnce sync.Once

// LoadEnv reads a `.env` file from the working directory into the process
// environment. Variables already set are left alone, and a missing file is not
// an error. It runs at most once.
//
// This used to be a blank import of `godotenv/autoload`, so merely importing
// the framework read a file off disk and mutated the process environment — a
// surprise for anything that imports Foundation as a library, and impossible to
// opt out of. Foundation calls it from Init and from the CLI; set
// FOUNDATION_SKIP_DOTENV to skip it.
func LoadEnv() {
	loadEnvOnce.Do(func() {
		if os.Getenv("FOUNDATION_SKIP_DOTENV") != "" {
			return
		}

		// The only error worth acting on would be a malformed file, and there
		// is no logger yet at this point in startup.
		_ = godotenv.Load()
	})
}

// Env represents the service environment name (development, production, etc).
type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
	EnvTest        Env = "test"
)

// FoundationEnv returns the service environment name.
func FoundationEnv() Env {
	return Env(GetEnvOrString("FOUNDATION_ENV", string(EnvDevelopment)))
}

// IsProductionEnv returns true if the service is running in production mode.
func IsProductionEnv() bool {
	return FoundationEnv() == EnvProduction
}

// IsDevelopmentEnv returns true if the service is running in development mode.
func IsDevelopmentEnv() bool {
	return FoundationEnv() == EnvDevelopment
}

// IsTestEnv returns true if the service is running in test mode.
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

// GetEnvOrFloat returns the value of the environment variable named by the key
// argument, or defaultValue if there is no such variable set or it is empty.
func GetEnvOrFloat(key string, defaultValue float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)

	if err != nil {
		return defaultValue
	}

	return value
}

// GetEnvOrDuration returns the value of the environment variable named by the
// key argument parsed as a time.Duration (e.g. `30s`, `1500ms`, `2m`), or
// defaultValue if there is no such variable set or it cannot be parsed.
func GetEnvOrDuration(key string, defaultValue time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))

	if err != nil {
		return defaultValue
	}

	return value
}

// GetEnvOrDurationSeconds parses the environment variable named by the key
// argument as a time.Duration, accepting a bare number as a count of seconds.
//
// The bare-number form exists for backwards compatibility with settings that
// were documented in seconds; prefer a unit suffix (`500ms`, `2s`).
func GetEnvOrDurationSeconds(key string, defaultValue time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}

	if value, err := time.ParseDuration(raw); err == nil {
		return value
	}

	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second
	}

	return defaultValue
}

// GetEnvOrStrings returns the value of the environment variable named by the
// key argument split on sep, or defaultValue if there is no such variable set
// or it is empty.
//
// Unlike a bare strings.Split, an unset variable yields an empty slice rather
// than a slice holding a single empty string — the latter reads downstream as
// one item with an empty value (an empty Kafka broker address, for instance).
// Empty and whitespace-only items are dropped.
func GetEnvOrStrings(key string, sep string, defaultValue []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}

	var values []string
	for _, part := range strings.Split(raw, sep) {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}

	if len(values) == 0 {
		return defaultValue
	}

	return values
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
