package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/pelletier/go-toml/v2"
)

const (
	defaultConfigPath             = "config.toml"
	defaultMaxDownloadBytes       = 2 * 1024 * 1024 * 1024
	defaultSendTimeout            = 45 * time.Minute
	defaultUploadStallInterval    = 30 * time.Second
	defaultProgressEditInterval   = 5 * time.Second
	defaultProgressBytesStep      = 8 * 1024 * 1024
	defaultProgressQueueSize      = 128
	defaultProgressMessageMaxChar = 3900
	defaultTempDir                = "/tmp"
	defaultExecTempPattern        = "server-bot-exec-*"
	defaultDownloadTempPattern    = "server-bot-download-*"
)

type Config struct {
	BotToken                string
	BotAPIURL               string
	AccessEnabled           bool
	AccessDenyMessage       string
	AllowedUsers            []AllowedUser
	MaxDownloadBytes        int64
	SendTimeout             time.Duration
	UploadStallInterval     time.Duration
	ProgressEditInterval    time.Duration
	ProgressBytesStep       int64
	ProgressQueueSize       int
	ProgressMessageMaxChars int
	TempDir                 string
	ExecTempPattern         string
	DownloadTempPattern     string
	AutoTerminalAfterGet    bool
	DeleteProgressOnSuccess bool
}

type AllowedUser struct {
	ID    int64
	Label string
}

type fileConfig struct {
	Telegram telegramConfig `toml:"telegram"`
	Access   accessConfig   `toml:"access"`
	Get      getConfig      `toml:"get"`
	Paths    pathsConfig    `toml:"paths"`
}

type telegramConfig struct {
	BotToken  string `toml:"bot_token"`
	BotAPIURL string `toml:"bot_api_url"`
}

type accessConfig struct {
	Enabled      *bool               `toml:"enabled"`
	DenyMessage  string              `toml:"deny_message"`
	AllowedUsers []allowedUserConfig `toml:"allowed_users"`
}

type allowedUserConfig struct {
	ID    int64  `toml:"id"`
	Label string `toml:"label"`
}

type getConfig struct {
	MaxDownloadBytes        *int64 `toml:"max_download_bytes"`
	SendTimeout             string `toml:"send_timeout"`
	UploadStallInterval     string `toml:"upload_stall_interval"`
	ProgressEditInterval    string `toml:"progress_edit_interval"`
	ProgressBytesStep       *int64 `toml:"progress_bytes_step"`
	ProgressQueueSize       *int   `toml:"progress_queue_size"`
	ProgressMessageMaxChars *int   `toml:"progress_message_max_chars"`
	AutoTerminalAfterGet    *bool  `toml:"auto_terminal_after_success"`
	DeleteProgressOnSuccess *bool  `toml:"delete_progress_on_success"`
}

type pathsConfig struct {
	TempDir             string `toml:"temp_dir"`
	ExecTempPattern     string `toml:"exec_temp_pattern"`
	DownloadTempPattern string `toml:"download_temp_pattern"`
}

func New() (*Config, error) {
	_ = godotenv.Load()

	cfg := defaultConfig()
	if err := loadConfigFile(cfg); err != nil {
		return nil, err
	}
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, errors.New("could not load bot token from environment or config")
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		MaxDownloadBytes:        defaultMaxDownloadBytes,
		AccessDenyMessage:       "Access denied",
		SendTimeout:             defaultSendTimeout,
		UploadStallInterval:     defaultUploadStallInterval,
		ProgressEditInterval:    defaultProgressEditInterval,
		ProgressBytesStep:       defaultProgressBytesStep,
		ProgressQueueSize:       defaultProgressQueueSize,
		ProgressMessageMaxChars: defaultProgressMessageMaxChar,
		TempDir:                 defaultTempDir,
		ExecTempPattern:         defaultExecTempPattern,
		DownloadTempPattern:     defaultDownloadTempPattern,
		AutoTerminalAfterGet:    true,
		DeleteProgressOnSuccess: true,
	}
}

