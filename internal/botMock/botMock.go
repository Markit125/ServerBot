package botmock

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type message struct {
	text string
	id   int64
}

type botMock struct {
	sendMessage  func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	sentMessages []message
}

func (bm *botMock) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	return bm.sendMessage(ctx, params)
}

func New(sendMessage func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)) *botMock {
	bot := &botMock{
		sendMessage: sendMessage,
	}
	if bot.sendMessage == nil {
		bot.sendMessage = bot.defaultSendMessage
	}

	return bot
}

func (m *message) Text() string {
	return m.text
}

func (m *message) ID() int64 {
	return m.id
}

func (bm *botMock) defaultSendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	text := params.Text
	id, ok := params.ChatID.(int64)
	if !ok {
		return nil, fmt.Errorf("cannot parse %v to int64", params.ChatID)
	}
	bm.sentMessages = append(bm.sentMessages, message{
		text: text,
		id:   id,
	})

	return nil, nil
}

func (bm *botMock) SentMessages() []message {
	return bm.sentMessages
}

func (bm *botMock) ClearMessageHistory() {
	bm.sentMessages = []message{}
}
