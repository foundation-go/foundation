package main

import (
	"context"

	f "github.com/ri-nat/foundation"
	pb "github.com/ri-nat/foundation/examples/clubchat/protos/chats"
)

func (s *chatsServer) ListChats(ctx context.Context, req *pb.ListChatsRequest) (*pb.ChatsList, error) {
	// Return an error with custom error code
	if false {
		return nil, f.NewInvalidArgumentError("Chat", "", f.ErrorViolations{
			"base": {ErrorCodeCustom},
		})
	}

	// Check required scopes
	if err := f.CheckAllScopesPresence(ctx, ScopeProfile, ScopeChats); err != nil {
		return nil, err
	}

	return &pb.ChatsList{}, nil
}
