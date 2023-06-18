package foundation

import (
	"os"
	"testing"
)

func TestGetBrokers(t *testing.T) {
	// Test case 1: KAFKA_BROKERS is not set
	os.Setenv("KAFKA_BROKERS", "")
	_, err := getBrokers()
	if err == nil {
		t.Errorf("Expected error, but got nil")
	}

	// Test case 2: KAFKA_BROKERS is set with one broker
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	brokers, err := getBrokers()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if brokers != "localhost:9092" {
		t.Errorf("Expected broker to be localhost:9092, but got %s", brokers)
	}

	// Test case 3: KAFKA_BROKERS is set with space on both sides
	os.Setenv("KAFKA_BROKERS", " localhost:9092 ")
	brokers, err = getBrokers()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if brokers != "localhost:9092" {
		t.Errorf("Expected broker to be localhost:9092, but got %s", brokers)
	}
}
