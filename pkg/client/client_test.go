package client_test

import (
	"context"
	"testing"

	"github.com/michaelahli/cegw/pkg/client"
)

func TestNew(t *testing.T) {
	ctx := context.Background()

	t.Run("missing address", func(t *testing.T) {
		_, err := client.New(ctx, client.Config{})
		if err == nil {
			t.Fatal("expected error for missing address")
		}
	})

	t.Run("default timeout", func(t *testing.T) {
		cfg := client.Config{
			Address: "localhost:50051",
			Timeout: 0,
		}
		// This will fail to connect but we just test config
		_, _ = client.New(ctx, cfg)
	})
}
