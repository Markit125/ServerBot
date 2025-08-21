package messagehandlers

import (
	"context"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Echo struct{}

func (t Echo) Handle(ctx context.Context, b ChatBot, update *models.Update, _ *serverworker.ServerWorker) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   update.Message.Text,
	})
}
