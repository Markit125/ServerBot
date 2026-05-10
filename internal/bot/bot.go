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
	publicAPI      *bot.Bot
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

	sb.serverWorker, err = serverworker.NewWithOptions(serverworker.Options{
		MaxDownloadBytes:    cfg.MaxDownloadBytes,
		ProgressBytesStep:   cfg.ProgressBytesStep,
		TempDir:             cfg.TempDir,
		ExecTempPattern:     cfg.ExecTempPattern,
		DownloadTempPattern: cfg.DownloadTempPattern,
	})
	if err != nil {
		return nil, err
	}

	opts := []bot.Option{
		bot.WithMiddlewares(sb.accessMiddleware),
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

	if cfg.BotAPIURL != "" {
		sb.publicAPI, err = bot.New(cfg.BotToken, bot.WithDefaultHandler(sb.inputHandler))
		if err != nil {
			return nil, err
		}
	}

	sb.registerHandlers()
	sb.messageHandler = &messagehandlers.Start{}

	return sb, err
}

func (sb *ServerBot) registerHandlers() {
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, sb.startHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/echo", bot.MatchTypePrefix, sb.echoHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/terminal", bot.MatchTypeExact, sb.terminalHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, "/get", bot.MatchTypeExact, sb.getHandler)
	sb.api.RegisterHandler(bot.HandlerTypeMessageText, messagehandlers.SignalTerminate, bot.MatchTypeExact, sb.signalTerminateHandler)
	sb.api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "get:", bot.MatchTypePrefix, sb.getSelectionHandler)
	sb.api.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return extractUploadedFile(update.Message) != nil
	}, sb.documentHandler)
}

func (sb *ServerBot) Start(ctx context.Context) {
	sb.api.Start(ctx)
}
