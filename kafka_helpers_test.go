package foundation

import (
	"testing"
)

func TestGetBrokers(t *testing.T) {
	app := Application{Config: &Config{}}

	// Test case 1: KAFKA_BROKERS is not set
	app.Config.KafkaBrokers = ""
	_, err := app.getKafkaBrokers()
	if err == nil {
		t.Errorf("Expected error, but got nil")
	}

	// Test case 2: KAFKA_BROKERS is set with one broker
	app.Config.KafkaBrokers = "localhost:9092"
	brokers, err := app.getKafkaBrokers()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if brokers != "localhost:9092" {
		t.Errorf("Expected broker to be localhost:9092, but got %s", brokers)
	}

	// Test case 3: KAFKA_BROKERS is set with space on both sides
	app.Config.KafkaBrokers = " localhost:9092 "
	brokers, err = app.getKafkaBrokers()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if brokers != "localhost:9092" {
		t.Errorf("Expected broker to be localhost:9092, but got %s", brokers)
	}
}
