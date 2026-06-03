// Package middleware provides shared authentication middleware for BananaKit services.
//
// This is the piece that every service imports. It handles JWT parsing and
// signature verification. It does NOT handle session revocation — that's
// BananAuth's responsibility.
//
// Usage in any service:
//
//	import "github.com/bananalabs-oss/potassium/middleware"
//
//	router.Use(middleware.JWTAuth(middleware.JWTConfig{
//	    Secret: []byte("your-jwt-secret"),
//	}))
package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims issued by BananAuth.
// Every service in the ecosystem uses this structure.
type Claims struct {
	jwt.RegisteredClaims
	AccountID string `json:"account_id"`
	SessionID string `json:"session_id"`
}

// JWTConfig holds configuration for the JWT middleware.
type JWTConfig struct {
	// Secret is the shared JWT signing key (must match BananAuth's JWT_SECRET).
	Secret []byte
	// Issuer, when non-empty, is the expected token issuer (iss claim).
	// Tokens whose iss does not match are rejected. Leave empty to skip.
	Issuer string
	// Audience, when non-empty, is this service's expected audience (aud claim).
	// Tokens that do not list this audience are rejected. Leave empty to skip.
	// Setting this per-service prevents a token minted for one service from
	// being replayed against another under a shared HMAC secret.
	Audience string
}

// newParser builds a JWT parser that pins the HMAC algorithm to HS256,
// requires the exp claim (tokens without exp are rejected rather than
// treated as never-expiring), and binds issuer/audience when configured.
func newParser(issuer, audience string) *jwt.Parser {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	}
	if issuer != "" {
		opts = append(opts, jwt.WithIssuer(issuer))
	}
	if audience != "" {
		opts = append(opts, jwt.WithAudience(audience))
	}
	return jwt.NewParser(opts...)
}

// JWTAuth returns Gin middleware that validates Bearer tokens.
// On success, it sets "account_id" and "session_id" in the Gin context.
func JWTAuth(cfg JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := extractAndValidate(c.GetHeader("Authorization"), cfg.Secret, cfg.Issuer, cfg.Audience)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.Set("account_id", claims.AccountID)
		c.Set("session_id", claims.SessionID)
		c.Next()
	}
}

// ParseToken validates a raw JWT string and returns the claims.
// Useful when you need claims without the Gin middleware (e.g., WebSocket upgrade).
//
// The token MUST carry an exp claim and be signed with HS256; tokens lacking
// exp are rejected rather than treated as never-expiring. To additionally bind
// the issuer/audience, use ParseTokenWithConfig.
func ParseToken(tokenString string, secret []byte) (*Claims, error) {
	return ParseTokenWithConfig(tokenString, JWTConfig{Secret: secret})
}

// ParseTokenWithConfig validates a raw JWT string against the given config,
// enforcing exp-required, HS256, and (when set) the configured issuer/audience.
func ParseTokenWithConfig(tokenString string, cfg JWTConfig) (*Claims, error) {
	parser := newParser(cfg.Issuer, cfg.Audience)
	token, err := parser.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return cfg.Secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// ServiceAuth returns Gin middleware for service-to-service authentication.
// Services pass a shared secret via the X-Service-Token header.
func ServiceAuth(serviceSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Service-Token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing service token",
			})
			return
		}

		// Constant-time compare prevents timing side-channel attacks
		// that could let a network-adjacent attacker recover the
		// service secret one byte at a time.
		if subtle.ConstantTimeCompare([]byte(token), []byte(serviceSecret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid service token",
			})
			return
		}

		c.Next()
	}
}

func extractAndValidate(authHeader string, secret []byte, issuer, audience string) (*Claims, error) {
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, fmt.Errorf("invalid authorization format, expected: Bearer <token>")
	}

	return ParseTokenWithConfig(parts[1], JWTConfig{Secret: secret, Issuer: issuer, Audience: audience})
}
