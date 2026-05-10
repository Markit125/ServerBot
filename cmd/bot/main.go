package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	servercommanderovertelegram "servercommanderovertelegram/internal/bot"
	"servercommanderovertelegram/internal/config"
	"syscall"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("fatal panic: %v", recovered)
			os.Exit(1)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	sb, err := servercommanderovertelegram.New(cfg)
	if err != nil {
		log.Fatalf("could not create bot: %v", err)
	}

	log.Println("bot successfully initiated")
	sb.Start(ctx)
	log.Println("bot stopped")
}
