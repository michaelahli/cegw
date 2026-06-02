package ccxt

import (
	"os"
	"testing"
)

func TestShouldUseProxy(t *testing.T) {
	tests := []struct {
		name     string
		noProxy  string
		addr     string
		expected bool
	}{
		{
			name:     "no NO_PROXY set",
			noProxy:  "",
			addr:     "example.com:443",
			expected: true,
		},
		{
			name:     "exact match",
			noProxy:  "example.com",
			addr:     "example.com:443",
			expected: false,
		},
		{
			name:     "domain suffix match",
			noProxy:  ".example.com",
			addr:     "api.example.com:443",
			expected: false,
		},
		{
			name:     "wildcard match",
			noProxy:  "*.example.com",
			addr:     "api.example.com:443",
			expected: false,
		},
		{
			name:     "no match",
			noProxy:  "example.com",
			addr:     "other.com:443",
			expected: true,
		},
		{
			name:     "multiple entries",
			noProxy:  "localhost,example.com,.internal",
			addr:     "api.internal:443",
			expected: false,
		},
		{
			name:     "localhost",
			noProxy:  "localhost",
			addr:     "localhost:8080",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("NO_PROXY", tt.noProxy)
			defer func() { _ = os.Unsetenv("NO_PROXY") }()

			result := shouldUseProxy(tt.addr)
			if result != tt.expected {
				t.Errorf("shouldUseProxy(%q) with NO_PROXY=%q = %v, want %v",
					tt.addr, tt.noProxy, result, tt.expected)
			}
		})
	}
}
