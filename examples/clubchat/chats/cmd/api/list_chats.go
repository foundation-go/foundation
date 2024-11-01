package main

import (
	"context"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
	pb "github.com/foundation-go/foundation/examples/clubchat/protos/chats"
)

func (s *chatsServer) ListChats(ctx context.Context, req *pb.ListChatsRequest) (*pb.ChatsList, error) {
	// Return an error with custom error code
	if false {
		return nil, ferr.NewInvalidArgumentError("Chat", "", ferr.ErrorViolations{
			"base": {ErrorCodeCustom},
		}, 0)
	}

	// Check required scopes
	if err := fctx.CheckAllScopesPresence(ctx, ScopeProfile, ScopeChats); err != nil {
		return nil, err
	}

	return &pb.ChatsList{}, nil
}
