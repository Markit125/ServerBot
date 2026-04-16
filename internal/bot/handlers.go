package bot

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"serverbot/internal/messagehandlers"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type uploadedFile struct {
	FileID   string
	FileName string
	Kind     string
}

func (sb *ServerBot) echoHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = &messagehandlers.Echo{}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Echo mode enabled",
	})
}

func (sb *ServerBot) terminalHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = &messagehandlers.Terminal{}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Terminal mode enabled",
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.serverWorker.TerminalAsk(),
	})
}

func (sb *ServerBot) inputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler.Handle(ctx, b, update, sb.serverWorker)
}

func (sb *ServerBot) documentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	uploadedFile := extractUploadedFile(update.Message)
	if uploadedFile == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No uploadable file found in message",
		})
		return
	}

	documentHandler, ok := sb.messageHandler.(messagehandlers.DocumentHandler)
	if !ok {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("%s upload is available only in terminal mode", uploadedFile.Kind),
		})
		return
	}

	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: uploadedFile.FileID,
	})
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   formatUploadedFileError(err),
		})
		return
	}

	if canUseTelegramLocalFile(file.FilePath) {
		documentHandler.HandleDocument(ctx, b, update, sb.serverWorker, &messagehandlers.UploadedDocument{
			FileName:  uploadedFile.FileName,
			LocalPath: file.FilePath,
		})
		return
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, b.FileDownloadLink(file), nil)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("Failed to create download request: %v", err),
		})
		return
	}

	response, err := sb.httpClient.Do(request)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("Failed to download file: %v", err),
		})
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("Failed to download file: unexpected status %d", response.StatusCode),
		})
		return
	}

	documentHandler.HandleDocument(ctx, b, update, sb.serverWorker, &messagehandlers.UploadedDocument{
		FileName: uploadedFile.FileName,
		Content:  response.Body,
	})
}

func (sb *ServerBot) interruptHandler(ctx context.Context, _ *bot.Bot, _ *models.Update) {

}

func (sb *ServerBot) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = &messagehandlers.Start{}
	sb.messageHandler.Handle(ctx, b, update, sb.serverWorker)
}

func extractUploadedFile(message *models.Message) *uploadedFile {
	if message == nil {
		return nil
	}

	if message.Document != nil {
		return &uploadedFile{
			FileID:   message.Document.FileID,
			FileName: message.Document.FileName,
			Kind:     "Document",
		}
	}

	if message.Audio != nil {
		return &uploadedFile{
			FileID:   message.Audio.FileID,
			FileName: message.Audio.FileName,
			Kind:     "Audio",
		}
	}

	if message.Video != nil {
		return &uploadedFile{
			FileID:   message.Video.FileID,
			FileName: message.Video.FileName,
			Kind:     "Video",
		}
	}

	return nil
}

func formatUploadedFileError(err error) string {
	errorText := err.Error()
	if strings.Contains(strings.ToLower(errorText), "file is too big") {
		return "File is too big for direct download via Telegram Bot API. Configure a local telegram-bot-api server and set BOT_API_URL to support large files."
	}

	return fmt.Sprintf("Failed to get file: %v", err)
}

func canUseTelegramLocalFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !fileInfo.IsDir()
}
