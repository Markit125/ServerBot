package messagehandlers

import (
	"context"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type ChatBot interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}

type MessageHandler interface {
	Handle(context.Context, ChatBot, *models.Update, *serverworker.ServerWorker)
}
