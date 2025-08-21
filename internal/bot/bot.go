package bot

import (
	"context"
	"serverbot/internal/config"
	"serverbot/internal/messagehandlers"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
)

type ServerBot struct {
	serverWorker   *serverworker.ServerWorker
	api            *bot.Bot
	config         *config.Config
	messageHandler messagehandlers.MessageHandler
}

func New(cfg *config.Config) (*ServerBot, error) {
	sb := &ServerBot{
		config: cfg,
	}

	var err error

	sb.serverWorker, err = serverworker.New()
	if err != nil {
		return nil, err
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(sb.inputHandler),
	}

	b, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		return nil, err
	}

	sb.api = b

	sb.registerHandlers()
	messagehandlers.ReadHandlersDescriptionText()
	sb.messageHandler = messagehandlers.Start{}

	return sb, err
}

func (sb *ServerBot) registerHandlers() {
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, sb.startHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/echo", bot.MatchTypePrefix, sb.echoHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/terminal", bot.MatchTypeExact, sb.execHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, messagehandlers.INTERRUPT, bot.MatchTypeExact, sb.interruptHandler)
}

func (sb *ServerBot) Start(ctx context.Context) {
	sb.api.Start(ctx)
}
