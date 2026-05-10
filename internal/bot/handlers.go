package bot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"servercommanderovertelegram/internal/messagehandlers"
	"servercommanderovertelegram/internal/serverworker"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type uploadedFile struct {
	FileID   string
	FileName string
	Kind     string
}

func (sb *ServerCommanderOverTelegram) echoHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = &messagehandlers.Echo{}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Echo mode enabled",
	})
}

func (sb *ServerCommanderOverTelegram) terminalHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.enableTerminalMode(ctx, b, update)
}

func (sb *ServerCommanderOverTelegram) enableTerminalMode(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID, ok := chatIDFromUpdate(update)
	if !ok {
		log.Printf("warning: cannot enable terminal mode: update has no chat")
		return
	}

	sb.messageHandler = &messagehandlers.Terminal{}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Terminal mode enabled",
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.serverWorker.TerminalAsk(),
	})
}

func chatIDFromUpdate(update *models.Update) (any, bool) {
	if update == nil {
		return nil, false
	}
	if update.Message != nil {
		return update.Message.Chat.ID, true
	}
	if update.CallbackQuery != nil && update.CallbackQuery.Message.Message != nil {
		return update.CallbackQuery.Message.Message.Chat.ID, true
	}

	return nil, false
}

func (sb *ServerCommanderOverTelegram) getHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sb.messageHandler = &messagehandlers.Get{}
	sb.messageHandler.Handle(ctx, b, update, sb.serverWorker)
}

func (sb *ServerCommanderOverTelegram) inputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil {
		return
	}

	sb.messageHandler.Handle(ctx, b, update, sb.serverWorker)
}

func (sb *ServerCommanderOverTelegram) getSelectionHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	getHandler, ok := sb.messageHandler.(*messagehandlers.Get)
	if !ok {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Run /get first",
			ShowAlert:       false,
		})
		return
	}

	entry, err := getHandler.ResolveSelection(update.CallbackQuery.Data)
	if err != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            err.Error(),
			ShowAlert:       false,
		})
		return
	}

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "Preparing download",
		ShowAlert:       false,
	})

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	go sb.downloadSelection(context.Background(), b, update, chatID, entry.Name)
}

