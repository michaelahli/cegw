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
	GRPCPort           string
	HTTPPort           string
	LogLevel           string
	HTTPSProxy         string
	HTTPProxy          string
	Timezone           *time.Location
	SandboxMode        bool
	Auth               AuthConfig
	AllowedWSOrigins   []string // Allowed WebSocket origins (empty = allow all)
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

	return &Config{
		GRPCPort:    grpcPort,
		HTTPPort:    httpPort,
		LogLevel:    logLevel,
		Timezone:    loc,
		SandboxMode: sandboxMode,
		HTTPSProxy:  httpsProxy,
		HTTPProxy:   httpProxy,
		Auth: AuthConfig{
			Enabled:        authEnabled,
			Type:           authType,
			BasicUsername:  basicUsername,
			BasicPassword:  basicPassword,
			OAuth2Issuer:   oauth2Issuer,
			OAuth2Audience: oauth2Audience,
		},
		AllowedWSOrigins: allowedOrigins,
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
