package ccxt

import (
	"context"
	"log"
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
}

func NewTokocryptoClient(cfg ClientConfig) *TokocryptoClient {
	return &TokocryptoClient{config: cfg}
}

func (c *TokocryptoClient) Client(ctx context.Context) (*ccxt.Tokocrypto, error) {
	config := make(map[string]any)

	if c.config.APIKey != "" {
		config["apiKey"] = c.config.APIKey
		config["secret"] = c.config.APISecret
	}

	if c.config.Options != nil {
		config["options"] = c.config.Options
	}

	exchange := ccxt.NewTokocrypto(config)

	if c.config.ProxyURL != nil {
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
			}
			dialer, err := proxy.SOCKS5("tcp", c.config.ProxyURL.Host, auth, proxy.Direct)
			if err != nil {
				return nil, status.Error(codes.Internal, "network connection failed")
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		default:
			transport.Proxy = http.ProxyURL(c.config.ProxyURL)
		}

		v := reflect.ValueOf(exchange).Elem()
		f := v.FieldByName("httpClient")
		client := *(**http.Client)(unsafe.Pointer(f.UnsafeAddr())) // nolint:gosec
		client.Transport = transport
		client.Timeout = 30 * time.Second
	}

	return exchange, nil
}

func ProxyFromEnv() *url.URL {
	proxyStr := os.Getenv("HTTPS_PROXY")
	if proxyStr == "" {
		proxyStr = os.Getenv("HTTP_PROXY")
	}
	if proxyStr == "" {
		return nil
	}
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		log.Printf("Invalid proxy URL %q: %v", proxyStr, err) // nolint:gosec
		return nil
	}
	return proxyURL
}
