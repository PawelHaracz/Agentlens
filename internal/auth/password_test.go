package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/auth"
)

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  string
	}{
		{
			name:     "too short",
			password: "Ab1!xyz",
			wantErr:  "password must be at least 10 characters",
		},
		{
			name:     "no uppercase",
			password: "abcdefgh1!",
			wantErr:  "password must contain at least one uppercase letter",
		},
		{
			name:     "no lowercase",
			password: "ABCDEFGH1!",
			wantErr:  "password must contain at least one lowercase letter",
		},
		{
			name:     "no digit",
			password: "Abcdefghij!",
			wantErr:  "password must contain at least one digit",
		},
		{
			name:     "no special char",
			password: "Abcdefghi1",
			wantErr:  "password must contain at least one special character",
		},
		{
			name:     "valid password",
			password: "Str0ng!Pass",
			wantErr:  "",
		},
		{
			name:     "valid complex password",
			password: "C0mpl3x#Passw0rd!",
			wantErr:  "",
		},
		{
			name:     "exactly 10 chars valid",
			password: "Abcde12!fg",
			wantErr:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := auth.ValidatePasswordStrength(tc.password)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestHashPassword_WeakRejected(t *testing.T) {
	_, err := auth.HashPassword("short1!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validating password")
}

func TestHashPassword_StrongAccepted(t *testing.T) {
	hash, err := auth.HashPassword("Str0ng!Pass")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "Str0ng!Pass", hash)
}

func TestCheckPassword_Correct(t *testing.T) {
	password := "Str0ng!Pass"
	hash, err := auth.HashPassword(password)
	require.NoError(t, err)

	assert.True(t, auth.CheckPassword(password, hash))
}

func TestCheckPassword_Wrong(t *testing.T) {
	hash, err := auth.HashPassword("Str0ng!Pass")
	require.NoError(t, err)

	assert.False(t, auth.CheckPassword("WrongPassw0rd!", hash))
}

func TestHashPassword_DifferentHashesForSameInput(t *testing.T) {
	password := "Str0ng!Pass"
	h1, err := auth.HashPassword(password)
	require.NoError(t, err)
	h2, err := auth.HashPassword(password)
	require.NoError(t, err)

	assert.NotEqual(t, h1, h2, "bcrypt should produce different hashes due to salting")
	assert.True(t, auth.CheckPassword(password, h1))
	assert.True(t, auth.CheckPassword(password, h2))
}
