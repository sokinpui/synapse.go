package client

import (
	"context"
	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"io"
	"strings"

	pb "github.com/sokinpui/synapse.go/v2/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GenerationConfig struct {
	Temperature  *float32
	TopP         *float32
	TopK         *float32
	OutputLength *int32
}

type GenerateRequest struct {
	Prompt    string
	ModelCode string
	Stream    bool
	Config    *GenerationConfig
	Images    [][]byte
}

type Result struct {
	Text        string
	Err         error
	IsKeepAlive bool
}

type Client interface {
	GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error)
	ListModels(ctx context.Context) ([]string, error)

	Close() error
}

type grpcClient struct {
	conn   *grpc.ClientConn
	client pb.GenerateClient
}

func New(addr string) (Client, error) {
	var opts []grpc.DialOption

	// Check if the address ends in :443 (standard SSL port)
	if strings.HasSuffix(addr, ":443") {
		// Create TLS credentials
		creds := credentials.NewTLS(&tls.Config{
			// In production, you might want to load system roots,
			// but typically empty Config{} uses system defaults which works for Let's Encrypt.
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// Fallback to insecure for localhost/dev
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}

	return &grpcClient{
		conn:   conn,
		client: pb.NewGenerateClient(conn),
	}, nil
}

func (c *grpcClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *grpcClient) GenerateTask(ctx context.Context, req *GenerateRequest) (<-chan Result, error) {
	pbReq := &pb.Request{
		Prompt:    req.Prompt,
		ModelCode: req.ModelCode,
		Stream:    req.Stream,
		Images:    req.Images,
	}

	if req.Config != nil {
		pbReq.Config = &pb.GenerationConfig{
			Temperature:  req.Config.Temperature,
			TopP:         req.Config.TopP,
			TopK:         req.Config.TopK,
			OutputLength: req.Config.OutputLength,
		}
	}

	stream, err := c.client.GenerateTask(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan Result)

	go func() {
		defer close(resultChan)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return // Stream finished successfully
			}
			if err != nil {
				resultChan <- Result{Err: err}
				return
			}
			switch resp.GetType() {
			case pb.Response_CHUNK:
				resultChan <- Result{Text: resp.GetChunk()}
			case pb.Response_KEEPALIVE:
				// Propagate keep-alive signal to the client application.
				resultChan <- Result{IsKeepAlive: true}
			}
		}
	}()

	return resultChan, nil
}

func (c *grpcClient) ListModels(ctx context.Context) ([]string, error) {
	resp, err := c.client.ListModels(ctx, &pb.ListModelsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}
