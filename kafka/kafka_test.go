package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An unset KAFKA_BROKERS reaches this list as []string{""} after a naive
// strings.Split, which kafka-go then retries against forever instead of
// failing at startup.
func TestValidateBrokers(t *testing.T) {
	tests := []struct {
		name    string
		brokers []string
		wantErr string
	}{
		{name: "valid", brokers: []string{"localhost:9092"}},
		{name: "several", brokers: []string{"a:9092", "b:9092"}},
		{name: "nil", brokers: nil, wantErr: "no Kafka brokers configured"},
		{name: "empty slice", brokers: []string{}, wantErr: "no Kafka brokers configured"},
		{name: "one empty address", brokers: []string{""}, wantErr: "contains an empty address"},
		{name: "blank address", brokers: []string{"a:9092", "   "}, wantErr: "contains an empty address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBrokers(tt.brokers)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestConsumerStartRejectsBadConfiguration(t *testing.T) {
	t.Run("no topics", func(t *testing.T) {
		c := NewConsumerComponent(WithConsumerBrokers([]string{"localhost:9092"}))

		require.Error(t, c.Start())
	})

	t.Run("no brokers", func(t *testing.T) {
		c := NewConsumerComponent(WithConsumerTopics([]string{"topic"}))

		err := c.Start()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no Kafka brokers configured")
	})
}

func TestProducerStartRejectsEmptyBrokers(t *testing.T) {
	err := NewProducerComponent(WithProducerBrokers([]string{""})).Start()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains an empty address")
}

// Every events worker of an application landed in one consumer group, so two
// workers reading different topics took partitions away from each other.
func TestConsumerGroupID(t *testing.T) {
	byAppName := NewConsumerComponent(WithConsumerAppName("chats"))
	assert.Equal(t, "chats-foundation", byAppName.groupID())

	explicit := NewConsumerComponent(
		WithConsumerAppName("chats"),
		WithConsumerGroupID("chats-outbox"),
	)
	assert.Equal(t, "chats-outbox", explicit.groupID())

	// An empty override keeps the default, so wiring an unset env var through
	// does not produce an empty group.
	empty := NewConsumerComponent(WithConsumerAppName("chats"), WithConsumerGroupID(""))
	assert.Equal(t, "chats-foundation", empty.groupID())
}

func TestProducerAllowAutoTopicCreationIsOptIn(t *testing.T) {
	assert.False(t, NewProducerComponent().allowAutoTopicCreate)
	assert.True(t, NewProducerComponent(WithProducerAllowAutoTopicCreation(true)).allowAutoTopicCreate)
}

// Stop and Health used to dereference fields that only exist after Start.
func TestComponentsAreSafeBeforeStart(t *testing.T) {
	consumer := NewConsumerComponent()
	producer := NewProducerComponent()

	assert.NotPanics(t, func() {
		assert.NoError(t, consumer.Stop())
		assert.NoError(t, producer.Stop())
	})

	assert.ErrorContains(t, consumer.Health(), "not initialized")
	assert.ErrorContains(t, producer.Health(), "not initialized")
}

// The consumer's Health used to be a TODO that always returned nil, so an
// unreachable Kafka never showed up in the readiness probe.
func TestHealthReportsUnreachableBrokers(t *testing.T) {
	consumer := NewConsumerComponent(
		WithConsumerAppName("test"),
		WithConsumerLogger(logrus.NewEntry(logrus.New())),
		// Reserved as invalid by RFC 6890, so nothing can answer here.
		WithConsumerBrokers([]string{"192.0.2.1:9092"}),
		WithConsumerTopics([]string{"topic"}),
	)
	require.NoError(t, consumer.Start())
	t.Cleanup(func() { _ = consumer.Stop() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := consumer.HealthContext(ctx)

	require.Error(t, err, "an unreachable broker must fail the health check")
	assert.Contains(t, err.Error(), "unreachable")
}

func TestNewSASLMechanism(t *testing.T) {
	tests := []struct {
		protocol string
		wantErr  bool
	}{
		{protocol: "plain"},
		{protocol: "PLAIN"},
		{protocol: "scram-sha-512"},
		{protocol: "SCRAM-SHA-512"},
		{protocol: "", wantErr: true},
		{protocol: "scram-sha-256", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			mechanism, err := newSASLMechanism(tt.protocol, "user", "pass")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, mechanism)
		})
	}
}
