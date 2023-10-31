package main

import (
	f "github.com/ri-nat/foundation"

	cablegrpc "github.com/ri-nat/foundation/cable/grpc"
)

var (
	app = f.InitCableGRPC("clubchat-cable_grpc")
)

func main() {
	app.Start(&f.CableGRPCOptions{
		Channels: map[string]cablegrpc.Channel{
			"ChatsChannel": &chatsChannel{},
			"UserChannel":  &userChannel{},
		},
		WithAuthentication: true,
		AuthenticationFunc: cablegrpc.HydraAuthenticationFunc,
	})
}
