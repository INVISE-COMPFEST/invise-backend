package valkey

import (
	"context"
	"net"

	"invise-backend/internal/bootstrap/config"

	"github.com/redis/go-redis/v9"
)

// NewClient creates a new Redis/Valkey client from the provided config.
func NewClient(cfg config.ValkeyConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     net.JoinHostPort(cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

// Ping checks the connection to the Valkey server.
func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}
