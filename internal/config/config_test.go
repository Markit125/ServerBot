package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuccessfulReadConfigFromEnv(t *testing.T) {
	chdirTemp(t)
	t.Setenv("BOT_TOKEN", "valid_bot_token")
	t.Setenv("BOT_API_URL", "http://localhost:8081")

	cfg, err := New()

	require.NoError(t, err)
	assert.Equal(t, "valid_bot_token", cfg.BotToken)
	assert.Equal(t, "http://localhost:8081", cfg.BotAPIURL)
	assert.Equal(t, int64(2*1024*1024*1024), cfg.MaxDownloadBytes)
	assert.Equal(t, 45*time.Minute, cfg.SendTimeout)
}

func TestSuccessfulReadConfigFromToml(t *testing.T) {
	tempDir := chdirTemp(t)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.toml"), []byte(`
[telegram]
bot_token = "config_token"
bot_api_url = "http://localhost:8081"

[access]
enabled = true
deny_message = "nope"

[[access.allowed_users]]
id = 123456789
label = "main-admin"

[get]
max_download_bytes = 123
send_timeout = "10m"
upload_stall_interval = "15s"
progress_edit_interval = "2s"
progress_bytes_step = 4096
progress_queue_size = 8
progress_message_max_chars = 1024
auto_terminal_after_success = false
delete_progress_on_success = false

[paths]
temp_dir = "/var/tmp"
exec_temp_pattern = "exec-*"
download_temp_pattern = "download-*"
`), 0o600))

	cfg, err := New()

	require.NoError(t, err)
	assert.Equal(t, "config_token", cfg.BotToken)
	assert.Equal(t, "http://localhost:8081", cfg.BotAPIURL)
	assert.True(t, cfg.AccessEnabled)
	assert.Equal(t, "nope", cfg.AccessDenyMessage)
	require.Len(t, cfg.AllowedUsers, 1)
	assert.Equal(t, int64(123456789), cfg.AllowedUsers[0].ID)
	assert.Equal(t, "main-admin", cfg.AllowedUsers[0].Label)
	assert.Equal(t, int64(123), cfg.MaxDownloadBytes)
	assert.Equal(t, 10*time.Minute, cfg.SendTimeout)
	assert.Equal(t, 15*time.Second, cfg.UploadStallInterval)
	assert.Equal(t, 2*time.Second, cfg.ProgressEditInterval)
	assert.Equal(t, int64(4096), cfg.ProgressBytesStep)
	assert.Equal(t, 8, cfg.ProgressQueueSize)
	assert.Equal(t, 1024, cfg.ProgressMessageMaxChars)
	assert.False(t, cfg.AutoTerminalAfterGet)
	assert.False(t, cfg.DeleteProgressOnSuccess)
	assert.Equal(t, "/var/tmp", cfg.TempDir)
	assert.Equal(t, "exec-*", cfg.ExecTempPattern)
	assert.Equal(t, "download-*", cfg.DownloadTempPattern)
}

func TestEnvOverridesToml(t *testing.T) {
	tempDir := chdirTemp(t)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.toml"), []byte(`
[telegram]
bot_token = "config_token"
bot_api_url = "http://localhost:8081"

[get]
send_timeout = "10m"
`), 0o600))
	t.Setenv("BOT_TOKEN", "env_token")
	t.Setenv("SERVERBOT_SEND_TIMEOUT", "20m")
	t.Setenv("SERVERBOT_ACCESS_ENABLED", "true")
	t.Setenv("SERVERBOT_ALLOWED_USER_IDS", "111,222")

	cfg, err := New()

	require.NoError(t, err)
	assert.Equal(t, "env_token", cfg.BotToken)
	assert.Equal(t, 20*time.Minute, cfg.SendTimeout)
	assert.True(t, cfg.AccessEnabled)
	require.Len(t, cfg.AllowedUsers, 2)
	assert.Equal(t, int64(111), cfg.AllowedUsers[0].ID)
	assert.Equal(t, int64(222), cfg.AllowedUsers[1].ID)
}

func TestAccessEnabledRequiresAllowedUsers(t *testing.T) {
	tempDir := chdirTemp(t)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.toml"), []byte(`
[telegram]
bot_token = "config_token"

[access]
enabled = true
`), 0o600))

	cfg, err := New()

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "allowed users")
}

func TestMissingToken(t *testing.T) {
	chdirTemp(t)
	t.Setenv("BOT_TOKEN", "")

	cfg, err := New()

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func chdirTemp(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})
	return tempDir
}
