package cable_courier

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/foundation-go/foundation/errors/proto"
)

// AnyCable expects `data` to hold the JSON *text* of the payload. That used to
// be produced by marshalling a throwaway struct and slicing the result with
// hardcoded offsets — res[9:len(res)-2] — which only worked as long as nobody
// touched that struct's field name.
func TestNewEventJSONFromMessage(t *testing.T) {
	raw, err := newEventJSONFromMessage(
		"foundation.errors.NotFoundError",
		&pb.NotFoundError{Kind: "chat", Id: "1"},
		"user:42",
		"corr-1",
	)
	require.NoError(t, err)

	var envelope Event
	require.NoError(t, json.Unmarshal([]byte(raw), &envelope))
	assert.Equal(t, "user:42", envelope.Stream)

	// envelope.Data is the JSON text of the payload, still quoted.
	var payloadJSON string
	require.NoError(t, json.Unmarshal([]byte(envelope.Data), &payloadJSON))

	var payload EventData
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))

	assert.Equal(t, "foundation.errors.NotFoundError", payload.Event)
	assert.Equal(t, "corr-1", payload.CorrelationID)
	assert.Equal(t, "chat", payload.Data["kind"])
	assert.Equal(t, "1", payload.Data["id"])
}

// Values that need escaping have to survive the double encoding intact.
func TestNewEventJSONFromMessageEscapesCorrectly(t *testing.T) {
	raw, err := newEventJSONFromMessage(
		"foundation.errors.NotFoundError",
		&pb.NotFoundError{Kind: `he said "hi"`, Id: "a\nb"},
		"user:42",
		"corr-1",
	)
	require.NoError(t, err)

	var envelope Event
	require.NoError(t, json.Unmarshal([]byte(raw), &envelope))

	var payloadJSON string
	require.NoError(t, json.Unmarshal([]byte(envelope.Data), &payloadJSON))

	var payload EventData
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))

	assert.Equal(t, `he said "hi"`, payload.Data["kind"])
	assert.Equal(t, "a\nb", payload.Data["id"])
}

// The publish used context.Background(), so an unresponsive Redis blocked the
// courier indefinitely and held up shutdown with it.
func TestBroadcastMessageHonoursTheContext(t *testing.T) {
	// Reserved as invalid by RFC 6890, so the dial can never succeed.
	client := NewClient(redis.NewClient(&redis.Options{Addr: "192.0.2.1:6379"}), "__anycable__")
	t.Cleanup(func() { _ = client.Redis.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- client.BroadcastMessage(ctx, "x.Y", &pb.NotFoundError{}, "user:1", "corr")
	}()

	select {
	case err := <-done:
		assert.Error(t, err, "a cancelled context must abort the publish")
	case <-time.After(2 * time.Second):
		t.Fatal("BroadcastMessage ignored the cancelled context")
	}
}
