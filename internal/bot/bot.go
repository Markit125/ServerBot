package bot

import (
	"context"
	"net/http"
	"serverbot/internal/config"
	"serverbot/internal/messagehandlers"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type ServerBot struct {
	serverWorker   *serverworker.ServerWorker
	api            *bot.Bot
	httpClient     *http.Client
	config         *config.Config
	messageHandler messagehandlers.MessageHandler
}

func New(cfg *config.Config) (*ServerBot, error) {
	sb := &ServerBot{
		config:     cfg,
		httpClient: http.DefaultClient,
	}

	var err error

	sb.serverWorker, err = serverworker.New()
	if err != nil {
		return nil, err
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(sb.inputHandler),
	}
	if cfg.BotAPIURL != "" {
		opts = append(opts, bot.WithServerURL(cfg.BotAPIURL))
	}

	b, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		return nil, err
	}

	sb.api = b

	sb.registerHandlers()
	sb.messageHandler = &messagehandlers.Start{}

	return sb, err
}

func (sb *ServerBot) registerHandlers() {
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, sb.startHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/echo", bot.MatchTypePrefix, sb.echoHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/terminal", bot.MatchTypeExact, sb.terminalHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, messagehandlers.INTERRUPT, bot.MatchTypeExact, sb.interruptHandler)
	sb.api.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return extractUploadedFile(update.Message) != nil
	}, sb.documentHandler)
}

func (sb *ServerBot) Start(ctx context.Context) {
	sb.api.Start(ctx)
}
