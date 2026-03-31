package auth

import (
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the bcrypt hashing cost factor.
const BcryptCost = 12

// MinPasswordLength is the minimum number of characters required in a password.
const MinPasswordLength = 10

// HashPassword validates the plaintext password strength, then returns the bcrypt hash.
func HashPassword(plaintext string) (string, error) {
	if err := ValidatePasswordStrength(plaintext); err != nil {
		return "", fmt.Errorf("validating password: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword returns true if the plaintext matches the bcrypt hash.
func CheckPassword(plaintext, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

// ValidatePasswordStrength checks that a password meets complexity requirements:
// at least 10 characters, 1 uppercase, 1 lowercase, 1 digit, and 1 special character.
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}
	return nil
}
