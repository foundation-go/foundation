package cable_courier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Command struct {
	Command string `json:"command"`
	Data    string `json:"data"`
}

type Event struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

type EventData struct {
	Event         string                 `json:"event"`
	Data          map[string]interface{} `json:"data"`
	CorrelationID string                 `json:"correlationId"`
}

type Client struct {
	Redis        *redis.Client
	RedisChannel string
}

func NewClient(rdc *redis.Client, redisChannel string) *Client {
	return &Client{Redis: rdc, RedisChannel: redisChannel}
}

// BroadcastMessage publishes a message to an AnyCable stream.
//
// The context bounds the Redis publish; it used to be context.Background(), so
// an unresponsive Redis blocked the courier indefinitely and held up shutdown
// with it.
func (c *Client) BroadcastMessage(ctx context.Context, msgName string, msg proto.Message, stream, correlationID string) error {
	msgJSON, err := newEventJSONFromMessage(msgName, msg, stream, correlationID)
	if err != nil {
		return fmt.Errorf("failed to marshal anycable message: %w", err)
	}

	if err = c.publish(ctx, msgJSON); err != nil {
		return fmt.Errorf("failed to publish anycable message: %w", err)
	}

	return nil
}

func (c *Client) publish(ctx context.Context, msg string) error {
	return c.Redis.Publish(ctx, c.RedisChannel, msg).Err()
}

func newEventJSONFromMessage(msgName string, msg protoreflect.ProtoMessage, stream string, correlationID string) (string, error) {
	res, err := protojson.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal proto message: %w", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(res, &data)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal json from proto message: %w", err)
	}

	eventData := EventData{
		Event:         msgName,
		Data:          data,
		CorrelationID: correlationID,
	}
	res, err = json.Marshal(eventData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event data: %w", err)
	}

	// AnyCable expects `data` to hold the JSON *text* of the payload, so what
	// goes in the field is the payload's JSON string literal — quotes and
	// escaping included.
	//
	// This used to be produced by marshalling a throwaway {"data": ...} struct
	// and slicing the result with hardcoded offsets, res[9:len(res)-2], which
	// only worked as long as nobody touched that struct's field name. Encoding
	// the string directly does the same thing and says so.
	quoted, err := json.Marshal(string(res))
	if err != nil {
		return "", fmt.Errorf("failed to encode event data as a JSON string: %w", err)
	}

	event := &Event{
		Stream: stream,
		Data:   string(quoted),
	}

	res, err = json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	return string(res), nil
}
