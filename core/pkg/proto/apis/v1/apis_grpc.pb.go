// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.4
// source: nitric/proto/apis/v1/apis.proto

package apispb

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

// ApiClient is the client API for Api service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ApiClient interface {
	// Serve a route on an API
	Serve(ctx context.Context, opts ...grpc.CallOption) (Api_ServeClient, error)
}

type apiClient struct {
	cc grpc.ClientConnInterface
}

func NewApiClient(cc grpc.ClientConnInterface) ApiClient {
	return &apiClient{cc}
}

func (c *apiClient) Serve(ctx context.Context, opts ...grpc.CallOption) (Api_ServeClient, error) {
	stream, err := c.cc.NewStream(ctx, &Api_ServiceDesc.Streams[0], "/nitric.proto.apis.v1.Api/Serve", opts...)
	if err != nil {
		return nil, err
	}
	x := &apiServeClient{stream}
	return x, nil
}

type Api_ServeClient interface {
	Send(*ClientMessage) error
	Recv() (*ServerMessage, error)
	grpc.ClientStream
}

type apiServeClient struct {
	grpc.ClientStream
}

func (x *apiServeClient) Send(m *ClientMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *apiServeClient) Recv() (*ServerMessage, error) {
	m := new(ServerMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ApiServer is the server API for Api service.
// All implementations should embed UnimplementedApiServer
// for forward compatibility
type ApiServer interface {
	// Serve a route on an API
	Serve(Api_ServeServer) error
}

// UnimplementedApiServer should be embedded to have forward compatible implementations.
type UnimplementedApiServer struct {
}

func (UnimplementedApiServer) Serve(Api_ServeServer) error {
	return status.Errorf(codes.Unimplemented, "method Serve not implemented")
}

// UnsafeApiServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ApiServer will
// result in compilation errors.
type UnsafeApiServer interface {
	mustEmbedUnimplementedApiServer()
}

func RegisterApiServer(s grpc.ServiceRegistrar, srv ApiServer) {
	s.RegisterService(&Api_ServiceDesc, srv)
}

func _Api_Serve_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ApiServer).Serve(&apiServeServer{stream})
}

type Api_ServeServer interface {
	Send(*ServerMessage) error
	Recv() (*ClientMessage, error)
	grpc.ServerStream
}

type apiServeServer struct {
	grpc.ServerStream
}

func (x *apiServeServer) Send(m *ServerMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *apiServeServer) Recv() (*ClientMessage, error) {
	m := new(ClientMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Api_ServiceDesc is the grpc.ServiceDesc for Api service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Api_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nitric.proto.apis.v1.Api",
	HandlerType: (*ApiServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Serve",
			Handler:       _Api_Serve_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "nitric/proto/apis/v1/apis.proto",
}
