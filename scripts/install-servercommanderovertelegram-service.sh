#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SERVICE_NAME="${SERVICE_NAME:-servercommanderovertelegram}"
SERVICE_USER="${SERVICE_USER:-root}"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/.env}"
LOCAL_BOT_API_SERVICE="${LOCAL_BOT_API_SERVICE:-telegram-bot-api.service}"
TEMPLATE_PATH="${ROOT_DIR}/deploy/servercommanderovertelegram.service.template"
UNIT_PATH="${UNIT_PATH:-/etc/systemd/system/${SERVICE_NAME}.service}"

if [[ ${EUID} -ne 0 ]]; then
    echo "This script installs a systemd unit and must be run as root." >&2
    exit 1
fi

if [[ ! -f "${TEMPLATE_PATH}" ]]; then
    echo "Missing template: ${TEMPLATE_PATH}" >&2
    exit 1
fi

escape_sed_replacement() {
    printf '%s' "$1" | sed -e 's/[\/&]/\\&/g'
}

root_dir_escaped="$(escape_sed_replacement "${ROOT_DIR}")"
env_file_escaped="$(escape_sed_replacement "${ENV_FILE}")"
service_user_escaped="$(escape_sed_replacement "${SERVICE_USER}")"
local_bot_api_service_escaped="$(escape_sed_replacement "${LOCAL_BOT_API_SERVICE}")"

tmp_unit="$(mktemp)"
trap 'rm -f "${tmp_unit}"' EXIT

sed \
    -e "s/{{ROOT_DIR}}/${root_dir_escaped}/g" \
    -e "s/{{ENV_FILE}}/${env_file_escaped}/g" \
    -e "s/{{SERVICE_USER}}/${service_user_escaped}/g" \
    -e "s/{{LOCAL_BOT_API_SERVICE}}/${local_bot_api_service_escaped}/g" \
    "${TEMPLATE_PATH}" > "${tmp_unit}"

install -m 0644 "${tmp_unit}" "${UNIT_PATH}"
systemctl daemon-reload

echo "Installed ${UNIT_PATH}"
echo "Run: systemctl enable --now ${SERVICE_NAME}.service"
