package bot

import (
	"serverbot/internal/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	cfg = &config.Config{
		BotToken: "some_token",
	}
)

func TestSuccessBotCreation(t *testing.T) {
	sb, err := New(cfg)

	assert.Nil(t, err)
	assert.NotNil(t, sb)
	assert.NotNil(t, sb.api)
}
