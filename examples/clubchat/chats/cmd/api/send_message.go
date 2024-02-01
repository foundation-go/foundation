package main

import (
	"context"
	"database/sql"

	"google.golang.org/protobuf/types/known/timestamppb"

	f "github.com/foundation-go/foundation"
	ferr "github.com/foundation-go/foundation/errors"
	pb "github.com/foundation-go/foundation/examples/clubchat/protos/chats"
)

func (s *chatsServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.Message, error) {
	var message = &pb.Message{}

	if err := app.WithTransaction(ctx, func(tx *sql.Tx) (*f.Event, ferr.FoundationError) {
		event := &pb.MessageSentEvent{
			Message:    message,
			HappenedAt: timestamppb.Now(),
		}

		return f.NewEventFromProto(event, message.Id, nil)
	}); err != nil {
		return nil, err
	}

	return message, nil
}
