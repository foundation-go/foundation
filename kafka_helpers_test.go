package foundation

import (
	"testing"
)

func TestGetBrokers(t *testing.T) {
	app := Service{Config: NewConfig()}

	// Test case 1: KAFKA_BROKERS is not set
	app.Config.Kafka.Brokers = []string{}
	_, err := app.getKafkaBrokers()
	if err == nil {
		t.Errorf("Expected error, but got nil")
	}

	// Test case 2: KAFKA_BROKERS is set with one broker
	app.Config.Kafka.Brokers = []string{"localhost:9092"}
	brokers, err := app.getKafkaBrokers()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if brokers[0] != "localhost:9092" {
		t.Errorf("Expected broker to be localhost:9092, but got %s", brokers[0])
	}
}