func loadConfigFile(cfg *Config) error {
	configPath := strings.TrimSpace(os.Getenv("SERVERCOMMANDEROVERTELEGRAM_CONFIG"))
	if configPath == "" {
		configPath = defaultConfigPath
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && os.Getenv("SERVERCOMMANDEROVERTELEGRAM_CONFIG") == "" {
			return nil
		}
		return fmt.Errorf("read config file %q: %w", configPath, err)
	}

	var raw fileConfig
	if err := toml.Unmarshal(content, &raw); err != nil {
		return fmt.Errorf("parse config file %q: %w", configPath, err)
	}

	return applyFileConfig(cfg, raw)
}

func applyFileConfig(cfg *Config, raw fileConfig) error {
	if raw.Telegram.BotToken != "" {
		cfg.BotToken = raw.Telegram.BotToken
	}
	if raw.Telegram.BotAPIURL != "" {
		cfg.BotAPIURL = raw.Telegram.BotAPIURL
	}

	if raw.Access.Enabled != nil {
		cfg.AccessEnabled = *raw.Access.Enabled
	}
	if raw.Access.DenyMessage != "" {
		cfg.AccessDenyMessage = raw.Access.DenyMessage
	}
	if raw.Access.AllowedUsers != nil {
		cfg.AllowedUsers = make([]AllowedUser, 0, len(raw.Access.AllowedUsers))
		for _, user := range raw.Access.AllowedUsers {
			cfg.AllowedUsers = append(cfg.AllowedUsers, AllowedUser{
				ID:    user.ID,
				Label: user.Label,
			})
		}
	}

	if raw.Get.MaxDownloadBytes != nil {
		cfg.MaxDownloadBytes = *raw.Get.MaxDownloadBytes
	}
	if raw.Get.SendTimeout != "" {
		duration, err := parseDuration("get.send_timeout", raw.Get.SendTimeout)
		if err != nil {
			return err
		}
		cfg.SendTimeout = duration
	}
	if raw.Get.UploadStallInterval != "" {
		duration, err := parseDuration("get.upload_stall_interval", raw.Get.UploadStallInterval)
		if err != nil {
			return err
		}
		cfg.UploadStallInterval = duration
	}
	if raw.Get.ProgressEditInterval != "" {
		duration, err := parseDuration("get.progress_edit_interval", raw.Get.ProgressEditInterval)
		if err != nil {
			return err
		}
		cfg.ProgressEditInterval = duration
	}
	if raw.Get.ProgressBytesStep != nil {
		cfg.ProgressBytesStep = *raw.Get.ProgressBytesStep
	}
	if raw.Get.ProgressQueueSize != nil {
		cfg.ProgressQueueSize = *raw.Get.ProgressQueueSize
	}
	if raw.Get.ProgressMessageMaxChars != nil {
		cfg.ProgressMessageMaxChars = *raw.Get.ProgressMessageMaxChars
	}
	if raw.Get.AutoTerminalAfterGet != nil {
		cfg.AutoTerminalAfterGet = *raw.Get.AutoTerminalAfterGet
	}
	if raw.Get.DeleteProgressOnSuccess != nil {
		cfg.DeleteProgressOnSuccess = *raw.Get.DeleteProgressOnSuccess
	}

	if raw.Paths.TempDir != "" {
		cfg.TempDir = raw.Paths.TempDir
	}
	if raw.Paths.ExecTempPattern != "" {
		cfg.ExecTempPattern = raw.Paths.ExecTempPattern
	}
	if raw.Paths.DownloadTempPattern != "" {
		cfg.DownloadTempPattern = raw.Paths.DownloadTempPattern
	}

	return nil
}

