package messagehandlers

import (
	"context"
	botmock "serverbot/internal/botMock"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoHandler(t *testing.T) {
	tests := []struct {
		name            string
		messageSend     *models.Message
		expectedMessage string
		expectedID      int64
	}{
		{
			name: "echo normal message",
			messageSend: &models.Message{
				Text: "repeat it",
				Chat: models.Chat{
					ID: 125,
				},
			},
			expectedMessage: "repeat it",
			expectedID:      int64(125),
		},
		{
			name: "echo empty message",
			messageSend: &models.Message{
				Chat: models.Chat{
					ID: 1,
				},
			},
			expectedMessage: "",
			expectedID:      int64(1),
		},
	}

	echoHandler := Echo{}
	bot := botmock.New(nil)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer bot.ClearMessageHistory()

			echoHandler.Handle(context.Background(), bot, &models.Update{
				Message: test.messageSend,
			}, nil)

			messages := bot.SentMessages()
			require.Equal(t, 1, len(messages))
			assert.Equal(t, test.expectedID, messages[0].ID())
			assert.Equal(t, test.expectedMessage, messages[0].Text())
		})
	}
}

func TestStartHandler(t *testing.T) {
	bot := botmock.New(nil)
	startHandler := Start{}

	startHandler.Handle(context.Background(), bot, &models.Update{
		Message: &models.Message{
			Text: "any",
			Chat: models.Chat{
				ID: 125,
			},
		},
	}, nil)

	messages := bot.SentMessages()

	require.Equal(t, 1, len(messages))
	assert.Equal(t, int64(125), messages[0].ID())
	assert.Equal(t, "default handlers description", messages[0].Text())
}

func TestSwitchHandlers(t *testing.T) {
	bot := botmock.New(nil)
	update := &models.Update{
		Message: &models.Message{
			Text: "some text",
		},
	}

	var handler MessageHandler
	handler = Start{}
	handler.Handle(context.Background(), bot, update, nil)
	handler = Echo{}
	handler.Handle(context.Background(), bot, update, nil)

	messages := bot.SentMessages()

	require.Equal(t, 2, len(messages))
	assert.Equal(t, messages[0].Text(), "default handlers description")
	assert.Equal(t, messages[1].Text(), "some text")

}
