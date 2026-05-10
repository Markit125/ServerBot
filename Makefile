.PHONY: help build test restart status logs deploy reload-units restart-bot-api install-service bootstrap

ROOT_DIR := $(CURDIR)
BIN_DIR := $(ROOT_DIR)/bin
BIN_PATH := $(BIN_DIR)/servercommanderovertelegram
SERVICE_NAME := servercommanderovertelegram
LOCAL_BOT_API_SERVICE := telegram-bot-api.service
SYSTEMCTL ?= systemctl

help:
	@printf '%s\n' \
		'make build         Build the bot binary' \
		'make test          Run Go tests' \
		'make restart       Restart the servercommanderovertelegram systemd service' \
		'make status        Show servercommanderovertelegram service status' \
		'make logs          Follow bot logs' \
		'make deploy        Build, test, restart, and show status' \
		'make reload-units  Reload systemd unit files' \
		'make restart-bot-api Restart local telegram-bot-api service' \
		'make install-service Generate and install servercommanderovertelegram systemd unit' \
		'make bootstrap     Prepare config, build, test, and install service'

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) ./cmd/bot

test:
	go test ./...

restart:
	$(SYSTEMCTL) restart $(SERVICE_NAME)

status:
	$(SYSTEMCTL) status $(SERVICE_NAME) --no-pager

logs:
	tail -f $(ROOT_DIR)/logs/servercommanderovertelegram.log

deploy: build test restart status

reload-units:
	$(SYSTEMCTL) daemon-reload

restart-bot-api:
	$(SYSTEMCTL) restart $(LOCAL_BOT_API_SERVICE)

install-service:
	./scripts/install-servercommanderovertelegram-service.sh

bootstrap:
	./scripts/bootstrap-servercommanderovertelegram.sh
