# ServerCommanderOverTelegram (ServerBot)

A Telegram bot that gives you secure remote access to your Linux server's terminal and file system right from your chat.

Current features:

*   `/terminal`: Run shell commands in the bot working directory.
    
*   File Uploads: Send any file to the bot, and it will be saved directly to the server's current working directory.
    
*   `/get`: Choose a file or directory from the current working directory and receive it in Telegram.
    
*   Directory downloads are archived as `Name.zip`.
    
*   Large File Transfers: Bypass standard Telegram file size limits by configuring a local Telegram Bot API server.
    
*   Security: Restrict bot access to specific users via a Telegram User ID allowlist.
    
*   `/echo`: Simple echo mode.
    

## Table of Contents

*   [Requirements](#requirements)
*   [Configuration](#configuration)
*   [Access Control](#access-control)
*   [First Run](#first-run)
*   [Daily Deploy](#daily-deploy)
*   [Useful Commands](#useful-commands)
*   [`/get` Behavior](#get-behavior)
*   [Telegram Command Menu](#telegram-command-menu)
*   [Important Files](#important-files)

## Requirements

*   Go installed on the server.
    
*   `systemd`.
    
*   Optional: local `telegram-bot-api` running in `--local` mode for large file transfers (30MB+).
    

## Configuration

The bot uses an `.env` file for secrets and an optional `config.toml` for general settings. Environment variables still override TOML values.

Minimal `.env` (required):

```
BOT_TOKEN = "YOUR_BOT_TOKEN_FROM_BOTFATHER"
```

Optional `config.toml`:

```
cp config.example.toml config.toml
```

For local Bot API, set this in `config.toml`:

```
[telegram]
bot_api_url = "http://localhost:8081"
```

## Access Control

> **⚠️ WARNING:** Because `/terminal` allows executing arbitrary code on your server, it is highly recommended to properly configure the allowlist before exposing the bot.

By default, the allowlist is empty, and access protection is disabled in the template. You must either explicitly enable and configure it or leave it disabled at your own risk. Local `config.toml` is ignored by git.

```
[access]
# Restrict bot usage to specific Telegram user IDs.
# Strongly recommended for production because /terminal can run server commands.
enabled = false
deny_message = "Access denied"

# Telegram usernames can change. Use them only as labels/comments.
# To find your numeric Telegram user ID, send any message to the bot while
# access is disabled and check logs, or use a trusted "user id" bot.
#
# [[access.allowed_users]]
# id = 123456789
# label = "main-admin"
#
# [[access.allowed_users]]
# id = 987654321
# label = "backup-admin"
```

## First Run

From the project root:

```
cp .env.example .env
# edit BOT_TOKEN in .env
./scripts/bootstrap-serverbot.sh
systemctl enable --now serverbot.service
```

To install and start immediately:

```
./scripts/bootstrap-serverbot.sh --start
```

## Daily Deploy

After code changes:

```
cd /path/to/ServerCommanderOverTelegram
./scripts/redeploy-serverbot.sh
```

The redeploy script builds the binary, runs tests, restarts `serverbot.service`, and prints service status.

If `deploy/serverbot.service.template` changed:

```
make install-service
./scripts/redeploy-serverbot.sh --reload-units
```

If local `telegram-bot-api` config or service changed:

```
./scripts/redeploy-serverbot.sh --reload-units --restart-bot-api
```

## Useful Commands

```
make build    # Build the application
make test     # Run the test suite
make restart  # Restart the serverbot service
make status   # Show the systemd service status
make logs     # Follow the serverbot logs
make deploy   # Build, test, restart, and show status in one go
```

## `/get` Behavior

`/get` lists files and directories from the bot's current working directory. After the user selects an item:

1.  The bot checks that the file or directory exists.
    
2.  The bot calculates size and file count.
    
3.  Directories are archived into `/tmp/serverbot-download-*/Name.zip`.
    
4.  A single Telegram progress message is created and then edited as stages change.
    
5.  With local Bot API, the bot sends the prepared file as `file:///tmp/.../Name.zip`.
    
6.  On success, Telegram shows the uploaded file and the progress message is deleted.
    
7.  On error, the progress message remains and contains the error.
    
8.  Temporary files are cleaned up.
    
9.  After a successful `/get`, the bot switches back to terminal mode automatically.
    

The main operational log contains `[GET]` lines for each stage:

```
tail -f logs/serverbot.log
```

## Telegram Command Menu

Commands can be added through `@BotFather`:

```
start - Start bot
terminal - Terminal mode
get - Download file or directory
echo - Echo mode
```

## Important Files

*   `config.example.toml`: non-secret config template.
    
*   `.env.example`: secret/env template.
    
*   `deploy/serverbot.service.template`: portable systemd unit template.
    
*   `scripts/run-serverbot.sh`: supervised process wrapper.
    
*   `scripts/install-serverbot-service.sh`: generates and installs the systemd unit for the current clone path.
    
*   `scripts/bootstrap-serverbot.sh`: first-run helper that creates local config files, builds, tests, and installs the service.
    
*   `scripts/redeploy-serverbot.sh`: build/test/restart helper.
    
*   `logs/serverbot.log`: local runtime log file, ignored by git.
    

`scripts/run-serverbot.sh` also shrinks `logs/serverbot.log` and prunes old `serverbot-run.*.log` files. Tune this with `LOG_MAX_BYTES`, `LOG_KEEP_BYTES`, `LOG_SHRINK_INTERVAL_SECONDS`, `RUN_LOG_MAX_AGE_DAYS`, and `RUN_LOG_MAX_FILES` in `.env`.
