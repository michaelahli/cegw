package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

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

	token := parts[1]
	if token == "" {
		log.WithContext(ctx).Debugf("bearer token is empty")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	// TODO: Implement JWT validation with issuer and audience
	// For now, just log that OAuth2 is configured
	log.WithContext(ctx).
		WithField("issuer", cfg.Auth.OAuth2Issuer).
		WithField("audience", cfg.Auth.OAuth2Audience).
		Warnf("OAuth2 validation not yet implemented - accepting all tokens")

	return true
}
