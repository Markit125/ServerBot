package messagehandlers

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	botmock "serverbot/internal/botmock"
	"serverbot/internal/serverworker"
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
	update := NewUpdateWithMessage(NewMessageBuilder().AddText("sleep 1").Message())
	worker, err := serverworker.New()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handler := &Terminal{}

	ch := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			handler.Handle(ctx, bot, update, worker)
			ch <- 0
		}()
	}

	<-ch
	<-ch

	messages := bot.SentMessages()

	require.Len(t, messages, 3)
	assert.Equal(t, "Wait for the previous command to complete", messages[0].Text())
}

func TestTerminalHandler(t *testing.T) {
	worker, err := serverworker.New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	bot := botmock.New(nil)
	update := NewUpdateWithMessage(
		NewMessageBuilder().
			AddChatID(125).
			AddText("echo from-terminal").Message(),
	)

	handler := &Terminal{}
	handler.Handle(context.Background(), bot, update, worker)

	messages := bot.SentMessages()

	require.Len(t, messages, 2)
	assert.Equal(t, int64(125), messages[0].ID())
	assert.Equal(t, "from-terminal", messages[0].Text())
	assert.Equal(t, int64(125), messages[1].ID())
	assert.Contains(t, messages[1].Text(), tempDir)
}

func TestTerminalDocumentHandler(t *testing.T) {
	worker, err := serverworker.New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	bot := botmock.New(nil)
	update := NewUpdateWithMessage(
		&models.Message{
			Chat: models.Chat{ID: 125},
			Document: &models.Document{
				FileName: "report.txt",
			},
		},
	)

	handler := &Terminal{}
	handler.HandleDocument(context.Background(), bot, update, worker, &UploadedDocument{
		FileName: "report.txt",
		Content:  bytes.NewBufferString("document-content"),
	})

	messages := bot.SentMessages()

	require.Len(t, messages, 2)
	assert.Equal(t, filepath.Join(tempDir, "report.txt"), messages[0].Text())
	assert.Contains(t, messages[1].Text(), tempDir)

	fileContent, err := os.ReadFile(filepath.Join(tempDir, "report.txt"))
	require.NoError(t, err)
	assert.Equal(t, "document-content", string(fileContent))
}
