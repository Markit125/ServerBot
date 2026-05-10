package messagehandlers

import (
	"context"
	"fmt"
	"servercommanderovertelegram/internal/serverworker"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const getCallbackPrefix = "get:"

type Get struct {
	entries []serverworker.DownloadableEntry
}

func (g *Get) Handle(ctx context.Context, b ChatBot, update *models.Update, serverWorker *serverworker.ServerWorker) {
	text, keyboard, err := g.buildSelection(serverWorker)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("Failed to prepare file list: %v", err),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
}

func (g *Get) ResolveSelection(data string) (serverworker.DownloadableEntry, error) {
	if !strings.HasPrefix(data, getCallbackPrefix) {
		return serverworker.DownloadableEntry{}, fmt.Errorf("unsupported callback data")
	}

	rawIndex := strings.TrimPrefix(data, getCallbackPrefix)
	index, err := strconv.Atoi(rawIndex)
	if err != nil {
		return serverworker.DownloadableEntry{}, fmt.Errorf("invalid selection")
	}
	if index < 0 || index >= len(g.entries) {
		return serverworker.DownloadableEntry{}, fmt.Errorf("selection is out of date, run /get again")
	}

	return g.entries[index], nil
}

func (g *Get) buildSelection(serverWorker *serverworker.ServerWorker) (string, *models.InlineKeyboardMarkup, error) {
	entries, err := serverWorker.ListCurrentDir()
	if err != nil {
		return "", nil, err
	}

	g.entries = entries
	if len(entries) == 0 {
		return "Текущая директория пуста.", nil, nil
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(entries))
	for index, entry := range entries {
		label := entry.Name
		if entry.IsDir {
			label += "/"
		}

		rows = append(rows, []models.InlineKeyboardButton{
			{
				Text:         label,
				CallbackData: fmt.Sprintf("%s%d", getCallbackPrefix, index),
			},
		})
	}

	return "Выберите файл для получения.", &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}, nil
}
