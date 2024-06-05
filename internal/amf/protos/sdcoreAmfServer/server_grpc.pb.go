// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.5
// source: server.proto

package sdcoreAmfServer

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

// NgapServiceClient is the client API for NgapService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type NgapServiceClient interface {
	HandleMessage(ctx context.Context, opts ...grpc.CallOption) (NgapService_HandleMessageClient, error)
}

type ngapServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewNgapServiceClient(cc grpc.ClientConnInterface) NgapServiceClient {
	return &ngapServiceClient{cc}
}

func (c *ngapServiceClient) HandleMessage(ctx context.Context, opts ...grpc.CallOption) (NgapService_HandleMessageClient, error) {
	stream, err := c.cc.NewStream(ctx, &NgapService_ServiceDesc.Streams[0], "/sdcoreAmfServer.NgapService/HandleMessage", opts...)
	if err != nil {
		return nil, err
	}
	x := &ngapServiceHandleMessageClient{stream}
	return x, nil
}

type NgapService_HandleMessageClient interface {
	Send(*SctplbMessage) error
	Recv() (*AmfMessage, error)
	grpc.ClientStream
}

type ngapServiceHandleMessageClient struct {
	grpc.ClientStream
}

func (x *ngapServiceHandleMessageClient) Send(m *SctplbMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *ngapServiceHandleMessageClient) Recv() (*AmfMessage, error) {
	m := new(AmfMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// NgapServiceServer is the server API for NgapService service.
// All implementations must embed UnimplementedNgapServiceServer
// for forward compatibility
type NgapServiceServer interface {
	HandleMessage(NgapService_HandleMessageServer) error
	mustEmbedUnimplementedNgapServiceServer()
}

// UnimplementedNgapServiceServer must be embedded to have forward compatible implementations.
type UnimplementedNgapServiceServer struct {
}

func (UnimplementedNgapServiceServer) HandleMessage(NgapService_HandleMessageServer) error {
	return status.Errorf(codes.Unimplemented, "method HandleMessage not implemented")
}
func (UnimplementedNgapServiceServer) mustEmbedUnimplementedNgapServiceServer() {}

// UnsafeNgapServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to NgapServiceServer will
// result in compilation errors.
type UnsafeNgapServiceServer interface {
	mustEmbedUnimplementedNgapServiceServer()
}

func RegisterNgapServiceServer(s grpc.ServiceRegistrar, srv NgapServiceServer) {
	s.RegisterService(&NgapService_ServiceDesc, srv)
}

func _NgapService_HandleMessage_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(NgapServiceServer).HandleMessage(&ngapServiceHandleMessageServer{stream})
}

type NgapService_HandleMessageServer interface {
	Send(*AmfMessage) error
	Recv() (*SctplbMessage, error)
	grpc.ServerStream
}

type ngapServiceHandleMessageServer struct {
	grpc.ServerStream
}

func (x *ngapServiceHandleMessageServer) Send(m *AmfMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *ngapServiceHandleMessageServer) Recv() (*SctplbMessage, error) {
	m := new(SctplbMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// NgapService_ServiceDesc is the grpc.ServiceDesc for NgapService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var NgapService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "sdcoreAmfServer.NgapService",
	HandlerType: (*NgapServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "HandleMessage",
			Handler:       _NgapService_HandleMessage_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "server.proto",
}
