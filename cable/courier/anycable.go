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

func (c *Client) BroadcastMessage(msgName string, msg proto.Message, stream, correlationID string) error {
	msgJSON, err := newEventJSONFromMessage(msgName, msg, stream, correlationID)
	if err != nil {
		return fmt.Errorf("failed to marshal anycable message: %w", err)
	}

	err = c.publish(msgJSON)
	if err != nil {
		return fmt.Errorf("failed to publish anycable message: %w", err)
	}

	return nil
}

func (c *Client) publish(msg string) error {
	return c.Redis.Publish(context.Background(), c.RedisChannel, msg).Err()
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

	type tmpWrapper struct {
		Data string `json:"data"`
	}
	tmp := tmpWrapper{string(res)}

	res, err = json.Marshal(tmp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event data wrapper: %w", err)
	}

	res = res[9 : len(res)-2]

	event := &Event{
		Stream: stream,
		Data:   fmt.Sprintf("\"%s\"", string(res)),
	}

	res, err = json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	return string(res), nil
}
