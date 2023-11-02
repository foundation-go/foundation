package main

import (
	"context"
	"fmt"

	f "github.com/foundation-go/foundation"
	"google.golang.org/protobuf/proto"

	cpb "github.com/foundation-go/foundation/examples/clubchat/protos/chats"
)

var (
	app = f.InitCableCourier("clubchat-cable_courier")
)

func main() {
	app.Start(&f.CableCourierOptions{
		Resolvers: f.CableCourierResolvers{
			&cpb.MessageSentEvent{}: {resolveMessageSent},
		},
	})
}

func resolveMessageSent(ctx context.Context, event *f.Event, msg proto.Message) (string, error) {
	return fmt.Sprintf("chats:%s", msg.(*cpb.MessageSentEvent).Message.ChatId), nil
}
