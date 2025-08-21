package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken string
}

func New() (*Config, error) {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, errors.New("could not load bot token from enviroment")
	}

	return &Config{
		BotToken: token,
	}, nil
}
