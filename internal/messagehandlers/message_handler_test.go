package messagehandlers

import (
	"context"
	botmock "serverbot/internal/botMock"
	"testing"
	"time"

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
			messageSend: NewMessageBuilder().
				AddChatID(125).
				AddText("repeat it").Message(),
			expectedMessage: "repeat it",
			expectedID:      int64(125),
		},
		{
			name: "echo empty message",
			messageSend: NewMessageBuilder().
				AddChatID(1).Message(),
			expectedMessage: "",
			expectedID:      int64(1),
		},
	}

	echoHandler := Echo{}
	bot := botmock.New(nil)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer bot.ClearMessageHistory()

			echoHandler.Handle(context.Background(), bot, NewUpdateWithMessage(test.messageSend), nil)

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

	startHandler.Handle(context.Background(), bot, NewUpdateWithMessage(
		NewMessageBuilder().
			AddChatID(125).
			AddText("any").Message()),
		nil)

	messages := bot.SentMessages()

	require.Equal(t, 1, len(messages))
	assert.Equal(t, int64(125), messages[0].ID())
	assert.Contains(t, messages[0].Text(), "/start")
}

func TestSwitchHandlers(t *testing.T) {
	bot := botmock.New(nil)
	update := NewUpdateWithMessage(NewMessageBuilder().AddText("some text").Message())

	var handler MessageHandler
	handler = &Start{}
	handler.Handle(context.Background(), bot, update, nil)
	handler = &Echo{}
	handler.Handle(context.Background(), bot, update, nil)

	messages := bot.SentMessages()

	require.Equal(t, 2, len(messages))
	assert.Contains(t, messages[0].Text(), "/start")
	assert.Equal(t, "some text", messages[1].Text())
}

func TestExecuteOnlyOneCommandAtTheSameTime(t *testing.T) {
	bot := botmock.New(nil)
	update := NewUpdateWithMessage(NewMessageBuilder().AddText("sleep 15").Message())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	handler := &Terminal{}

	ch := make(chan int, 1)
	for i := 0; i < 2; i++ {
		go func() {
			handler.Handle(ctx, bot, update, nil)
			ch <- 0
		}()
	}

	<-ch

	messages := bot.SentMessages()

	assert.Equal(t, 1, len(messages))
	assert.Equal(t, "Wait for the previous command to complete", messages[0].Text())
}
