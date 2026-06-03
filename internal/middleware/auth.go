package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
)

type contextKey string

const loggerKey contextKey = "logger"

// AuthMiddleware handles authentication for HTTP requests
func AuthMiddleware(cfg *config.Config, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if not enabled
			if !cfg.Auth.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for health check endpoints
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), loggerKey, log)

			switch cfg.Auth.Type {
			case "basic":
				if !checkBasicAuth(w, r, cfg, log) {
					return
				}
			case "oauth2":
				if !checkOAuth2(w, r, cfg, log) {
					return
				}
			default:
				log.WithContext(ctx).WithField("auth_type", cfg.Auth.Type).Warnf("unknown auth type")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func checkBasicAuth(w http.ResponseWriter, r *http.Request, cfg *config.Config, log *logger.Logger) bool {
	ctx := context.WithValue(r.Context(), loggerKey, log)

	username, password, ok := r.BasicAuth()
	if !ok {
		log.WithContext(ctx).Debugf("basic auth credentials not provided")
		w.Header().Set("WWW-Authenticate", `Basic realm="CEGW"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	// Constant time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Auth.BasicUsername)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Auth.BasicPassword)) == 1

	if !usernameMatch || !passwordMatch {
		log.WithContext(ctx).WithField("username", username).Warnf("invalid basic auth credentials")
		w.Header().Set("WWW-Authenticate", `Basic realm="CEGW"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	log.WithContext(ctx).WithField("username", username).Debugf("basic auth successful")
	return true
}

func checkOAuth2(w http.ResponseWriter, r *http.Request, cfg *config.Config, log *logger.Logger) bool {
	ctx := context.WithValue(r.Context(), loggerKey, log)

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.WithContext(ctx).Debugf("authorization header not provided")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		log.WithContext(ctx).Debugf("invalid authorization header format")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	tokenString := parts[1]
	if tokenString == "" {
		log.WithContext(ctx).Debugf("bearer token is empty")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	// Parse and validate JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
		}

		// Fetch JWKS from issuer
		return getPublicKey(token, cfg.Auth.OAuth2Issuer)
	})
	if err != nil {
		log.WithContext(ctx).WithError(err).Warnf("failed to parse JWT token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	if !token.Valid {
		log.WithContext(ctx).Warnf("invalid JWT token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	// Validate claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.WithContext(ctx).Warnf("invalid JWT claims format")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	// Validate issuer
	if cfg.Auth.OAuth2Issuer != "" {
		iss, ok := claims["iss"].(string)
		if !ok || iss != cfg.Auth.OAuth2Issuer {
			log.WithContext(ctx).WithField("expected", cfg.Auth.OAuth2Issuer).WithField("got", iss).Warnf("invalid issuer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}
	}

	// Validate audience
	if cfg.Auth.OAuth2Audience != "" {
		audValid := false
		if aud, ok := claims["aud"].(string); ok {
			audValid = aud == cfg.Auth.OAuth2Audience
		} else if auds, ok := claims["aud"].([]interface{}); ok {
			for _, a := range auds {
				if audStr, ok := a.(string); ok && audStr == cfg.Auth.OAuth2Audience {
					audValid = true
					break
				}
			}
		}
		if !audValid {
			log.WithContext(ctx).WithField("expected", cfg.Auth.OAuth2Audience).Warnf("invalid audience")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}
	}

	// Validate expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			log.WithContext(ctx).Warnf("JWT token expired")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}
	}

	log.WithContext(ctx).WithField("issuer", cfg.Auth.OAuth2Issuer).Debugf("OAuth2 validation successful")
	return true
}

// getPublicKey fetches the public key from JWKS endpoint
func getPublicKey(token *jwt.Token, issuer string) (interface{}, error) {
	// Get kid from token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("kid not found in token header")
	}

	// Construct JWKS URL
	jwksURL := strings.TrimSuffix(issuer, "/") + "/.well-known/jwks.json"

	// Fetch JWKS
	resp, err := http.Get(jwksURL) // nolint:gosec // JWKS URL is constructed from trusted issuer config
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	// Parse JWKS
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
			X   string `json:"x"`
			Y   string `json:"y"`
			Crv string `json:"crv"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Find matching key
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			// Convert JWK to public key based on key type
			switch key.Kty {
			case "RSA":
				return jwt.ParseRSAPublicKeyFromPEM([]byte(fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----", key.N)))
			case "EC":
				return jwt.ParseECPublicKeyFromPEM([]byte(fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----", key.X)))
			default:
				return nil, fmt.Errorf("unsupported key type: %s", key.Kty)
			}
		}
	}

	return nil, fmt.Errorf("key with kid %s not found in JWKS", kid)
}
