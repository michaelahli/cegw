package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/server"
)

func TestHTTP_MarketDataEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", httpAddr)
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("ListMarkets", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange API")
		url := fmt.Sprintf("%s/v1/market/list?exchange=1", baseURL)
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if _, ok := result["markets"]; !ok {
			t.Error("Response missing 'markets' field")
		}
	})

	t.Run("ListMarkets_InvalidExchange", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/market/list?exchange=0", baseURL)
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for invalid exchange")
		}
	})

	t.Run("GetCurrentPrice_InvalidExchange", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/market/price/0/BTC/USDT", baseURL)
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for invalid exchange")
		}
	})

	t.Run("SearchTicker", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange API")
		url := fmt.Sprintf("%s/v1/market/search?exchange=1&query=BTC", baseURL)
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
	})

	t.Run("SearchTicker_EmptyQuery", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/market/search?exchange=1&query=", baseURL)
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for empty query")
		}
	})

	t.Run("GetQuotes_POST", func(t *testing.T) {
		t.Skip("Skipping GetQuotes test - takes too long with real API")
		url := fmt.Sprintf("%s/v1/market/quotes", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 1,
			"symbol":   "BTC/USDT",
			"interval": 4,
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Status: %d, Body: %s", resp.StatusCode, string(body))
		}
	})
}

func TestHTTP_TradingEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", httpAddr)
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("CreateMarketOrder_MissingCredentials", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/trading/order", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 1,
			"symbol":   "BTC/USDT",
			"side":     1,
			"quantity": 0.001,
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for missing credentials")
		}
	})

	t.Run("CreateMarketOrder_InvalidExchange", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/trading/order", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 0,
			"symbol":   "BTC/USDT",
			"side":     1,
			"quantity": 0.001,
			"credentials": map[string]interface{}{
				"api_key":    "test_key",
				"api_secret": "test_secret",
				"sandbox":    true,
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for invalid exchange")
		}
	})

	t.Run("CreateMarketOrder_ZeroQuantity", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/trading/order", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 1,
			"symbol":   "BTC/USDT",
			"side":     1,
			"quantity": 0,
			"credentials": map[string]interface{}{
				"api_key":    "test_key",
				"api_secret": "test_secret",
				"sandbox":    true,
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for zero quantity")
		}
	})

	t.Run("TestCredentials_InvalidExchange", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/trading/credentials/test", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 0,
			"credentials": map[string]interface{}{
				"api_key":    "test_key",
				"api_secret": "test_secret",
				"sandbox":    true,
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for invalid exchange")
		}
	})

	t.Run("TestCredentials_MissingCredentials", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/trading/credentials/test", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 1,
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for missing credentials")
		}
	})
}

func TestHTTP_MonitoringEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", httpAddr)
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("CheckPriceAlerts_InvalidExchange", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/monitoring/alerts", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 0,
			"alerts": []map[string]interface{}{
				{
					"symbol":       "BTC/USDT",
					"target_price": 50000,
					"condition":    1,
				},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for invalid exchange")
		}
	})

	t.Run("CheckPriceAlerts_EmptyAlerts", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/monitoring/alerts", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 1,
			"alerts":   []map[string]interface{}{},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for empty alerts")
		}
	})

	t.Run("CheckPriceAlerts_InvalidTargetPrice", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/monitoring/alerts", baseURL)
		reqBody := map[string]interface{}{
			"exchange": 1,
			"alerts": []map[string]interface{}{
				{
					"symbol":       "BTC/USDT",
					"target_price": 0,
					"condition":    1,
				},
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error status for invalid target price")
		}
	})
}

func TestHTTP_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Skipping test that requires real exchange API")

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", httpAddr)
	client := &http.Client{Timeout: 30 * time.Second}

	const numRequests = 10

	errChan := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			url := fmt.Sprintf("%s/v1/market/list?exchange=1", baseURL)
			resp, err := client.Get(url)
			if err != nil {
				errChan <- err
				return
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				errChan <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				return
			}

			errChan <- nil
		}()
	}

	for i := 0; i < numRequests; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}
}

func TestHTTP_JSONResponseFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", httpAddr)
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("ValidJSON_ListMarkets", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange API")
		url := fmt.Sprintf("%s/v1/market/list?exchange=1", baseURL)
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		var result cegwv1.ListMarketsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode JSON: %v", err)
		}
	})

	t.Run("ValidJSON_ErrorResponse", func(t *testing.T) {
		url := fmt.Sprintf("%s/v1/market/list?exchange=0", baseURL)
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode JSON error: %v", err)
		}
	})
}
