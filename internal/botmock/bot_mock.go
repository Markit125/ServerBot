package botmock

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type message struct {
	text      string
	id        int64
	messageID int
}

type botMock struct {
	sendMessage   func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	mu            sync.Mutex
	nextMessageID int
	sentMessages  []message
}

func (bm *botMock) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	return bm.sendMessage(ctx, params)
}

func (bm *botMock) SendDocument(context.Context, *bot.SendDocumentParams) (*models.Message, error) {
	return nil, nil
}

func (bm *botMock) EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for i, message := range bm.sentMessages {
		if message.messageID == params.MessageID {
			bm.sentMessages[i].text = params.Text
			return &models.Message{
				ID:   params.MessageID,
				Chat: models.Chat{ID: message.id},
				Text: params.Text,
			}, nil
		}
	}

	return nil, fmt.Errorf("message %d not found", params.MessageID)
}

func (bm *botMock) DeleteMessage(ctx context.Context, params *bot.DeleteMessageParams) (bool, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for i, message := range bm.sentMessages {
		if message.messageID == params.MessageID {
			bm.sentMessages = append(bm.sentMessages[:i], bm.sentMessages[i+1:]...)
			return true, nil
		}
	}

	return false, nil
}

func New(sendMessage func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)) *botMock {
	bot := &botMock{
		sendMessage:   sendMessage,
		nextMessageID: 1,
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
	bm.mu.Lock()
	defer bm.mu.Unlock()

	messageID := bm.nextMessageID
	bm.nextMessageID++
	bm.sentMessages = append(bm.sentMessages, message{
		text:      text,
		id:        id,
		messageID: messageID,
	})

	return &models.Message{
		ID:   messageID,
		Chat: models.Chat{ID: id},
		Text: text,
	}, nil
}

func (bm *botMock) SentMessages() []message {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	messages := make([]message, len(bm.sentMessages))
	copy(messages, bm.sentMessages)
	return messages
}

func (bm *botMock) ClearMessageHistory() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.sentMessages = []message{}
}
