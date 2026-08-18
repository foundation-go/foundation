package foundation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fkafka "github.com/foundation-go/foundation/kafka"
	fpg "github.com/foundation-go/foundation/postgresql"
)

// The dependency getters used to call Logger.Fatal, so a library function
// terminated the caller's process. Nothing could handle it and nothing could
// test it. They panic instead, which the recovery middleware and interceptors
// turn into a reported 500.
func TestDependencyGettersReportMissingComponents(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name    string
		try     func() error
		get     func()
		wantErr string
	}{
		{
			name:    "postgresql",
			try:     func() error { _, err := svc.TryGetPostgreSQL(); return err },
			get:     func() { svc.GetPostgreSQL() },
			wantErr: "no PostgreSQL component is registered",
		},
		{
			name:    "redis",
			try:     func() error { _, err := svc.TryGetRedis(); return err },
			get:     func() { svc.GetRedis() },
			wantErr: "no Redis component is registered",
		},
		{
			name:    "jobs enqueuer",
			try:     func() error { _, err := svc.TryGetJobsEnqueuer(); return err },
			get:     func() { svc.GetJobsEnqueuer() },
			wantErr: "jobs enqueuer component is not registered",
		},
		{
			name:    "kafka consumer",
			try:     func() error { _, err := svc.TryGetKafkaConsumer(); return err },
			get:     func() { svc.GetKafkaConsumer() },
			wantErr: "no Kafka consumer component is registered",
		},
		{
			name:    "kafka producer",
			try:     func() error { _, err := svc.TryGetKafkaProducer(); return err },
			get:     func() { svc.GetKafkaProducer() },
			wantErr: "no Kafka producer component is registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.try()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)

			// The error explains how to fix the wiring, rather than just
			// stating that something is missing.
			assert.Panics(t, tt.get)
		})
	}
}

// A component that is registered but not started is a distinct, and much more
// confusing, failure than one that was never registered.
func TestDependencyGettersDistinguishUnstartedComponents(t *testing.T) {
	svc := newTestService()
	svc.Components = []Component{
		&fpg.Component{},
		fkafka.NewConsumerComponent(),
	}

	_, err := svc.TryGetPostgreSQL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not been started")

	_, err = svc.TryGetKafkaConsumer()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not been started")
}

// The outbox stores the moment an event was created; letting the broker stamp
// delivery time instead made the same event look younger when it travelled
// through the outbox than when it went straight to Kafka.
func TestNewMessageFromEventCarriesTheEventTime(t *testing.T) {
	createdAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	msg, err := NewMessageFromEvent(&Event{
		Topic:     "chats",
		Key:       "chat-1",
		Payload:   []byte("payload"),
		CreatedAt: createdAt,
		Headers:   map[string]string{"proto-name": "chats.MessageSent"},
	})
	require.NoError(t, err)

	assert.Equal(t, "chats", msg.Topic)
	assert.Equal(t, []byte("chat-1"), msg.Key)
	assert.Equal(t, []byte("payload"), msg.Value)
	assert.Equal(t, createdAt, msg.Time)

	require.Len(t, msg.Headers, 1)
	assert.Equal(t, "proto-name", msg.Headers[0].Key)
	assert.Equal(t, []byte("chats.MessageSent"), msg.Headers[0].Value)
}

func TestAddSuffix(t *testing.T) {
	assert.Equal(t, "suffix", AddSuffix("", "suffix"))
	assert.Equal(t, "name-suffix", AddSuffix("name", "suffix"))
	assert.Equal(t, "name-suffix", AddSuffix("name-suffix", "suffix"))
}

func TestGenerateRandomString(t *testing.T) {
	assert.Len(t, GenerateRandomString(16), 16)
	assert.Empty(t, GenerateRandomString(0))

	// Two draws colliding would mean the generator is not random at all.
	assert.NotEqual(t, GenerateRandomString(32), GenerateRandomString(32))

	for _, c := range GenerateRandomString(256) {
		assert.Contains(t, Alphabet, string(c))
	}
}

// The Redis URL can carry a password, and an error message is a place it must
// not appear.
func TestRedactRedisURL(t *testing.T) {
	assert.Equal(t, "redis://user:xxxxx@host:6379/3", redactRedisURL("redis://user:hunter2@host:6379/3"))
	assert.Equal(t, "redis://host:6379", redactRedisURL("redis://host:6379"))
	assert.Equal(t, "the configured Redis URL", redactRedisURL("://nonsense"))
}
