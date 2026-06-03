package ccxt

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"time"
	"unsafe"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"golang.org/x/net/proxy"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/michaelahli/cegw/internal/logger"
)

type ClientConfig struct {
	APIKey    string
	APISecret string
	Options   map[string]any
	ProxyURL  *url.URL
	Sandbox   bool
}

type TokocryptoClient struct {
	config ClientConfig
	log    *logger.Logger
}

func NewTokocryptoClient(cfg ClientConfig, log *logger.Logger) *TokocryptoClient {
	return &TokocryptoClient{
		config: cfg,
		log:    log,
	}
}

func (c *TokocryptoClient) Client(ctx context.Context) (*ccxt.Tokocrypto, error) {
	log := c.log.WithContext(ctx).WithField("operation", "InitializeTokocryptoClient")

	config := make(map[string]any)

	if c.config.APIKey != "" {
		config["apiKey"] = c.config.APIKey
		config["secret"] = c.config.APISecret
		log.Debugf("configuring with API credentials")
	}

	if c.config.Options != nil {
		config["options"] = c.config.Options
	}

	log.Debugf("creating Tokocrypto exchange instance")
	exchange := ccxt.NewTokocrypto(config)

	if c.config.ProxyURL != nil {
		log.WithField("proxy_scheme", c.config.ProxyURL.Scheme).
			WithField("proxy_host", c.config.ProxyURL.Host).
			Debugf("configuring proxy")

		transport := &http.Transport{}

		switch c.config.ProxyURL.Scheme {
		case "socks5":
			var auth *proxy.Auth
			if c.config.ProxyURL.User != nil {
				password, _ := c.config.ProxyURL.User.Password()
				auth = &proxy.Auth{
					User:     c.config.ProxyURL.User.Username(),
					Password: password,
				}
				log.Debugf("SOCKS5 proxy configured with authentication")
			} else {
				log.Debugf("SOCKS5 proxy configured without authentication")
			}

			dialer, err := proxy.SOCKS5("tcp", c.config.ProxyURL.Host, auth, proxy.Direct)
			if err != nil {
				log.WithError(err).Errorf("failed to create SOCKS5 proxy dialer")
				return nil, status.Error(codes.Internal, "network connection failed")
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if !shouldUseProxy(addr) {
					log.WithField("addr", addr).Debugf("bypassing proxy due to NO_PROXY")
					return proxy.Direct.Dial(network, addr)
				}
				return dialer.Dial(network, addr)
			}
		default:
			log.Debugf("configuring HTTP proxy")
			transport.Proxy = http.ProxyURL(c.config.ProxyURL)
		}

		v := reflect.ValueOf(exchange).Elem()
		f := v.FieldByName("httpClient")
		client := *(**http.Client)(unsafe.Pointer(f.UnsafeAddr())) // nolint:gosec
		client.Transport = transport
		client.Timeout = 30 * time.Second
		log.Debugf("HTTP client transport updated with proxy configuration")
	}

	log.Infof("Tokocrypto client initialized successfully")
	return exchange, nil
}

type BinanceClient struct {
	config ClientConfig
	log    *logger.Logger
}

func NewBinanceClient(cfg ClientConfig, log *logger.Logger) *BinanceClient {
	return &BinanceClient{
		config: cfg,
		log:    log,
	}
}

