package valkey

import (
	"testing"

	"invise-backend/internal/bootstrap/config"
)

func TestNewClient(t *testing.T) {
	cfg := config.ValkeyConfig{
		Addr:     "localhost:6379",
		Password: "secret",
		DB:       2,
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	opts := client.Options()
	if opts.Addr != "localhost:6379" {
		t.Errorf("Addr = %q, want %q", opts.Addr, "localhost:6379")
	}
	if opts.Password != "secret" {
		t.Errorf("Password = %q, want %q", opts.Password, "secret")
	}
	if opts.DB != 2 {
		t.Errorf("DB = %d, want %d", opts.DB, 2)
	}
}

func TestNewClientDefaults(t *testing.T) {
	cfg := config.ValkeyConfig{
		Addr: "valkey:6379",
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	opts := client.Options()
	if opts.DB != 0 {
		t.Errorf("DB = %d, want default 0", opts.DB)
	}
}

func TestNewClientPing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := config.ValkeyConfig{
		Addr: "localhost:6379",
	}

	client := NewClient(cfg)
	err := client.Ping(t.Context()).Err()
	if err != nil {
		t.Errorf("Ping failed: %v (is Valkey running?)", err)
	}
}
