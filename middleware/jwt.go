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
}

// JWTAuth returns Gin middleware that validates Bearer tokens.
// On success, it sets "account_id" and "session_id" in the Gin context.
func JWTAuth(cfg JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := extractAndValidate(c.GetHeader("Authorization"), cfg.Secret)
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
func ParseToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
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

		if token != serviceSecret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid service token",
			})
			return
		}

		c.Next()
	}
}

func extractAndValidate(authHeader string, secret []byte) (*Claims, error) {
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, fmt.Errorf("invalid authorization format, expected: Bearer <token>")
	}

	return ParseToken(parts[1], secret)
}
