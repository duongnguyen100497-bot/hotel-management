package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client wraps go-redis/v9 with healthcheck and lifecycle helpers.
type Client struct {
	rdb *goredis.Client
}

// NewClient parses a Redis URL (e.g. redis://localhost:6379/0) and initializes a pooled go-redis client.
func NewClient(redisURL string) (*Client, error) {
	trimmed := strings.TrimSpace(redisURL)
	if trimmed == "" {
		return nil, fmt.Errorf("redis URL is required")
	}
	opts, err := goredis.ParseURL(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 3 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 2 * time.Second
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 2 * time.Second
	}
	return &Client{rdb: goredis.NewClient(opts)}, nil
}

// Raw returns the underlying *goredis.Client for module persistence adapters.
func (c *Client) Raw() *goredis.Client {
	if c == nil {
		return nil
	}
	return c.rdb
}

// Ping verifies connectivity to Redis.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return c.rdb.Ping(ctx).Err()
}

// PingCheck returns a readiness probe function compatible with platform/http.CheckFunc.
func (c *Client) PingCheck() func(context.Context) error {
	return func(ctx context.Context) error {
		return c.Ping(ctx)
	}
}

// Close closes the underlying Redis connection pool.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}
