// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.23.4
// source: examples/clubchat/protos/chats/service.proto

package chats

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Chats_ListChats_FullMethodName   = "/foundation.examples.clubchat.chats.Chats/ListChats"
	Chats_SendMessage_FullMethodName = "/foundation.examples.clubchat.chats.Chats/SendMessage"
)

// ChatsClient is the client API for Chats service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ChatsClient interface {
	// ListChats returns a list of chats
	ListChats(ctx context.Context, in *ListChatsRequest, opts ...grpc.CallOption) (*ChatsList, error)
	// SendMessage sends a message to a chat
	SendMessage(ctx context.Context, in *SendMessageRequest, opts ...grpc.CallOption) (*Message, error)
}

type chatsClient struct {
	cc grpc.ClientConnInterface
}

func NewChatsClient(cc grpc.ClientConnInterface) ChatsClient {
	return &chatsClient{cc}
}

func (c *chatsClient) ListChats(ctx context.Context, in *ListChatsRequest, opts ...grpc.CallOption) (*ChatsList, error) {
	out := new(ChatsList)
	err := c.cc.Invoke(ctx, Chats_ListChats_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatsClient) SendMessage(ctx context.Context, in *SendMessageRequest, opts ...grpc.CallOption) (*Message, error) {
	out := new(Message)
	err := c.cc.Invoke(ctx, Chats_SendMessage_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ChatsServer is the server API for Chats service.
// All implementations must embed UnimplementedChatsServer
// for forward compatibility
type ChatsServer interface {
	// ListChats returns a list of chats
	ListChats(context.Context, *ListChatsRequest) (*ChatsList, error)
	// SendMessage sends a message to a chat
	SendMessage(context.Context, *SendMessageRequest) (*Message, error)
	mustEmbedUnimplementedChatsServer()
}

// UnimplementedChatsServer must be embedded to have forward compatible implementations.
type UnimplementedChatsServer struct {
}

func (UnimplementedChatsServer) ListChats(context.Context, *ListChatsRequest) (*ChatsList, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListChats not implemented")
}
func (UnimplementedChatsServer) SendMessage(context.Context, *SendMessageRequest) (*Message, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendMessage not implemented")
}
func (UnimplementedChatsServer) mustEmbedUnimplementedChatsServer() {}

// UnsafeChatsServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ChatsServer will
// result in compilation errors.
type UnsafeChatsServer interface {
	mustEmbedUnimplementedChatsServer()
}

func RegisterChatsServer(s grpc.ServiceRegistrar, srv ChatsServer) {
	s.RegisterService(&Chats_ServiceDesc, srv)
}

func _Chats_ListChats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListChatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ChatsServer).ListChats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Chats_ListChats_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ChatsServer).ListChats(ctx, req.(*ListChatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Chats_SendMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendMessageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ChatsServer).SendMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Chats_SendMessage_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ChatsServer).SendMessage(ctx, req.(*SendMessageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Chats_ServiceDesc is the grpc.ServiceDesc for Chats service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Chats_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "foundation.examples.clubchat.chats.Chats",
	HandlerType: (*ChatsServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListChats",
			Handler:    _Chats_ListChats_Handler,
		},
		{
			MethodName: "SendMessage",
			Handler:    _Chats_SendMessage_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "examples/clubchat/protos/chats/service.proto",
}
