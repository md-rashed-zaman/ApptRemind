//go:build protogen

package entitlements

import (
	"context"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/grpcx"
	entitlementsv1 "github.com/md-rashed-zaman/apptremind/protos/gen/entitlements/v1"
	"google.golang.org/grpc"
)

type Client struct {
	conn   *grpc.ClientConn
	client entitlementsv1.EntitlementsServiceClient
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpcx.Dial(context.Background(), addr, grpcx.DialOptions{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: entitlementsv1.NewEntitlementsServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) GetEntitlements(ctx context.Context, businessID string) (*entitlementsv1.EntitlementsResponse, error) {
	return c.client.GetEntitlements(ctx, &entitlementsv1.EntitlementsRequest{
		BusinessId: businessID,
	})
}