func applyEnvOverrides(cfg *Config) error {
	if value := os.Getenv("BOT_TOKEN"); value != "" {
		cfg.BotToken = value
	}
	if value := os.Getenv("BOT_API_URL"); value != "" {
		cfg.BotAPIURL = value
	}
	if err := applyBoolEnv("SERVERCOMMANDEROVERTELEGRAM_ACCESS_ENABLED", &cfg.AccessEnabled); err != nil {
		return err
	}
	if value := os.Getenv("SERVERCOMMANDEROVERTELEGRAM_ACCESS_DENY_MESSAGE"); value != "" {
		cfg.AccessDenyMessage = value
	}
	if value := os.Getenv("SERVERCOMMANDEROVERTELEGRAM_ALLOWED_USER_IDS"); value != "" {
		allowedUsers, err := parseAllowedUserIDs(value)
		if err != nil {
			return err
		}
		cfg.AllowedUsers = allowedUsers
	}

	if err := applyInt64Env("SERVERCOMMANDEROVERTELEGRAM_MAX_DOWNLOAD_BYTES", &cfg.MaxDownloadBytes); err != nil {
		return err
	}
	if err := applyDurationEnv("SERVERCOMMANDEROVERTELEGRAM_SEND_TIMEOUT", &cfg.SendTimeout); err != nil {
		return err
	}
	if err := applyDurationEnv("SERVERCOMMANDEROVERTELEGRAM_UPLOAD_STALL_INTERVAL", &cfg.UploadStallInterval); err != nil {
		return err
	}
	if err := applyDurationEnv("SERVERCOMMANDEROVERTELEGRAM_PROGRESS_EDIT_INTERVAL", &cfg.ProgressEditInterval); err != nil {
		return err
	}
	if err := applyInt64Env("SERVERCOMMANDEROVERTELEGRAM_PROGRESS_BYTES_STEP", &cfg.ProgressBytesStep); err != nil {
		return err
	}
	if err := applyIntEnv("SERVERCOMMANDEROVERTELEGRAM_PROGRESS_QUEUE_SIZE", &cfg.ProgressQueueSize); err != nil {
		return err
	}
	if err := applyIntEnv("SERVERCOMMANDEROVERTELEGRAM_PROGRESS_MESSAGE_MAX_CHARS", &cfg.ProgressMessageMaxChars); err != nil {
		return err
	}
	if err := applyBoolEnv("SERVERCOMMANDEROVERTELEGRAM_AUTO_TERMINAL_AFTER_GET", &cfg.AutoTerminalAfterGet); err != nil {
		return err
	}
	if err := applyBoolEnv("SERVERCOMMANDEROVERTELEGRAM_DELETE_PROGRESS_ON_SUCCESS", &cfg.DeleteProgressOnSuccess); err != nil {
		return err
	}

	if value := os.Getenv("SERVERCOMMANDEROVERTELEGRAM_TEMP_DIR"); value != "" {
		cfg.TempDir = value
	}
	if value := os.Getenv("SERVERCOMMANDEROVERTELEGRAM_EXEC_TEMP_PATTERN"); value != "" {
		cfg.ExecTempPattern = value
	}
	if value := os.Getenv("SERVERCOMMANDEROVERTELEGRAM_DOWNLOAD_TEMP_PATTERN"); value != "" {
		cfg.DownloadTempPattern = value
	}

	return nil
}

func validateConfig(cfg *Config) error {
	if cfg.AccessEnabled && len(cfg.AllowedUsers) == 0 {
		return errors.New("access is enabled but no allowed users are configured")
	}
	for _, user := range cfg.AllowedUsers {
		if user.ID <= 0 {
			return fmt.Errorf("invalid allowed Telegram user id: %d", user.ID)
		}
	}

	return nil
}

func parseAllowedUserIDs(value string) ([]AllowedUser, error) {
	parts := strings.Split(value, ",")
	users := make([]AllowedUser, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		id, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid SERVERCOMMANDEROVERTELEGRAM_ALLOWED_USER_IDS value %q: %w", trimmed, err)
		}
		users = append(users, AllowedUser{ID: id})
	}
	return users, nil
}

func parseDuration(name string, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s duration %q: %w", name, value, err)
	}
	return duration, nil
}

func applyDurationEnv(name string, target *time.Duration) error {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	*target = duration
	return nil
}

func applyInt64Env(name string, target *int64) error {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	*target = parsed
	return nil
}

func applyIntEnv(name string, target *int) error {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	*target = parsed
	return nil
}

func applyBoolEnv(name string, target *bool) error {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	*target = parsed
	return nil
}
