package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort    string
	HTTPPort    string
	LogLevel    string
	HTTPSProxy  string
	HTTPProxy   string
	Timezone    *time.Location
	SandboxMode bool
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

	return &Config{
		GRPCPort:    grpcPort,
		HTTPPort:    httpPort,
		LogLevel:    logLevel,
		Timezone:    loc,
		SandboxMode: sandboxMode,
		HTTPSProxy:  httpsProxy,
		HTTPProxy:   httpProxy,
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
