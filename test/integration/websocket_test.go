package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/michaelahli/cegw/internal/server"
)

func TestWebSocket_PriceStream(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	t.Run("MissingExchangeParameter", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/price?symbol=BTC/USDT", httpAddr)
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("Expected error for missing exchange parameter")
		}
		if resp != nil && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingSymbolParameter", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/price?exchange=2", httpAddr)
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("Expected error for missing symbol parameter")
		}
		if resp != nil && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidExchangeParameter", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/price?exchange=0&symbol=BTC/USDT", httpAddr)
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}
		if resp != nil && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("ValidConnection_Binance", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange WebSocket")

		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/price?exchange=2&symbol=BTC/USDT", httpAddr)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()

		// Set read deadline
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("Failed to set read deadline: %v", err)
		}

		// Read first message
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		var priceMsg map[string]interface{}
		if err := json.Unmarshal(message, &priceMsg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		// Validate message structure
		if _, ok := priceMsg["symbol"]; !ok {
			t.Error("Message missing 'symbol' field")
		}
		if _, ok := priceMsg["price"]; !ok {
			t.Error("Message missing 'price' field")
		}
		if _, ok := priceMsg["timestamp"]; !ok {
			t.Error("Message missing 'timestamp' field")
		}

		// Validate symbol
		if symbol, ok := priceMsg["symbol"].(string); !ok || symbol != "BTC/USDT" {
			t.Errorf("Expected symbol 'BTC/USDT', got %v", priceMsg["symbol"])
		}

		// Validate price is positive
		if price, ok := priceMsg["price"].(float64); !ok || price <= 0 {
			t.Errorf("Expected positive price, got %v", priceMsg["price"])
		}
	})

	t.Run("MultipleMessages_Binance", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange WebSocket")

		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/price?exchange=2&symbol=ETH/USDT", httpAddr)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		messageCount := 0
		maxMessages := 3

		for messageCount < maxMessages {
			select {
			case <-ctx.Done():
				if messageCount == 0 {
					t.Fatal("Timeout: no messages received")
				}
				return
			default:
			}

			if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatalf("Failed to set read deadline: %v", err)
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("Failed to read message %d: %v", messageCount+1, err)
			}

			var priceMsg map[string]interface{}
			if err := json.Unmarshal(message, &priceMsg); err != nil {
				t.Fatalf("Failed to unmarshal message %d: %v", messageCount+1, err)
			}

			messageCount++
			t.Logf("Received message %d: symbol=%v, price=%v", messageCount, priceMsg["symbol"], priceMsg["price"])
		}

		if messageCount < maxMessages {
			t.Errorf("Expected at least %d messages, got %d", maxMessages, messageCount)
		}
	})
}

