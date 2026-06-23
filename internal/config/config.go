package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type AuthConfig struct {
	Enabled        bool
	Type           string // "basic" or "oauth2"
	BasicUsername  string
	BasicPassword  string
	OAuth2Issuer   string
	OAuth2Audience string
}

type Config struct {
	GRPCPort              string
	HTTPPort              string
	LogLevel              string
	HTTPSProxy            string
	HTTPProxy             string
	Timezone              *time.Location
	SandboxMode           bool
	Auth                  AuthConfig
	AllowedWSOrigins      []string      // Allowed WebSocket origins (empty = allow all)
	WSPricePollInterval   time.Duration // WebSocket price stream poll interval (fallback)
	WSOrderBookPollInterval time.Duration // WebSocket order book stream poll interval (fallback)
}

func Load() (*Config, error) {
	grpcPort := getEnv("GRPC_PORT", "50051")
	httpPort := getEnv("HTTP_PORT", "8080")
	logLevel := getEnv("LOG_LEVEL", "info")
	timezoneName := getEnv("TIMEZONE", "Asia/Jakarta")
	sandboxMode := getEnvBool("SANDBOX_MODE", false)
	httpsProxy := getEnv("HTTPS_PROXY", "")
	httpProxy := getEnv("HTTP_PROXY", "")

	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		loc = time.UTC
	}

	// Load auth config
	authEnabled := getEnvBool("AUTH_ENABLED", false)
	authType := getEnv("AUTH_TYPE", "basic")
	basicUsername := getEnv("AUTH_BASIC_USERNAME", "")
	basicPassword := getEnv("AUTH_BASIC_PASSWORD", "")
	oauth2Issuer := getEnv("AUTH_OAUTH2_ISSUER", "")
	oauth2Audience := getEnv("AUTH_OAUTH2_AUDIENCE", "")

	// Load WebSocket origin config
	allowedOrigins := []string{}
	if originsEnv := getEnv("ALLOWED_WS_ORIGINS", ""); originsEnv != "" {
		for _, origin := range strings.Split(originsEnv, ",") {
			if trimmed := strings.TrimSpace(origin); trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	// Load WebSocket poll intervals (in seconds, parsed as time.Duration)
	wsPricePoll := getEnvDuration("WS_PRICE_POLL_INTERVAL", 5*time.Second)
	wsOrderBookPoll := getEnvDuration("WS_ORDERBOOK_POLL_INTERVAL", 3*time.Second)

	return &Config{
		GRPCPort:               grpcPort,
		HTTPPort:               httpPort,
		LogLevel:               logLevel,
		Timezone:               loc,
		SandboxMode:            sandboxMode,
		HTTPSProxy:             httpsProxy,
		HTTPProxy:              httpProxy,
		Auth: AuthConfig{
			Enabled:        authEnabled,
			Type:           authType,
			BasicUsername:  basicUsername,
			BasicPassword:  basicPassword,
			OAuth2Issuer:   oauth2Issuer,
			OAuth2Audience: oauth2Audience,
		},
		AllowedWSOrigins:       allowedOrigins,
		WSPricePollInterval:    wsPricePoll,
		WSOrderBookPollInterval: wsOrderBookPoll,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// getEnvDuration reads an environment variable as a duration string (e.g., "5s", "1m").
// Returns the parsed duration or the default if parsing fails or the variable is unset.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
