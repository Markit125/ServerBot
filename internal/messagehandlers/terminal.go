package messagehandlers

import (
	"context"
	"fmt"
	"serverbot/internal/serverworker"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Terminal struct {
	mu               sync.Mutex
	commandExecuting bool
	cancelCommand    context.CancelFunc
}

const SignalTerminate = "/sigterm"
const executingUpdateInterval = time.Second

func (t *Terminal) Handle(ctx context.Context, b ChatBot, update *models.Update, serverWorker *serverworker.ServerWorker) {
	if update == nil || update.Message == nil {
		return
	}

	commandCtx, cancel := context.WithCancel(ctx)
	if !t.beginCommand(ctx, b, update.Message.Chat.ID, cancel) {
		cancel()
		return
	}
	defer cancel()
	defer t.endCommand()

	executingMessage, _ := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Executing...",
	})
	stopExecutingUpdates := t.startExecutingUpdates(ctx, b, update.Message.Chat.ID, executingMessage)

	result, path := serverWorker.Exec(commandCtx, update.Message.Text)

	stopExecutingUpdates()
	if executingMessage != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    update.Message.Chat.ID,
			MessageID: executingMessage.ID,
		})
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   result,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   path,
	})
}

func (t *Terminal) startExecutingUpdates(ctx context.Context, b ChatBot, chatID int64, message *models.Message) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})

	if message == nil {
		close(stopped)
		return func() {}
	}

	startedAt := time.Now()
	go func() {
		defer close(stopped)

		ticker := time.NewTicker(executingUpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
					ChatID:    chatID,
					MessageID: message.ID,
					Text:      "Executing... (" + formatExecutingElapsed(time.Since(startedAt)) + ")",
				})
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
	}
}

func formatExecutingElapsed(elapsed time.Duration) string {
	totalSeconds := int(elapsed.Round(time.Second).Seconds())
	if totalSeconds < 1 {
		totalSeconds = 1
	}

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %02dm %02ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func (t *Terminal) HandleDocument(ctx context.Context, b ChatBot, update *models.Update, serverWorker *serverworker.ServerWorker, document *UploadedDocument) {
	if !t.beginCommand(ctx, b, update.Message.Chat.ID, nil) {
		return
	}
	defer t.endCommand()

	var (
		savedPath string
		err       error
	)

	switch {
	case document.LocalPath != "":
		savedPath, err = serverWorker.SaveTelegramLocalFile(document.LocalPath, document.FileName)
	case document.Content != nil:
		savedPath, err = serverWorker.SaveUploadedFile(document.FileName, document.Content)
	default:
		err = fmt.Errorf("no uploaded file content provided")
	}

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   err.Error(),
		})
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   serverWorker.TerminalAsk(),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   savedPath,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   serverWorker.TerminalAsk(),
	})
}

func (t *Terminal) beginCommand(ctx context.Context, b ChatBot, chatID int64, cancel context.CancelFunc) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.commandExecuting {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Wait for the previous command to complete",
		})
		return false
	}

	t.commandExecuting = true
	t.cancelCommand = cancel
	return true
}

func (t *Terminal) endCommand() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.commandExecuting = false
	t.cancelCommand = nil
}

func (t *Terminal) RequestTerminate() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.commandExecuting || t.cancelCommand == nil {
		return false
	}

	t.cancelCommand()
	return true
}
