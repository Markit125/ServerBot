# ServerBot Architecture

## High-Level Flow

```mermaid
flowchart TD
    User[Telegram User]
    TG[Telegram Platform]
    Main[cmd/bot/main.go]
    Config[internal/config]
    ServerBot[internal/bot.ServerBot]
    TgApi[github.com/go-telegram/bot]
    StartH[messagehandlers.Start]
    EchoH[messagehandlers.Echo]
    TerminalH[messagehandlers.Terminal]
    Worker[internal/serverworker.ServerWorker]
    OS[OS Shell / Process Execution]
    Resources[resources/embed.go + handlers_description.txt]

    User --> TG
    TG --> TgApi
    Main --> Config
    Main --> ServerBot
    Config --> ServerBot
    ServerBot --> TgApi

    TgApi -->|/start| StartH
    TgApi -->|/echo| EchoH
    TgApi -->|/terminal| TerminalH
    TgApi -->|other text| ServerBot

    ServerBot -->|delegates current mode| StartH
    ServerBot -->|delegates current mode| EchoH
    ServerBot -->|delegates current mode| TerminalH

    StartH --> Resources
    TerminalH --> Worker
    Worker --> OS

    StartH --> TgApi
    EchoH --> TgApi
    TerminalH --> TgApi
    TgApi --> TG
    TG --> User
```

## Package-Level View

```mermaid
flowchart LR
    cmd["cmd/bot"]
    bot["internal/bot"]
    config["internal/config"]
    handlers["internal/messagehandlers"]
    worker["internal/serverworker"]
    resources["resources"]
    extbot["go-telegram/bot"]
    dotenv["godotenv"]
    os["os/exec + os"]

    cmd --> config
    cmd --> bot
    bot --> config
    bot --> handlers
    bot --> worker
    bot --> extbot
    handlers --> worker
    handlers --> resources
    handlers --> extbot
    config --> dotenv
    config --> os
    worker --> os
```

## Runtime Mode Switching

```mermaid
stateDiagram-v2
    [*] --> StartMode
    StartMode --> EchoMode: /echo
    StartMode --> TerminalMode: /terminal
    EchoMode --> StartMode: /start
    TerminalMode --> StartMode: /start
    EchoMode --> EchoMode: any text -> repeat text
    TerminalMode --> TerminalMode: any text -> execute bash script
```

## Current Responsibilities

- `cmd/bot/main.go`: starts the application, loads config, builds `ServerBot`, starts bot loop with context cancellation.
- `internal/config`: loads `BOT_TOKEN` from environment via `.env`.
- `internal/bot`: composes the application, registers Telegram handlers, stores current message mode.
- `internal/messagehandlers`: mode-specific behavior for `/start`, `/echo`, and `/terminal`.
- `internal/serverworker`: executes terminal input as a temporary bash script and returns output plus prompt text.
- `resources`: embeds static help text shown by `/start`.

## Architectural Notes

- The bot uses a stateful "current handler" model: `ServerBot.messageHandler` determines how non-command text is interpreted.
- That mode is stored once at bot level, not per chat/user, so different chats currently share the same active mode.
- `Terminal` directly calls `ServerWorker.Exec`, which writes the incoming text to a temporary shell script and runs it through `bash`.
- `/c` is registered as an interrupt command, but `interruptHandler` is still empty.
- Tests exist around config loading, handler behavior, and command execution, which helps preserve the current shape of the architecture.
