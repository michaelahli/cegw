package ccxt

import (
	"context"
	"sync"
	"testing"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/logger"
)

func TestClientPool_AcquireRelease(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()

	// First acquire creates a new client
	client1, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if client1 == nil {
		t.Fatal("first acquire returned nil client")
	}

	// Second acquire should return the same client (pooled)
	client2, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	if client2 == nil {
		t.Fatal("second acquire returned nil client")
	}

	// Both should be the same pointer
	if client1 != client2 {
		t.Error("pool should return the same client instance")
	}

	// Check ref count
	pool.mu.Lock()
	ref := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if ref == nil {
		t.Fatal("client not found in pool")
	}
	if ref.refs != 2 {
		t.Errorf("expected refs=2, got %d", ref.refs)
	}

	// Release once
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)
	pool.mu.Lock()
	ref = pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if ref == nil {
		t.Fatal("client should still be in pool after one release")
	}
	if ref.refs != 1 {
		t.Errorf("expected refs=1 after one release, got %d", ref.refs)
	}

	// Release second time - should remove from pool
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)
	pool.mu.Lock()
	_, exists := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if exists {
		t.Error("client should be removed from pool after all releases")
	}
}

func TestClientPool_DifferentExchanges(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()

	// Acquire for two different exchanges
	client1, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("tokocrypto acquire failed: %v", err)
	}
	client2, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_BINANCE, nil)
	if err != nil {
		t.Fatalf("binance acquire failed: %v", err)
	}

	// Different exchanges should have different clients
	if client1 == client2 {
		t.Error("different exchanges should have different client instances")
	}

	// Both should be in pool
	pool.mu.Lock()
	tokoRef := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	binanceRef := pool.clients[cegwv1.Exchange_EXCHANGE_BINANCE]
	pool.mu.Unlock()

	if tokoRef == nil || binanceRef == nil {
		t.Fatal("both clients should be in pool")
	}

	// Release both
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_BINANCE)

	pool.mu.Lock()
	_, tokoExists := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	_, binanceExists := pool.clients[cegwv1.Exchange_EXCHANGE_BINANCE]
	pool.mu.Unlock()

	if tokoExists || binanceExists {
		t.Error("both clients should be removed from pool after release")
	}
}

func TestClientPool_ConcurrentAcquire(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	clients := make([]interface{}, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
			if err != nil {
				t.Errorf("concurrent acquire %d failed: %v", idx, err)
				return
			}
			clients[idx] = client
		}(i)
	}
	wg.Wait()

	// All should be the same pointer
	first := clients[0]
	for i := 1; i < 10; i++ {
		if clients[i] != first {
			t.Errorf("concurrent acquire %d returned different client", i)
		}
	}

	// Release all
	for i := 0; i < 10; i++ {
		pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)
	}

	pool.mu.Lock()
	_, exists := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if exists {
		t.Error("client should be removed after all releases")
	}
}

func TestClientPool_ReleaseUnknown(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()

	// Release on empty pool should not panic
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)
	// Test passes if we reach here without panic
}

func TestClientPool_AcquireAfterRelease(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()

	// Acquire and release
	client1, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)

	// Acquire again - should create a new client (not the old one, but same exchange)
	client2, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}

	// New client should be valid
	if client2 == nil {
		t.Fatal("second acquire returned nil client")
	}

	// Clean up
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)

	// Suppress unused warning
	_ = client1
}

func TestGetClientPool_Singleton(t *testing.T) {
	log := logger.New("error", nil)

	pool1 := GetClientPool(log)
	pool2 := GetClientPool(log)

	if pool1 != pool2 {
		t.Error("GetClientPool should return the same singleton instance")
	}
}

func TestClientPool_BorrowDoesNotIncrementRefs(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()

	// Borrow creates a client with refs=0
	client1, err := pool.Borrow(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("borrow failed: %v", err)
	}
	if client1 == nil {
		t.Fatal("borrow returned nil client")
	}

	pool.mu.Lock()
	ref := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if ref == nil {
		t.Fatal("client not found in pool after borrow")
	}
	if ref.refs != 0 {
		t.Errorf("borrow should not increment refs, got %d", ref.refs)
	}

	// Second borrow returns same client, still refs=0
	client2, err := pool.Borrow(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("second borrow failed: %v", err)
	}
	if client1 != client2 {
		t.Error("borrow should return same client instance")
	}

	pool.mu.Lock()
	ref = pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if ref.refs != 0 {
		t.Errorf("refs should still be 0 after multiple borrows, got %d", ref.refs)
	}
}

func TestClientPool_BorrowThenAcquire(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()

	// Borrow first (refs=0)
	client1, err := pool.Borrow(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("borrow failed: %v", err)
	}

	// Then acquire (refs=1)
	client2, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	if client1 != client2 {
		t.Error("borrow and acquire should return same client instance")
	}

	pool.mu.Lock()
	ref := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if ref.refs != 1 {
		t.Errorf("expected refs=1 after borrow+acquire, got %d", ref.refs)
	}

	// Release the acquire - should remove from pool (refs goes to 0)
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)

	pool.mu.Lock()
	_, exists := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if exists {
		t.Error("client should be removed after release when only acquire ref was held")
	}
}

func TestClientPool_AcquireKeepsClientAlive(t *testing.T) {
	log := logger.New("error", nil)
	pool := &ClientPool{
		clients: make(map[cegwv1.Exchange]*clientRef),
		log:     log,
	}

	ctx := context.Background()

	// Acquire first (refs=1)
	client1, err := pool.Acquire(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// Borrow many times - refs stays at 1
	for i := 0; i < 5; i++ {
		client, err := pool.Borrow(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
		if err != nil {
			t.Fatalf("borrow %d failed: %v", i, err)
		}
		if client != client1 {
			t.Errorf("borrow %d returned different client", i)
		}
	}

	pool.mu.Lock()
	ref := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if ref.refs != 1 {
		t.Errorf("refs should still be 1 after borrows, got %d", ref.refs)
	}

	// Release the acquire
	pool.Release(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)

	pool.mu.Lock()
	_, exists := pool.clients[cegwv1.Exchange_EXCHANGE_TOKOCRYPTO]
	pool.mu.Unlock()
	if exists {
		t.Error("client should be removed after all acquires released")
	}
}
