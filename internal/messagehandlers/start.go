package messagehandlers

import (
	"context"
	"servercommanderovertelegram/internal/serverworker"
	"servercommanderovertelegram/resources"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Start struct{}

func (t *Start) Handle(ctx context.Context, b ChatBot, update *models.Update, _ *serverworker.ServerWorker) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   handlersDescriptionText(),
	})
}

func handlersDescriptionText() string {
	return resources.HandlersDescription
}
