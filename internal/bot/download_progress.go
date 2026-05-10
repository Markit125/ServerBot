package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"servercommanderovertelegram/internal/serverworker"
	"strings"
	"sync/atomic"
	"time"

	tgbot "github.com/go-telegram/bot"
)

type getProgressReporter struct {
	b         *tgbot.Bot
	chatID    any
	fileName  string
	events    chan serverworker.DownloadProgress
	done      chan struct{}
	lastSent  time.Time
	messageID int
	lines     []string
	success   atomic.Bool
	startedAt time.Time
	editEvery time.Duration
	maxChars  int
}

func newGetProgressReporter(ctx context.Context, b *tgbot.Bot, chatID any, fileName string, queueSize int, editEvery time.Duration, maxChars int) *getProgressReporter {
	if queueSize <= 0 {
		queueSize = 128
	}
	if editEvery <= 0 {
		editEvery = 5 * time.Second
	}
	if maxChars <= 0 {
		maxChars = 3900
	}

	reporter := &getProgressReporter{
		b:         b,
		chatID:    chatID,
		fileName:  fileName,
		events:    make(chan serverworker.DownloadProgress, queueSize),
		done:      make(chan struct{}),
		startedAt: time.Now(),
		editEvery: editEvery,
		maxChars:  maxChars,
	}

	go reporter.run(ctx)
	return reporter
}

func (r *getProgressReporter) Event(event serverworker.DownloadProgress) {
	if event.Name == "" {
		event.Name = r.fileName
	}

	log.Printf("[GET] file=%q stage=%s path=%q files=%d bytes=%d total=%d elapsed=%s", event.Name, event.Stage, event.Path, event.Files, event.Bytes, event.TotalBytes, time.Since(r.startedAt).Round(time.Millisecond))

	select {
	case r.events <- event:
	default:
		log.Printf("[GET] file=%q stage=progress_queue_full dropped_stage=%s", r.fileName, event.Stage)
	}
}

func (r *getProgressReporter) Close() {
	close(r.events)
	<-r.done
}

func (r *getProgressReporter) DeleteOnClose() {
	r.success.Store(true)
}

func (r *getProgressReporter) run(ctx context.Context) {
	defer close(r.done)

	for event := range r.events {
		text, force := r.telegramText(event)
		if text == "" {
			continue
		}
		if !force && time.Since(r.lastSent) < r.editEvery {
			continue
		}

		r.appendLine(text)
		if err := r.upsertMessage(ctx, r.messageText()); err != nil {
			log.Printf("[GET] file=%q stage=telegram_progress_failed progress_stage=%s error=%v", r.fileName, event.Stage, err)
			continue
		}
		r.lastSent = time.Now()
	}

	if r.success.Load() && r.messageID != 0 {
		if _, err := r.b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{ChatID: r.chatID, MessageID: r.messageID}); err != nil {
			log.Printf("[GET] file=%q stage=telegram_progress_delete_failed error=%v", r.fileName, err)
		}
	}
}

func (r *getProgressReporter) upsertMessage(ctx context.Context, text string) error {
	if r.messageID == 0 {
		message, err := r.b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:              r.chatID,
			Text:                text,
			DisableNotification: true,
		})
		if err != nil {
			return err
		}
		r.messageID = message.ID
		return nil
	}

	_, err := r.b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:    r.chatID,
		MessageID: r.messageID,
		Text:      text,
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
		return nil
	}
	return err
}

func (r *getProgressReporter) appendLine(text string) {
	if len(r.lines) == 0 || r.lines[len(r.lines)-1] != text {
		r.lines = append(r.lines, text)
	}

	for len(r.lines) > 1 && len(r.messageText()) > r.maxChars {
		r.lines = r.lines[1:]
	}
}

func (r *getProgressReporter) messageText() string {
	return strings.Join(r.lines, "\n")
}

func (r *getProgressReporter) telegramText(event serverworker.DownloadProgress) (string, bool) {
	switch event.Stage {
	case "request_started":
		return fmt.Sprintf("/get started: %s", event.Name), true
	case "resolve_done":
		return fmt.Sprintf("Selected: %s", event.Name), true
	case "size_check_done":
		return fmt.Sprintf("Size check done: %s, files: %d", formatProgressBytes(event.TotalBytes), event.Files), true
	case "file_ready":
		return fmt.Sprintf("File is ready: %s", formatProgressBytes(event.TotalBytes)), true
	case "archive_started":
		return fmt.Sprintf("Archiving started: %s", event.Name), true
	case "archive_bytes", "archive_file_done":
		return fmt.Sprintf("Archiving: %s / %s, files: %d", formatProgressBytes(event.Bytes), formatProgressBytes(event.TotalBytes), event.Files), false
	case "archive_ready":
		return fmt.Sprintf("Archive ready: %s, zip: %s", formatProgressBytes(event.TotalBytes), formatProgressBytes(event.Bytes)), true
	case "open_started":
		return "Opening prepared file", true
	case "open_done":
		return fmt.Sprintf("Upload to Telegram started: %s", formatProgressBytes(event.TotalBytes)), true
	case "telegram_request_started":
		return "Streaming request to local Bot API started", true
	case "telegram_local_path_send_started":
		return "Local Bot API send started from prepared file path", true
	case "telegram_local_path_send_done":
		return "Local Bot API accepted prepared file path", true
	case "telegram_upload_waiting":
		return fmt.Sprintf("Upload is waiting: %s / %s", formatProgressBytes(event.Bytes), formatProgressBytes(event.TotalBytes)), false
	case "telegram_upload_bytes":
		return fmt.Sprintf("Uploading to Telegram: %s / %s", formatProgressBytes(event.Bytes), formatProgressBytes(event.TotalBytes)), false
	case "send_done":
		return "Telegram send completed", true
	case "cleanup_done":
		return "Temporary files cleaned up", true
	case "failed":
		return fmt.Sprintf("Failed: %s", event.Name), true
	}

	if strings.HasSuffix(event.Stage, "_failed") {
		return fmt.Sprintf("Failed at stage: %s", event.Stage), true
	}

	return "", false
}

type progressReader struct {
	reader      io.Reader
	reporter    *getProgressReporter
	stage       string
	name        string
	totalBytes  int64
	readBytes   int64
	nextReport  int64
	reportEvery int64
	lastRead    *atomic.Int64
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.readBytes += int64(n)
		if r.lastRead != nil {
			r.lastRead.Store(r.readBytes)
		}
		if r.nextReport == 0 {
			r.nextReport = r.reportEvery
		}
		for r.readBytes >= r.nextReport {
			r.reporter.Event(serverworker.DownloadProgress{
				Stage:      r.stage,
				Name:       r.name,
				Bytes:      r.readBytes,
				TotalBytes: r.totalBytes,
			})
			r.nextReport += r.reportEvery
		}
	}

	if err == io.EOF {
		r.reporter.Event(serverworker.DownloadProgress{
			Stage:      r.stage,
			Name:       r.name,
			Bytes:      r.readBytes,
			TotalBytes: r.totalBytes,
		})
	}

	return n, err
}

func formatProgressBytes(size int64) string {
	const mib = 1024 * 1024
	const gib = 1024 * mib

	switch {
	case size >= gib:
		return fmt.Sprintf("%.2f GiB", float64(size)/gib)
	case size >= mib:
		return fmt.Sprintf("%.1f MiB", float64(size)/mib)
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}
