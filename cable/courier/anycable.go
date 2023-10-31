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

// RedisChannel is a name of redis channel used by AnyCable
//
// TODO: Make configurable via `ANYCABLE_REDIS_CHANNEL`
const RedisChannel = "__anycable__"

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
	Redis *redis.Client
}

func NewClient(rdc *redis.Client) *Client {
	return &Client{Redis: rdc}
}

func (c *Client) BroadcastMessage(msgName string, msg proto.Message, stream, correlationID string) {
	msgJSON := newEventJSONFromMessage(msgName, msg, stream, correlationID)
	c.publish(msgJSON)
}

func (c *Client) publish(msg string) {
	c.Redis.Publish(context.Background(), RedisChannel, msg)
}

// I'm really sorry about loads of `_` in this function. Proper error handling seemed too much for this task at the moment.
// Nevertheless, it should just work if not touched, trust me :)
//
// TODO: Add proper error handling
func newEventJSONFromMessage(msgName string, msg protoreflect.ProtoMessage, stream string, correlationID string) string {
	res, _ := protojson.Marshal(msg)
	var data map[string]interface{}
	_ = json.Unmarshal(res, &data)

	eventData := EventData{
		Event:         msgName,
		Data:          data,
		CorrelationID: correlationID,
	}
	res, _ = json.Marshal(eventData)

	type tmpWrapper struct {
		Data string `json:"data"`
	}
	tmp := tmpWrapper{string(res)}
	res, _ = json.Marshal(tmp)
	res = res[9 : len(res)-2]

	event := &Event{
		Stream: stream,
		Data:   fmt.Sprintf("\"%s\"", string(res)),
	}

	res, _ = json.Marshal(event)
	return string(res)
}