func (c *BinanceClient) Client(ctx context.Context) (*ccxt.Binance, error) {
	log := c.log.WithContext(ctx).WithField("operation", "InitializeBinanceClient")

	config := make(map[string]any)

	if c.config.APIKey != "" {
		config["apiKey"] = c.config.APIKey
		config["secret"] = c.config.APISecret
		log.Debugf("configuring with API credentials")
	}

	if c.config.Options != nil {
		config["options"] = c.config.Options
	}

	if c.config.Sandbox {
		log.Debugf("enabling sandbox mode")
		if config["options"] == nil {
			config["options"] = make(map[string]any)
		}
		opts := config["options"].(map[string]any)
		opts["defaultType"] = "spot"
		opts["sandboxMode"] = true
	}

	log.Debugf("creating Binance exchange instance")
	exchange := ccxt.NewBinance(config)

	if c.config.ProxyURL != nil {
		log.WithField("proxy_scheme", c.config.ProxyURL.Scheme).
			WithField("proxy_host", c.config.ProxyURL.Host).
			Debugf("configuring proxy")

		transport := &http.Transport{}

		switch c.config.ProxyURL.Scheme {
		case "socks5":
			var auth *proxy.Auth
			if c.config.ProxyURL.User != nil {
				password, _ := c.config.ProxyURL.User.Password()
				auth = &proxy.Auth{
					User:     c.config.ProxyURL.User.Username(),
					Password: password,
				}
				log.Debugf("SOCKS5 proxy configured with authentication")
			} else {
				log.Debugf("SOCKS5 proxy configured without authentication")
			}

			dialer, err := proxy.SOCKS5("tcp", c.config.ProxyURL.Host, auth, proxy.Direct)
			if err != nil {
				log.WithError(err).Errorf("failed to create SOCKS5 proxy dialer")
				return nil, status.Error(codes.Internal, "network connection failed")
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if !shouldUseProxy(addr) {
					log.WithField("addr", addr).Debugf("bypassing proxy due to NO_PROXY")
					return proxy.Direct.Dial(network, addr)
				}
				return dialer.Dial(network, addr)
			}
		default:
			log.Debugf("configuring HTTP proxy")
			transport.Proxy = http.ProxyURL(c.config.ProxyURL)
		}

		v := reflect.ValueOf(exchange).Elem()
		f := v.FieldByName("httpClient")
		client := *(**http.Client)(unsafe.Pointer(f.UnsafeAddr())) // nolint:gosec
		client.Transport = transport
		client.Timeout = 30 * time.Second
		log.Debugf("HTTP client transport updated with proxy configuration")
	}

	log.Infof("Binance client initialized successfully")
	return exchange, nil
}

type CoinbaseClient struct {
	config ClientConfig
	log    *logger.Logger
}

func NewCoinbaseClient(cfg ClientConfig, log *logger.Logger) *CoinbaseClient {
	return &CoinbaseClient{
		config: cfg,
		log:    log,
	}
}

func (c *CoinbaseClient) Client(ctx context.Context) (*ccxt.Coinbase, error) {
	log := c.log.WithContext(ctx).WithField("operation", "InitializeCoinbaseClient")

	config := make(map[string]any)

	if c.config.APIKey != "" {
		config["apiKey"] = c.config.APIKey
		config["secret"] = c.config.APISecret
		log.Debugf("configuring with API credentials")
	}

	if c.config.Options != nil {
		config["options"] = c.config.Options
	}

	if c.config.Sandbox {
		log.Debugf("enabling sandbox mode")
		if config["options"] == nil {
			config["options"] = make(map[string]any)
		}
		opts := config["options"].(map[string]any)
		opts["sandboxMode"] = true
	}

	log.Debugf("creating Coinbase exchange instance")
	exchange := ccxt.NewCoinbase(config)

	if c.config.ProxyURL != nil {
		log.WithField("proxy_scheme", c.config.ProxyURL.Scheme).
			WithField("proxy_host", c.config.ProxyURL.Host).
			Debugf("configuring proxy")

		transport := &http.Transport{}

		switch c.config.ProxyURL.Scheme {
		case "socks5":
			var auth *proxy.Auth
			if c.config.ProxyURL.User != nil {
				password, _ := c.config.ProxyURL.User.Password()
				auth = &proxy.Auth{
					User:     c.config.ProxyURL.User.Username(),
					Password: password,
				}
				log.Debugf("SOCKS5 proxy configured with authentication")
			} else {
				log.Debugf("SOCKS5 proxy configured without authentication")
			}

			dialer, err := proxy.SOCKS5("tcp", c.config.ProxyURL.Host, auth, proxy.Direct)
			if err != nil {
				log.WithError(err).Errorf("failed to create SOCKS5 proxy dialer")
				return nil, status.Error(codes.Internal, "network connection failed")
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if !shouldUseProxy(addr) {
					log.WithField("addr", addr).Debugf("bypassing proxy due to NO_PROXY")
					return proxy.Direct.Dial(network, addr)
				}
				return dialer.Dial(network, addr)
			}
		default:
			log.Debugf("configuring HTTP proxy")
			transport.Proxy = http.ProxyURL(c.config.ProxyURL)
		}

		v := reflect.ValueOf(exchange).Elem()
		f := v.FieldByName("httpClient")
		client := *(**http.Client)(unsafe.Pointer(f.UnsafeAddr())) // nolint:gosec
		client.Transport = transport
		client.Timeout = 30 * time.Second
		log.Debugf("HTTP client transport updated with proxy configuration")
	}

	log.Infof("Coinbase client initialized successfully")
	return exchange, nil
}

