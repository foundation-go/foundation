package main

import (
	f "github.com/ri-nat/foundation"

	cable_grpc "github.com/ri-nat/foundation/cable/grpc"
)

var (
	app = f.InitCableGRPC("clubchat-cable_grpc")
)

func main() {
	app.Start(&f.CableGRPCOptions{
		Channels: map[string]cable_grpc.Channel{
			"ChatsChannel": &chatsChannel{},
			"UserChannel":  &userChannel{},
		},
		WithAuthentication: true,
		AuthenticationFunc: cable_grpc.HydraAuthenticationFunc,
	})
}
