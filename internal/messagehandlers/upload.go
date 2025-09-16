package messagehandlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"serverbot/internal/serverworker"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Upload struct{}

func (u *Upload) Handle(ctx context.Context, b ChatBot, update *models.Update, sw *serverworker.ServerWorker) {
	msg := update.Message

	var (
		fileID   string
		fileName string
	)

	switch {
	case msg.Audio != nil:
		fileID = msg.Audio.FileID
		fileName = msg.Audio.FileName
	case msg.Document != nil:
		fileID = msg.Document.FileID
		fileName = msg.Document.FileName
	case msg.Video != nil:
		fileID = msg.Video.FileID
		fileName = msg.Video.FileName
	default:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Attach file to upload",
		})
		return
	}

	err := uploadFile(ctx, b, fileID, fileName)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "File uploaded successfully",
	})
}

func uploadFile(ctx context.Context, b ChatBot, fileID string, fileName string) error {
	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return fmt.Errorf("error trying to get local file: %w", err)
	}

	srcFile := file.FilePath
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current working directory: %w", err)
	}

	destPath := filepath.Join(currentDir, fileName)
	if err := os.Rename(srcFile, destPath); err != nil {
		return fmt.Errorf("error moving file: %w", err)
	}

	return nil
}

func downloadFile(ctx context.Context, b ChatBot, fileID string, fileName string) error {
	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return fmt.Errorf("error trying to GetFile: %w", err)
	}

	link := b.FileDownloadLink(file)

	resp, err := http.Get(link)
	if err != nil {
		return fmt.Errorf("error trying to execute Get method: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	localFile, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("create file %s error: %w", fileName, err)
	}

	_, err = io.Copy(localFile, resp.Body)
	if err != nil {
		return fmt.Errorf("file copy error: %w", err)
	}

	return nil
}