type IndodaxClient struct {
	config ClientConfig
	log    *logger.Logger
}

func NewIndodaxClient(cfg ClientConfig, log *logger.Logger) *IndodaxClient {
	return &IndodaxClient{
		config: cfg,
		log:    log,
	}
}

func (c *IndodaxClient) Client(ctx context.Context) (*ccxt.Indodax, error) {
	log := c.log.WithContext(ctx).WithField("operation", "InitializeIndodaxClient")

	config := make(map[string]any)

	if c.config.APIKey != "" {
		config["apiKey"] = c.config.APIKey
		config["secret"] = c.config.APISecret
		log.Debugf("configuring with API credentials")
	}

	if c.config.Options != nil {
		config["options"] = c.config.Options
	}

	log.Debugf("creating Indodax exchange instance")
	exchange := ccxt.NewIndodax(config)

	if c.config.ProxyURL != nil {
		log.WithField("proxy_scheme", c.config.ProxyURL.Scheme).
			WithField("proxy_host", c.config.ProxyURL.Host).
			Debugf("configuring proxy")

		transport := &http.Transport{}

		switch c.config.ProxyURL.Scheme {
		case "socks5":
			var auth *proxy.Auth
			if c.config.ProxyURL.User != nil {
				password, _ := c.config.ProxyURL.User.Password()
				auth = &proxy.Auth{
					User:     c.config.ProxyURL.User.Username(),
					Password: password,
				}
				log.Debugf("SOCKS5 proxy configured with authentication")
			} else {
				log.Debugf("SOCKS5 proxy configured without authentication")
			}

			dialer, err := proxy.SOCKS5("tcp", c.config.ProxyURL.Host, auth, proxy.Direct)
			if err != nil {
				log.WithError(err).Errorf("failed to create SOCKS5 proxy dialer")
				return nil, status.Error(codes.Internal, "network connection failed")
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if !shouldUseProxy(addr) {
					log.WithField("addr", addr).Debugf("bypassing proxy due to NO_PROXY")
					return proxy.Direct.Dial(network, addr)
				}
				return dialer.Dial(network, addr)
			}
		default:
			log.Debugf("configuring HTTP proxy")
			transport.Proxy = http.ProxyURL(c.config.ProxyURL)
		}

		v := reflect.ValueOf(exchange).Elem()
		f := v.FieldByName("httpClient")
		client := *(**http.Client)(unsafe.Pointer(f.UnsafeAddr())) // nolint:gosec
		client.Transport = transport
		client.Timeout = 30 * time.Second
		log.Debugf("HTTP client transport updated with proxy configuration")
	}

	log.Infof("Indodax client initialized successfully")
	return exchange, nil
}

func ProxyFromEnv(log *logger.Logger) *url.URL {
	proxyStr := os.Getenv("HTTPS_PROXY")
	if proxyStr == "" {
		proxyStr = os.Getenv("HTTP_PROXY")
	}
	if proxyStr == "" {
		log.Debugf("no proxy configured in environment")
		return nil
	}

	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		log.WithError(err).
			WithField("proxy_url", proxyStr).
			Warnf("invalid proxy URL configuration")
		return nil
	}

	log.WithField("proxy_scheme", proxyURL.Scheme).
		WithField("proxy_host", proxyURL.Host).
		Infof("proxy loaded from environment")

	return proxyURL
}
