package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

const passwordLength = 20

var (
	upperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerChars   = "abcdefghijklmnopqrstuvwxyz"
	digitChars   = "0123456789"
	specialChars = "!@#$%^&*()"
	allChars     = upperChars + lowerChars + digitChars + specialChars
)

// BootstrapAdmin creates the initial admin user if no users exist.
// Returns the generated password if a user was created, or empty string if users already exist.
func BootstrapAdmin(ctx context.Context, userStore *store.UserStore) (string, error) {
	count, err := userStore.Count(ctx)
	if err != nil {
		return "", fmt.Errorf("counting users: %w", err)
	}
	if count > 0 {
		return "", nil
	}

	password, err := generatePassword()
	if err != nil {
		return "", fmt.Errorf("generating password: %w", err)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hashing admin password: %w", err)
	}

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     "admin",
		DisplayName:  "Administrator",
		PasswordHash: hash,
		RoleID:       "role-admin",
		IsActive:     true,
	}

	if err := userStore.Create(ctx, user); err != nil {
		return "", fmt.Errorf("creating admin user: %w", err)
	}

	return password, nil
}

// generatePassword creates a cryptographically random password that satisfies
// ValidatePasswordStrength: at least 1 upper, 1 lower, 1 digit, 1 special.
func generatePassword() (string, error) {
	buf := make([]byte, passwordLength)

	// Place one required character from each category at random positions.
	required := []string{upperChars, lowerChars, digitChars, specialChars}
	used := make(map[int]bool)
	for _, charset := range required {
		pos, err := randomInt(passwordLength)
		if err != nil {
			return "", err
		}
		for used[pos] {
			pos = (pos + 1) % passwordLength
		}
		used[pos] = true
		ch, err := randomChar(charset)
		if err != nil {
			return "", err
		}
		buf[pos] = ch
	}

	// Fill remaining positions from the full character set.
	for i := 0; i < passwordLength; i++ {
		if used[i] {
			continue
		}
		ch, err := randomChar(allChars)
		if err != nil {
			return "", err
		}
		buf[i] = ch
	}

	return string(buf), nil
}

func randomInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("generating random int: %w", err)
	}
	return int(n.Int64()), nil
}

func randomChar(charset string) (byte, error) {
	idx, err := randomInt(len(charset))
	if err != nil {
		return 0, err
	}
	return charset[idx], nil
}
