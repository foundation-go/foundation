package foundation

import (
	"os"
	"testing"
)

func TestAppEnv(t *testing.T) {
	// Test that the default environment is development
	env := AppEnv()
	if env != EnvDevelopment {
		t.Errorf("Expected default environment to be %s, but got %s", EnvDevelopment, env)
	}

	// Test that the environment can be set via the APP_ENV environment variable
	os.Setenv("APP_ENV", "production")
	env = AppEnv()
	if env != EnvProduction {
		t.Errorf("Expected environment to be %s, but got %s", EnvProduction, env)
	}

	// Test that the environment is forced to development if not production
	os.Setenv("APP_ENV", "test")
	env = AppEnv()
	if env != EnvDevelopment {
		t.Errorf("Expected environment to be %s, but got %s", EnvDevelopment, env)
	}
}

func TestIsProductionEnv(t *testing.T) {
	// Test that the function returns false if the environment is not production
	os.Setenv("APP_ENV", "development")
	isProduction := IsProductionEnv()
	if isProduction {
		t.Errorf("Expected IsProductionEnv() to return false, but got true")
	}

	// Test that the function returns true if the environment is production
	os.Setenv("APP_ENV", "production")
	isProduction = IsProductionEnv()
	if !isProduction {
		t.Errorf("Expected IsProductionEnv() to return true, but got false")
	}
}

func TestIsDevelopmentEnv(t *testing.T) {
	// Test that the function returns false if the environment is not development
	os.Setenv("APP_ENV", "production")
	isDevelopment := IsDevelopmentEnv()
	if isDevelopment {
		t.Errorf("Expected IsDevelopmentEnv() to return false, but got true")
	}

	// Test that the function returns true if the environment is development
	os.Setenv("APP_ENV", "development")
	isDevelopment = IsDevelopmentEnv()
	if !isDevelopment {
		t.Errorf("Expected IsDevelopmentEnv() to return true, but got false")
	}
}

func TestGetEnvOrBool(t *testing.T) {
	// Test that the function returns the default value when the environment variable is not set
	defaultValue := true
	value := GetEnvOrBool("NON_EXISTING_ENV_VAR", defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %t, but got %t", defaultValue, value)
	}

	// Test that the function returns the environment variable value when it is set
	envVar := "ENV_VAR"
	envVarValue := "false"
	os.Setenv(envVar, envVarValue)
	value = GetEnvOrBool(envVar, defaultValue)
	if value != false {
		t.Errorf("Expected value to be false, but got %t", value)
	}

	// Test that the function returns the default value when the environment variable value is not a valid boolean
	os.Setenv(envVar, "not_a_boolean")
	value = GetEnvOrBool(envVar, defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %t, but got %t", defaultValue, value)
	}
}

func TestGetEnvOrInt(t *testing.T) {
	// Test that the function returns the default value when the environment variable is not set
	defaultValue := 123
	value := GetEnvOrInt("NON_EXISTING_ENV_VAR", defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %d, but got %d", defaultValue, value)
	}

	// Test that the function returns the environment variable value when it is set
	envVar := "ENV_VAR"
	envVarValue := "456"
	os.Setenv(envVar, envVarValue)
	value = GetEnvOrInt(envVar, defaultValue)
	if value != 456 {
		t.Errorf("Expected value to be 456, but got %d", value)
	}

	// Test that the function returns the default value when the environment variable value is not a valid integer
	os.Setenv(envVar, "not_an_integer")
	value = GetEnvOrInt(envVar, defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %d, but got %d", defaultValue, value)
	}
}

func TestGetEnvOrString(t *testing.T) {
	// Test that the function returns the default value when the environment variable is not set
	defaultValue := "default"
	value := GetEnvOrString("NON_EXISTING_ENV_VAR", defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %s, but got %s", defaultValue, value)
	}

	// Test that the function returns the environment variable value when it is set
	envVar := "ENV_VAR"
	envVarValue := "value"
	os.Setenv(envVar, envVarValue)
	value = GetEnvOrString(envVar, defaultValue)
	if value != envVarValue {
		t.Errorf("Expected value to be %s, but got %s", envVarValue, value)
	}

	// Test that the function returns the default value when the environment variable value is empty
	os.Setenv(envVar, "")
	value = GetEnvOrString(envVar, defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %s, but got %s", defaultValue, value)
	}
}
