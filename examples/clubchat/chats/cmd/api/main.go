package main

import (
	f "github.com/ri-nat/foundation"
	pb "github.com/ri-nat/foundation/examples/clubchat/protos/chats"

	"google.golang.org/grpc"
)

var (
	app = f.InitGRPCServer("chats-grpc")
)

type chatsServer struct {
	pb.UnimplementedChatsServer
}

func main() {
	opts := &f.GRPCServerOptions{
		RegisterFunc: func(s *grpc.Server) {
			pb.RegisterChatsServer(s, &chatsServer{})
		},
		StartComponentsOptions: []f.StartComponentsOption{
			f.WithKafkaProducer(),
		},
	}

	app.Start(opts)
}
