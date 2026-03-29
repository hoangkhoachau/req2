package grpcclient

import (
	"context"
	"crypto/tls"

	"github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type GrpcClient struct {
	conn *grpc.ClientConn
}

func NewGrpcClient(address string, secure bool) (*GrpcClient, error) {
	creds := lo.Ternary(secure, credentials.NewTLS(&tls.Config{}), insecure.NewCredentials())
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &GrpcClient{conn: conn}, nil
}

func (c *GrpcClient) Close() error {
	return c.conn.Close()
}

func (c *GrpcClient) SendRequest(ctx context.Context, reqStr string, method protoreflect.MethodDescriptor) (*dynamicpb.Message, error) {
	req := dynamicpb.NewMessage(method.Input())
	if err := protojson.Unmarshal([]byte(reqStr), req); err != nil {
		return nil, err
	}

	rsp := dynamicpb.NewMessage(method.Output())
	err := c.conn.Invoke(ctx, "/"+string(method.Parent().FullName())+"/"+string(method.Name()), req, rsp)
	return rsp, err
}
