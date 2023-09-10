package main

import (
	f "github.com/ri-nat/foundation"
	ferr "github.com/ri-nat/foundation/errors"
	pb "github.com/ri-nat/foundation/examples/clubchat/protos/chats"

	"google.golang.org/grpc"
)

// Initialize the application
var (
	app = f.InitGRPCServer("chats-grpc")
)

// Custom error codes
const (
	// ErrorCodeCustom is the custom error code used in the example.
	ErrorCodeCustom ferr.ErrorCode = "custom_code"
)

// OAuth scopes
const (
	// ScopeProfile is the scope required to access the profile service.
	ScopeProfile = "profile"

	// ScopeChats is the scope required to access the chats service.
	ScopeChats = "chats"
)

type chatsServer struct {
	pb.UnimplementedChatsServer
}

func main() {
	// Define the options for the gRPC server
	opts := &f.GRPCServerOptions{
		RegisterFunc: func(s *grpc.Server) {
			pb.RegisterChatsServer(s, &chatsServer{})
		},
		StartComponentsOptions: []f.StartComponentsOption{
			f.WithKafkaProducer(),
		},
	}

	// Start the application
	app.Start(opts)
}
