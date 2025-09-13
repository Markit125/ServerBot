package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	serverbot "serverbot/internal/bot"
	"serverbot/internal/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := config.New()
	if err != nil {
		panic(err)
	}

	sb, err := serverbot.New(cfg)
	if err != nil {
		panic(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	err = os.Chdir(home)
	if err != nil {
		panic(err)
	}

	fmt.Println("Bot successfully initiated")
	sb.Start(ctx)
}
