package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jlry-dev/whirl/internal/config"
)

const validJWTKey = "test-key-at-least-32-bytes-long-1234"

func Test_Load_ServerAddress(t *testing.T) {
	t.Setenv("FRONTEND_ADDRESS", "https://whirl.example")
	t.Setenv("JWT_KEY", validJWTKey)

	t.Run("defaults to :8080", func(t *testing.T) {
		t.Setenv("SERVER_ADDRESS", "")

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, config.DefaultServerAddress, cfg.Address)
		assert.Equal(t, ":8080", cfg.Address)
	})

	t.Run("keeps an explicit address and trims spaces", func(t *testing.T) {
		t.Setenv("SERVER_ADDRESS", "  127.0.0.1:9090  ")

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:9090", cfg.Address)
	})
}

func Test_Load_RequiresValidJWTKey(t *testing.T) {
	t.Setenv("FRONTEND_ADDRESS", "https://whirl.example")
	t.Setenv("SERVER_ADDRESS", ":8080")

	t.Run("missing key", func(t *testing.T) {
		t.Setenv("JWT_KEY", "")

		_, err := config.Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JWT_KEY is required")
	})

	t.Run("too short", func(t *testing.T) {
		t.Setenv("JWT_KEY", "short-key")

		_, err := config.Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 32 bytes")
	})

	t.Run("valid key", func(t *testing.T) {
		t.Setenv("JWT_KEY", validJWTKey)

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, []byte(validJWTKey), cfg.JWTKey)
	})
}

func Test_InitDB_RejectsEmptyDSN(t *testing.T) {
	t.Setenv("DATABASE_CONN_STR", "   ")

	pool, err := config.InitDB()
	require.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "DATABASE_CONN_STR is required")
}