func TestWebSocket_OrderBookStream(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	t.Run("MissingExchangeParameter", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/orderbook?symbol=BTC/USDT&limit=10", httpAddr)
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("Expected error for missing exchange parameter")
		}
		if resp != nil && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingSymbolParameter", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/orderbook?exchange=2&limit=10", httpAddr)
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("Expected error for missing symbol parameter")
		}
		if resp != nil && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidExchangeParameter", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/orderbook?exchange=0&symbol=BTC/USDT&limit=10", httpAddr)
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}
		if resp != nil && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("DefaultLimitParameter", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange WebSocket")

		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/orderbook?exchange=2&symbol=BTC/USDT", httpAddr)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()

		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("Failed to set read deadline: %v", err)
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		var obMsg map[string]interface{}
		if err := json.Unmarshal(message, &obMsg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		// Default limit is 20
		if bids, ok := obMsg["bids"].([]interface{}); ok {
			if len(bids) > 20 {
				t.Errorf("Expected max 20 bids with default limit, got %d", len(bids))
			}
		}
		if asks, ok := obMsg["asks"].([]interface{}); ok {
			if len(asks) > 20 {
				t.Errorf("Expected max 20 asks with default limit, got %d", len(asks))
			}
		}
	})

	t.Run("ValidConnection_Binance", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange WebSocket")

		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/orderbook?exchange=2&symbol=BTC/USDT&limit=5", httpAddr)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()

		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("Failed to set read deadline: %v", err)
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		var obMsg map[string]interface{}
		if err := json.Unmarshal(message, &obMsg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		// Validate message structure
		if _, ok := obMsg["symbol"]; !ok {
			t.Error("Message missing 'symbol' field")
		}
		if _, ok := obMsg["bids"]; !ok {
			t.Error("Message missing 'bids' field")
		}
		if _, ok := obMsg["asks"]; !ok {
			t.Error("Message missing 'asks' field")
		}
		if _, ok := obMsg["timestamp"]; !ok {
			t.Error("Message missing 'timestamp' field")
		}

		// Validate symbol
		if symbol, ok := obMsg["symbol"].(string); !ok || symbol != "BTC/USDT" {
			t.Errorf("Expected symbol 'BTC/USDT', got %v", obMsg["symbol"])
		}

		// Validate bids structure
		if bids, ok := obMsg["bids"].([]interface{}); ok {
			if len(bids) > 5 {
				t.Errorf("Expected max 5 bids, got %d", len(bids))
			}
			if len(bids) > 0 {
				if bid, ok := bids[0].([]interface{}); ok && len(bid) >= 2 {
					if price, ok := bid[0].(float64); !ok || price <= 0 {
						t.Errorf("Expected positive bid price, got %v", bid[0])
					}
					if amount, ok := bid[1].(float64); !ok || amount <= 0 {
						t.Errorf("Expected positive bid amount, got %v", bid[1])
					}
				} else {
					t.Error("Invalid bid structure")
				}
			}
		} else {
			t.Error("Bids is not an array")
		}

		// Validate asks structure
		if asks, ok := obMsg["asks"].([]interface{}); ok {
			if len(asks) > 5 {
				t.Errorf("Expected max 5 asks, got %d", len(asks))
			}
			if len(asks) > 0 {
				if ask, ok := asks[0].([]interface{}); ok && len(ask) >= 2 {
					if price, ok := ask[0].(float64); !ok || price <= 0 {
						t.Errorf("Expected positive ask price, got %v", ask[0])
					}
					if amount, ok := ask[1].(float64); !ok || amount <= 0 {
						t.Errorf("Expected positive ask amount, got %v", ask[1])
					}
				} else {
					t.Error("Invalid ask structure")
				}
			}
		} else {
			t.Error("Asks is not an array")
		}
	})

	t.Run("MultipleMessages_Binance", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange WebSocket")

		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/orderbook?exchange=2&symbol=ETH/USDT&limit=10", httpAddr)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		messageCount := 0
		maxMessages := 3

		for messageCount < maxMessages {
			select {
			case <-ctx.Done():
				if messageCount == 0 {
					t.Fatal("Timeout: no messages received")
				}
				return
			default:
			}

			if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatalf("Failed to set read deadline: %v", err)
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("Failed to read message %d: %v", messageCount+1, err)
			}

			var obMsg map[string]interface{}
			if err := json.Unmarshal(message, &obMsg); err != nil {
				t.Fatalf("Failed to unmarshal message %d: %v", messageCount+1, err)
			}

			messageCount++
			bidCount := 0
			askCount := 0
			if bids, ok := obMsg["bids"].([]interface{}); ok {
				bidCount = len(bids)
			}
			if asks, ok := obMsg["asks"].([]interface{}); ok {
				askCount = len(asks)
			}
			t.Logf("Received message %d: symbol=%v, bids=%d, asks=%d", messageCount, obMsg["symbol"], bidCount, askCount)
		}

		if messageCount < maxMessages {
			t.Errorf("Expected at least %d messages, got %d", maxMessages, messageCount)
		}
	})
}

func TestWebSocket_ConnectionHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, httpAddr, cancel := server.StartRealServer(t)
	defer cancel()

	time.Sleep(500 * time.Millisecond)

	t.Run("ClientDisconnect", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange WebSocket")

		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/price?exchange=2&symbol=BTC/USDT", httpAddr)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		// Read one message
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("Failed to set read deadline: %v", err)
		}
		_, _, err = conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		// Close connection
		if err := conn.Close(); err != nil {
			t.Fatalf("Failed to close connection: %v", err)
		}

		// Connection should be closed without panic
		t.Log("Client disconnect handled successfully")
	})

	t.Run("InvalidURLPath", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/invalid?exchange=2&symbol=BTC/USDT", httpAddr)
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("Expected error for invalid URL path")
		}
		if resp != nil && resp.StatusCode == http.StatusOK {
			t.Error("Expected non-OK status for invalid URL path")
		}
	})

	t.Run("SpecialCharactersInSymbol", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange WebSocket")

		symbol := url.QueryEscape("BTC/USDT")
		wsURL := fmt.Sprintf("ws://%s/v1/ws/market/price?exchange=2&symbol=%s", httpAddr, symbol)

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect with escaped symbol: %v", err)
		}
		defer func() { _ = conn.Close() }()

		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("Failed to set read deadline: %v", err)
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		var priceMsg map[string]interface{}
		if err := json.Unmarshal(message, &priceMsg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		if symbol, ok := priceMsg["symbol"].(string); !ok || symbol != "BTC/USDT" {
			t.Errorf("Expected symbol 'BTC/USDT', got %v", priceMsg["symbol"])
		}
	})
}
