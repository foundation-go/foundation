package main

import (
	f "github.com/foundation-go/foundation"
)

var (
	svc = f.InitOutboxCourier("chats-outbox")
)

func main() {
	svc.Start(f.NewOutboxCourierOptions())
}
