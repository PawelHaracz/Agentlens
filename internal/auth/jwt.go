package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	Secret        string
	Expiration    time.Duration
	RefreshWindow time.Duration
}

// Claims represents the JWT claims for an authenticated user.
type Claims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	RoleID      string   `json:"role_id"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token generation and validation.
type JWTService struct {
	config JWTConfig
}

// NewJWTService creates a new JWTService with the given configuration.
// If the secret is empty, a random 64-byte secret is generated and a warning is logged.
func NewJWTService(cfg JWTConfig) *JWTService {
	if cfg.Secret == "" {
		b := make([]byte, 64)
		if _, err := rand.Read(b); err != nil {
			panic(fmt.Sprintf("failed to generate random JWT secret: %v", err))
		}
		cfg.Secret = hex.EncodeToString(b)
		slog.Warn("no JWT secret configured, using auto-generated secret (tokens will not survive restarts)")
	}
	if cfg.Expiration == 0 {
		cfg.Expiration = 24 * time.Hour
	}
	if cfg.RefreshWindow == 0 {
		cfg.RefreshWindow = 30 * time.Minute
	}
	return &JWTService{config: cfg}
}

// GenerateToken creates a signed JWT token for the given user and role.
func (s *JWTService) GenerateToken(user *model.User, role *model.Role) (string, error) {
	now := time.Now()
	var permissions []string
	if role != nil {
		permissions = []string(role.Permissions)
	}

	claims := Claims{
		UserID:      user.ID,
		Username:    user.Username,
		RoleID:      user.RoleID,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.Expiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "agentlens",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.config.Secret))
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and validates a JWT token string, returning the claims.
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.config.Secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// RefreshWindow returns the refresh window duration.
func (s *JWTService) RefreshWindow() time.Duration {
	return s.config.RefreshWindow
}