func (sb *ServerCommanderOverTelegram) downloadSelection(ctx context.Context, b *bot.Bot, update *models.Update, chatID any, name string) {
	progressReporter := newGetProgressReporter(ctx, b, chatID, name, sb.config.ProgressQueueSize, sb.config.ProgressEditInterval, sb.config.ProgressMessageMaxChars)
	defer progressReporter.Close()

	progressReporter.Event(serverworker.DownloadProgress{Stage: "request_started", Name: name})

	preparedDownload, err := sb.serverWorker.PrepareDownloadWithProgress(name, progressReporter.Event)
	if err != nil {
		progressReporter.Event(serverworker.DownloadProgress{Stage: "failed", Name: fmt.Sprintf("prepare download: %v", err)})
		return
	}
	downloadSucceeded := false
	defer func() {
		if err := preparedDownload.Cleanup(); err != nil {
			if downloadSucceeded {
				progressReporter.Event(serverworker.DownloadProgress{Stage: "cleanup_failed", Name: preparedDownload.FileName, Path: preparedDownload.Path})
			}
			log.Printf("[GET] file=%q stage=cleanup_failed path=%q error=%v", preparedDownload.FileName, preparedDownload.Path, err)
			return
		}
		if downloadSucceeded {
			progressReporter.Event(serverworker.DownloadProgress{Stage: "cleanup_done", Name: preparedDownload.FileName, Path: preparedDownload.Path})
		}
	}()

	progressReporter.Event(serverworker.DownloadProgress{Stage: "open_started", Name: preparedDownload.FileName, Path: preparedDownload.Path, TotalBytes: preparedDownload.Size})
	file, err := os.Open(preparedDownload.Path)
	if err != nil {
		progressReporter.Event(serverworker.DownloadProgress{Stage: "failed", Name: fmt.Sprintf("open prepared file: %v", err), Path: preparedDownload.Path})
		return
	}
	defer file.Close()
	progressReporter.Event(serverworker.DownloadProgress{Stage: "open_done", Name: preparedDownload.FileName, Path: preparedDownload.Path, TotalBytes: preparedDownload.Size})

	sendCtx, cancelSend := context.WithTimeout(ctx, sb.config.SendTimeout)
	defer cancelSend()

	if sb.config.BotAPIURL != "" {
		progressReporter.Event(serverworker.DownloadProgress{Stage: "telegram_local_path_send_started", Name: preparedDownload.FileName, Path: preparedDownload.Path, TotalBytes: preparedDownload.Size})
		if err := sb.sendDocumentLocalPath(sendCtx, chatID, preparedDownload.FileName, preparedDownload.Path); err != nil {
			progressReporter.Event(serverworker.DownloadProgress{Stage: "failed", Name: fmt.Sprintf("send %s: %v", preparedDownload.FileName, err), Path: preparedDownload.Path})
			return
		}
		progressReporter.Event(serverworker.DownloadProgress{Stage: "telegram_local_path_send_done", Name: preparedDownload.FileName, Path: preparedDownload.Path, TotalBytes: preparedDownload.Size})
		progressReporter.Event(serverworker.DownloadProgress{Stage: "send_done", Name: preparedDownload.FileName, Path: preparedDownload.Path, TotalBytes: preparedDownload.Size})
		downloadSucceeded = true
		if sb.config.DeleteProgressOnSuccess {
			progressReporter.DeleteOnClose()
		}
		if sb.config.AutoTerminalAfterGet {
			sb.enableTerminalMode(ctx, b, update)
		}
		return
	}

	var lastRead atomic.Int64
	stopUploadWatch := make(chan struct{})
	go logUploadStalls(stopUploadWatch, progressReporter, preparedDownload.FileName, preparedDownload.Size, &lastRead, sb.config.UploadStallInterval)
	defer close(stopUploadWatch)

	progressReporter.Event(serverworker.DownloadProgress{Stage: "telegram_request_started", Name: preparedDownload.FileName, Path: preparedDownload.Path, TotalBytes: preparedDownload.Size})
	err = sb.sendDocumentStreaming(sendCtx, chatID, preparedDownload.FileName, &progressReader{
		reader:      file,
		reporter:    progressReporter,
		stage:       "telegram_upload_bytes",
		name:        preparedDownload.FileName,
		totalBytes:  preparedDownload.Size,
		reportEvery: sb.config.ProgressBytesStep,
		lastRead:    &lastRead,
	})
	if err != nil {
		progressReporter.Event(serverworker.DownloadProgress{Stage: "failed", Name: fmt.Sprintf("send %s: %v", preparedDownload.FileName, err), Path: preparedDownload.Path})
		return
	}
	progressReporter.Event(serverworker.DownloadProgress{Stage: "send_done", Name: preparedDownload.FileName, Path: preparedDownload.Path, TotalBytes: preparedDownload.Size})
	downloadSucceeded = true
	if sb.config.DeleteProgressOnSuccess {
		progressReporter.DeleteOnClose()
	}
	if sb.config.AutoTerminalAfterGet {
		sb.enableTerminalMode(ctx, b, update)
	}
}

func logUploadStalls(done <-chan struct{}, reporter *getProgressReporter, name string, totalBytes int64, lastRead *atomic.Int64, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var previousBytes int64 = -1
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			currentBytes := lastRead.Load()
			if currentBytes == previousBytes {
				reporter.Event(serverworker.DownloadProgress{
					Stage:      "telegram_upload_waiting",
					Name:       name,
					Bytes:      currentBytes,
					TotalBytes: totalBytes,
				})
			}
			previousBytes = currentBytes
		}
	}
}

func (sb *ServerCommanderOverTelegram) documentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
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

	fileAPI := b
	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: uploadedFile.FileID,
	})
	if err != nil && shouldRetryGetFileWithPublicAPI(err, sb.publicAPI) {
		log.Printf("warning: local Bot API getFile failed for %s %q, retrying via public Bot API: %v", strings.ToLower(uploadedFile.Kind), uploadedFile.FileName, err)
		file, err = sb.publicAPI.GetFile(ctx, &bot.GetFileParams{
			FileID: uploadedFile.FileID,
		})
		if err == nil {
			fileAPI = sb.publicAPI
		}
	}
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

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fileAPI.FileDownloadLink(file), nil)
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

func (sb *ServerCommanderOverTelegram) interruptHandler(ctx context.Context, _ *bot.Bot, _ *models.Update) {

}

func (sb *ServerCommanderOverTelegram) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
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

func shouldRetryGetFileWithPublicAPI(err error, publicAPI *bot.Bot) bool {
	if err == nil || publicAPI == nil {
		return false
	}

	errorText := strings.ToLower(err.Error())
	return strings.Contains(errorText, "wrong file_id") || strings.Contains(errorText, "temporarily unavailable")
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
