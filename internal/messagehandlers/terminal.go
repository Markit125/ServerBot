package messagehandlers

import (
	"context"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Terminal struct{}

const INTERRUPT = "/c"

func (t Terminal) Handle(ctx context.Context, b ChatBot, update *models.Update, serverWorker *serverworker.ServerWorker) {
	result, path := serverWorker.Exec(ctx, update.Message.Text)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   result,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   path,
	})
}
