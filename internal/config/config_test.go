package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuccessfulReadConfig(t *testing.T) {
	t.Setenv("BOT_TOKEN", "valid_bot_token")
	t.Setenv("BOT_API_URL", "http://localhost:8081")

	cfg, err := New()

	assert.Nil(t, err)
	assert.Equal(t, "valid_bot_token", cfg.BotToken)
	assert.Equal(t, "http://localhost:8081", cfg.BotAPIURL)
}

func TestMissingToken(t *testing.T) {
	t.Setenv("BOT_TOKEN", "")

	cfg, err := New()

	assert.Error(t, err)
	assert.Nil(t, cfg)
}
