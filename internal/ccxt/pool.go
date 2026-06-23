package ccxt

import (
	"context"
	"sync"
	"sync/atomic"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/logger"
)

// clientRef tracks an exchange client with its reference count.
// A positive refs count means at least one long-lived consumer (WebSocket,
// gRPC stream) is holding the client. Short-lived consumers (REST calls)
// use Borrow which does not affect refs.
type clientRef struct {
	client interface{}
	refs   int64 // long-lived references only
}

// ClientPool is a shared pool of CCXT exchange clients per exchange ID.
// It ensures that only one client instance is created for each exchange,
// and clients are recycled via reference counting.
type ClientPool struct {
	mu       sync.Mutex
	clients  map[cegwv1.Exchange]*clientRef
	log      *logger.Logger
}

// globalPool is the singleton pool used across the application.
var globalPool *ClientPool
var poolOnce sync.Once

// GetClientPool returns the singleton ClientPool.
func GetClientPool(log *logger.Logger) *ClientPool {
	poolOnce.Do(func() {
		globalPool = &ClientPool{
			clients: make(map[cegwv1.Exchange]*clientRef),
			log:     log,
		}
	})
	return globalPool
}

// Acquire returns a shared CCXT exchange client for the given exchange
// and increments the long-lived reference count. Use this for long-lived
// consumers such as WebSocket connections and gRPC streams.
//
// The caller MUST call Release when the client is no longer needed.
func (p *ClientPool) Acquire(ctx context.Context, exchange cegwv1.Exchange, creds *cegwv1.Credentials) (interface{}, error) {
	return p.getOrCreate(ctx, exchange, creds, true)
}

// Borrow returns a shared CCXT exchange client for the given exchange
// WITHOUT incrementing the reference count. Use this for short-lived
// consumers such as REST API calls.
//
// The caller does NOT need to call Release.
func (p *ClientPool) Borrow(ctx context.Context, exchange cegwv1.Exchange, creds *cegwv1.Credentials) (interface{}, error) {
	return p.getOrCreate(ctx, exchange, creds, false)
}

// getOrCreate returns an existing client or creates a new one.
// If trackRef is true, the reference count is incremented.
func (p *ClientPool) getOrCreate(ctx context.Context, exchange cegwv1.Exchange, creds *cegwv1.Credentials, trackRef bool) (interface{}, error) {
	p.mu.Lock()

	// Check for existing client
	if ref, ok := p.clients[exchange]; ok {
		if trackRef {
			atomic.AddInt64(&ref.refs, 1)
		}
		client := ref.client
		p.mu.Unlock()
		p.log.WithContext(ctx).
			WithField("exchange", exchange.String()).
			WithField("refs", atomic.LoadInt64(&ref.refs)).
			WithField("tracked", trackRef).
			Debugf("client pool: reusing existing client")
		return client, nil
	}

	// Create new client
	client, err := newClientForExchange(ctx, exchange, creds)
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}

	if client == nil {
		p.mu.Unlock()
		return nil, nil
	}

	ref := &clientRef{
		client: client,
		refs:   0,
	}
	if trackRef {
		ref.refs = 1
	}
	p.clients[exchange] = ref

	p.mu.Unlock()

	p.log.WithContext(ctx).
		WithField("exchange", exchange.String()).
		WithField("tracked", trackRef).
		Debugf("client pool: created new client")

	return client, nil
}

// Release decrements the reference count for the given exchange client.
// When the count reaches zero, the client is closed and removed from the pool.
func (p *ClientPool) Release(ctx context.Context, exchange cegwv1.Exchange) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ref, ok := p.clients[exchange]
	if !ok {
		p.log.WithContext(ctx).
			WithField("exchange", exchange.String()).
			Warnf("client pool: release called for unknown client")
		return
	}

	newRefs := atomic.AddInt64(&ref.refs, -1)
	p.log.WithContext(ctx).
		WithField("exchange", exchange.String()).
		WithField("refs", newRefs).
		Debugf("client pool: released client reference")

	if newRefs <= 0 {
		delete(p.clients, exchange)
		p.log.WithContext(ctx).
			WithField("exchange", exchange.String()).
			Debugf("client pool: destroyed client (no more references)")
	}
}

// newClientForExchange creates a new CCXT exchange client (unpooled).
// This is the underlying factory used by the pool.
func newClientForExchange(ctx context.Context, exchange cegwv1.Exchange, creds *cegwv1.Credentials) (interface{}, error) {
	var log *logger.Logger
	if logVal := ctx.Value("logger"); logVal != nil {
		if l, ok := logVal.(*logger.Logger); ok {
			log = l
		}
	}
	if log == nil {
		log = logger.New("error", nil)
	}

	cfg := ClientConfig{
		Sandbox:  false,
		ProxyURL: ProxyFromEnv(log),
		Options: map[string]any{
			"recvWindow": 5000,
		},
	}

	if creds != nil {
		cfg.APIKey = creds.ApiKey
		cfg.APISecret = creds.ApiSecret
		cfg.Sandbox = creds.Sandbox
		if creds.Sandbox {
			cfg.Options["sandbox"] = true
		}
	}

	switch exchange {
	case cegwv1.Exchange_EXCHANGE_TOKOCRYPTO:
		c := NewTokocryptoClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_BINANCE:
		c := NewBinanceClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_COINBASE:
		c := NewCoinbaseClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_CEXIO:
		c := NewCEXIOClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_INDODAX:
		c := NewIndodaxClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_OKX:
		c := NewOKXClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_KUCOIN:
		c := NewKuCoinClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_CRYPTOCOM:
		c := NewCryptocomClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_BYBIT:
		c := NewBybitClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_BITGET:
		c := NewBitgetClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_COINEX:
		c := NewCoinexClient(cfg, log)
		return c.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_HASHKEY:
		c := NewHashkeyClient(cfg, log)
		return c.Client(ctx)
	default:
		log.WithContext(ctx).
			WithField("exchange", exchange.String()).
			Warnf("unsupported exchange")
		return nil, nil
	}
}