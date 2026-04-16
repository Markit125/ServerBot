package bot

import (
	"errors"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestExtractUploadedFile(t *testing.T) {
	tests := []struct {
		name     string
		message  *models.Message
		expected *uploadedFile
	}{
		{
			name: "document message",
			message: &models.Message{
				Document: &models.Document{
					FileID:   "doc-id",
					FileName: "report.txt",
				},
			},
			expected: &uploadedFile{
				FileID:   "doc-id",
				FileName: "report.txt",
				Kind:     "Document",
			},
		},
		{
			name: "audio message",
			message: &models.Message{
				Audio: &models.Audio{
					FileID:   "audio-id",
					FileName: "track.mp3",
				},
			},
			expected: &uploadedFile{
				FileID:   "audio-id",
				FileName: "track.mp3",
				Kind:     "Audio",
			},
		},
		{
			name: "video message",
			message: &models.Message{
				Video: &models.Video{
					FileID:   "video-id",
					FileName: "clip.mp4",
				},
			},
			expected: &uploadedFile{
				FileID:   "video-id",
				FileName: "clip.mp4",
				Kind:     "Video",
			},
		},
		{
			name: "unsupported message",
			message: &models.Message{
				Text: "hello",
			},
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, extractUploadedFile(test.message))
		})
	}
}

func TestFormatUploadedFileError(t *testing.T) {
	assert.Equal(
		t,
		"File is too big for direct download via Telegram Bot API. Configure a local telegram-bot-api server and set BOT_API_URL to support large files.",
		formatUploadedFileError(errors.New("Bad Request: file is too big")),
	)

	assert.Equal(
		t,
		"Failed to get file: some other error",
		formatUploadedFileError(errors.New("some other error")),
	)
}
