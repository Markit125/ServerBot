#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SERVICE_NAME="${SERVICE_NAME:-servercommanderovertelegram}"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/.env}"
CONFIG_FILE="${CONFIG_FILE:-${ROOT_DIR}/config.toml}"
RUN_TESTS=1
START_SERVICE=0
ENABLE_SERVICE=1
FORCE=0

usage() {
    cat <<EOF
Usage: $0 [options]

Prepare ServerCommanderOverTelegram for first run on this machine.

Options:
  --start          Enable and start the systemd service after install.
  --no-enable     Do not enable the service when --start is used.
  --no-test       Skip go test ./...
  --force         Overwrite generated .env/config.toml from examples.
  -h, --help      Show this help.

Environment overrides:
  ROOT_DIR                 Project root. Default: auto-detected.
  SERVICE_NAME             systemd service name. Default: servercommanderovertelegram.
  SERVICE_USER             systemd service user. Default: root.
  ENV_FILE                 .env path. Default: \$ROOT_DIR/.env.
  CONFIG_FILE              config.toml path. Default: \$ROOT_DIR/config.toml.
  LOCAL_BOT_API_SERVICE    Local Bot API unit name. Default: telegram-bot-api.service.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --start)
            START_SERVICE=1
            ;;
        --no-enable)
            ENABLE_SERVICE=0
            ;;
        --no-test)
            RUN_TESTS=0
            ;;
        --force)
            FORCE=1
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
    shift
done

cd "${ROOT_DIR}"

log() {
    printf '[bootstrap] %s\n' "$*"
}

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Missing required command: $1" >&2
        exit 1
    fi
}

copy_example() {
    local source_path="$1"
    local target_path="$2"
    local display_name="$3"

    if [[ -f "${target_path}" && ${FORCE} -ne 1 ]]; then
        log "${display_name} already exists: ${target_path}"
        return
    fi

    if [[ ! -f "${source_path}" ]]; then
        echo "Missing example file: ${source_path}" >&2
        exit 1
    fi

    cp "${source_path}" "${target_path}"
    chmod 0600 "${target_path}"
    log "created ${display_name}: ${target_path}"
}

warn_if_token_placeholder() {
    if [[ ! -f "${ENV_FILE}" ]]; then
        return
    fi

    if grep -Eq 'BOT_TOKEN *= *"?(YOUR_BOT_TOKEN_FROM_BOTFATHER|)"?' "${ENV_FILE}"; then
        cat >&2 <<EOF

WARNING: ${ENV_FILE} still contains a placeholder BOT_TOKEN.
Edit it before starting the service:

  ${EDITOR:-nano} ${ENV_FILE}

EOF
    fi
}

require_command go
require_command make
require_command systemctl

log "project root: ${ROOT_DIR}"

copy_example "${ROOT_DIR}/.env.example" "${ENV_FILE}" ".env"
copy_example "${ROOT_DIR}/config.example.toml" "${CONFIG_FILE}" "config.toml"
warn_if_token_placeholder

log "ensuring scripts are executable"
chmod +x \
    "${ROOT_DIR}/scripts/run-servercommanderovertelegram.sh" \
    "${ROOT_DIR}/scripts/redeploy-servercommanderovertelegram.sh" \
    "${ROOT_DIR}/scripts/install-servercommanderovertelegram-service.sh" \
    "${ROOT_DIR}/scripts/bootstrap-servercommanderovertelegram.sh"

log "building binary"
make build

if [[ ${RUN_TESTS} -eq 1 ]]; then
    log "running tests"
    make test
fi

log "installing systemd service"
"${ROOT_DIR}/scripts/install-servercommanderovertelegram-service.sh"

if [[ ${START_SERVICE} -eq 1 ]]; then
    if [[ ${ENABLE_SERVICE} -eq 1 ]]; then
        log "enabling and starting ${SERVICE_NAME}.service"
        systemctl enable --now "${SERVICE_NAME}.service"
    else
        log "starting ${SERVICE_NAME}.service"
        systemctl start "${SERVICE_NAME}.service"
    fi
    systemctl status "${SERVICE_NAME}.service" --no-pager
else
    cat <<EOF

Bootstrap complete.

Review configuration:
  ${ENV_FILE}
  ${CONFIG_FILE}

Then start the service:
  systemctl enable --now ${SERVICE_NAME}.service

EOF
fi
