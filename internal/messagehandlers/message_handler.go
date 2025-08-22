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

type messageBuilder struct {
	message *models.Message
}

func NewMessageBuilder() *messageBuilder {
	return &messageBuilder{
		message: &models.Message{},
	}
}

func (mb *messageBuilder) AddChatID(id int64) *messageBuilder {
	mb.message.Chat = models.Chat{
		ID: id,
	}

	return mb
}

func (mb *messageBuilder) AddText(text string) *messageBuilder {
	mb.message.Text = text
	return mb
}

func (mb *messageBuilder) Message() *models.Message {
	return mb.message
}

func NewUpdateWithMessage(message *models.Message) *models.Update {
	return &models.Update{
		Message: message,
	}
}
