package bot

import (
	"context"
	"serverbot/internal/messagehandlers"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (sb *ServerBot) echoHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = messagehandlers.Echo{}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Echo mode enabled",
	})
}

func (sb *ServerBot) execHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = messagehandlers.Terminal{}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Terminal mode enabled",
	})
}

func (sb *ServerBot) inputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler.Handle(ctx, b, update, sb.serverWorker)
}

func (sb *ServerBot) interruptHandler(ctx context.Context, _ *bot.Bot, _ *models.Update) {

}

func (sb *ServerBot) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = messagehandlers.Start{}
	sb.messageHandler.Handle(ctx, b, update, sb.serverWorker)
}
