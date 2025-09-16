package messagehandlers

import (
	"context"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Terminal struct {
	commandExecuting bool
}

const INTERRUPT = "/c"

func (t *Terminal) Handle(ctx context.Context, b ChatBot, update *models.Update, serverWorker *serverworker.ServerWorker) {
	if update.Message.Audio != nil || update.Message.Document != nil || update.Message.Video != nil {
		handler := &Upload{}
		handler.Handle(ctx, b, update, serverWorker)
		return
	}

	if t.commandExecuting {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Wait for the previous command to complete",
		})

		return
	}

	t.commandExecuting = true
	result, path := serverWorker.Exec(ctx, update.Message.Text)
	t.commandExecuting = false

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   result,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   path,
	})
}
