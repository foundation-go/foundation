package main

import (
	f "github.com/foundation-go/foundation"
)

var (
	svc = f.InitOutboxProducer("chats-outbox")
)

func main() {
	svc.Start(&f.OutboxProducerOptions{
		BatchSize: 100,
	})
}
