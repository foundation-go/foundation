package foundation

import (
	"os"
	"testing"
)

func TestFoundationEnv(t *testing.T) {
	// Test that the default environment is current environment
	expectedEnv := Env(GetEnvOrString("FOUNDATION_ENV", string(EnvDevelopment)))
	env := FoundationEnv()
	if env != expectedEnv {
		t.Errorf("Expected default environment to be %s, but got %s", EnvDevelopment, env)
	}

	// Test that the environment can be set via the FOUNDATION_ENV environment variable
	os.Setenv("FOUNDATION_ENV", "production")
	env = FoundationEnv()
	if env != EnvProduction {
		t.Errorf("Expected environment to be %s, but got %s", EnvProduction, env)
	}
}

func TestIsProductionEnv(t *testing.T) {
	// Test that the function returns false if the environment is not production
	os.Setenv("FOUNDATION_ENV", "development")
	isProduction := IsProductionEnv()
	if isProduction {
		t.Errorf("Expected IsProductionEnv() to return false, but got true")
	}

	// Test that the function returns true if the environment is production
	os.Setenv("FOUNDATION_ENV", "production")
	isProduction = IsProductionEnv()
	if !isProduction {
		t.Errorf("Expected IsProductionEnv() to return true, but got false")
	}
}

func TestIsDevelopmentEnv(t *testing.T) {
	// Test that the function returns false if the environment is not development
	os.Setenv("FOUNDATION_ENV", "production")
	isDevelopment := IsDevelopmentEnv()
	if isDevelopment {
		t.Errorf("Expected IsDevelopmentEnv() to return false, but got true")
	}

	// Test that the function returns true if the environment is development
	os.Setenv("FOUNDATION_ENV", "development")
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

func TestGetEnvOrFloat(t *testing.T) {
	// Test that the function returns the default value when the environment variable is not set
	defaultValue := 123.456
	value := GetEnvOrFloat("NON_EXISTING_ENV_VAR", defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %f, but got %f", defaultValue, value)
	}

	// Test that the function returns the environment variable value when it is set
	envVar := "ENV_VAR"
	envVarValue := "456.789"
	os.Setenv(envVar, envVarValue)
	value = GetEnvOrFloat(envVar, defaultValue)
	if value != 456.789 {
		t.Errorf("Expected value to be 456.789, but got %f", value)
	}

	// Test that the function returns the default value when the environment variable value is not a valid float
	os.Setenv(envVar, "not_a_float")
	value = GetEnvOrFloat(envVar, defaultValue)
	if value != defaultValue {
		t.Errorf("Expected value to be %f, but got %f", defaultValue, value)
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
