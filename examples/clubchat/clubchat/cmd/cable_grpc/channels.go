package main

import (
	"context"
	"fmt"
)

type userChannel struct{}

func (c *userChannel) Authorize(ctx context.Context, userID string, _ map[string]string) error {
	return nil
}

func (c *userChannel) GetStreams(ctx context.Context, userID string, _ map[string]string) []string {
	return []string{fmt.Sprintf("user:%s", userID)}
}

type chatsChannel struct{}

func (c *chatsChannel) Authorize(ctx context.Context, userID string, ident map[string]string) error {
	val, ok := ident["chat_id"]
	if !ok || val == "" {
		return fmt.Errorf("chat_id is required")
	}

	return nil
}

func (c *chatsChannel) GetStreams(ctx context.Context, userID string, ident map[string]string) []string {
	return []string{fmt.Sprintf("chats:%s", ident["chat_id"])}
}
