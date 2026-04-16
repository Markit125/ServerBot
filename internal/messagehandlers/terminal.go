package messagehandlers

import (
	"context"
	"fmt"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Terminal struct {
	commandExecuting bool
}

const INTERRUPT = "/c"

func (t *Terminal) Handle(ctx context.Context, b ChatBot, update *models.Update, serverWorker *serverworker.ServerWorker) {
	if !t.beginCommand(ctx, b, update.Message.Chat.ID) {
		return
	}
	defer t.endCommand()

	result, path := serverWorker.Exec(ctx, update.Message.Text)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   result,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   path,
	})
}

func (t *Terminal) HandleDocument(ctx context.Context, b ChatBot, update *models.Update, serverWorker *serverworker.ServerWorker, document *UploadedDocument) {
	if !t.beginCommand(ctx, b, update.Message.Chat.ID) {
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

func (t *Terminal) beginCommand(ctx context.Context, b ChatBot, chatID int64) bool {
	if t.commandExecuting {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Wait for the previous command to complete",
		})
		return false
	}

	t.commandExecuting = true
	return true
}

func (t *Terminal) endCommand() {
	t.commandExecuting = false
}
