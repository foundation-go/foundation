package main

import (
	f "github.com/ri-nat/foundation"
	fg "github.com/ri-nat/foundation/gateway"

	pb "github.com/ri-nat/foundation/examples/clubchat/protos/chats"
)

var (
	svc = f.InitGateway("clubchat-gateway")

	services = []*fg.Service{
		{Name: "chats", Register: pb.RegisterChatsHandlerFromEndpoint},
	}
)

func main() {
	svc.Start(&f.GatewayOptions{
		Services:                        services,
		WithAuthentication:              true,
		AuthenticationDetailsMiddleware: fg.WithHydraAuthenticationDetails,
	})
}
