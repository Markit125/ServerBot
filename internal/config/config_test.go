package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuccessfulReadConfig(t *testing.T) {
	t.Setenv("BOT_TOKEN", "valid_bot_token")

	cfg, err := New()

	assert.Nil(t, err)
	assert.Equal(t, cfg.BotToken, "valid_bot_token")
}

func TestMissingToken(t *testing.T) {
	t.Setenv("BOT_TOKEN", "")

	cfg, err := New()

	assert.Error(t, err)
	assert.Nil(t, cfg)
}
