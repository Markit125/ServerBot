#!/usr/bin/env bash

set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/bin/servercommanderovertelegram"
LOG_DIR="${ROOT_DIR}/logs"
LOG_FILE="${LOG_DIR}/servercommanderovertelegram.log"
RESTART_DELAY_SECONDS="${RESTART_DELAY_SECONDS:-5}"
RUN_LOG_TAIL_LINES="${RUN_LOG_TAIL_LINES:-20}"
LOG_MAX_BYTES="${LOG_MAX_BYTES:-52428800}"
LOG_KEEP_BYTES="${LOG_KEEP_BYTES:-10485760}"
LOG_SHRINK_INTERVAL_SECONDS="${LOG_SHRINK_INTERVAL_SECONDS:-300}"
RUN_LOG_MAX_AGE_DAYS="${RUN_LOG_MAX_AGE_DAYS:-7}"
RUN_LOG_MAX_FILES="${RUN_LOG_MAX_FILES:-20}"

mkdir -p "${LOG_DIR}"

timestamp() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

log_line() {
    printf '[%s] %s\n' "$(timestamp)" "$*" >> "${LOG_FILE}"
}

is_non_negative_integer() {
    [[ "$1" =~ ^[0-9]+$ ]]
}

shrink_log_file() {
    local file_path="$1"
    local max_bytes="$2"
    local keep_bytes="$3"

    if [[ ! -f "${file_path}" ]]; then
        return
    fi
    if ! is_non_negative_integer "${max_bytes}" || ! is_non_negative_integer "${keep_bytes}"; then
        return
    fi
    if [[ ${max_bytes} -eq 0 || ${keep_bytes} -eq 0 || ${keep_bytes} -ge ${max_bytes} ]]; then
        return
    fi

    local current_size
    current_size="$(stat -c '%s' "${file_path}" 2>/dev/null || echo 0)"
    if [[ ${current_size} -le ${max_bytes} ]]; then
        return
    fi

    local tmp_file
    tmp_file="$(mktemp "${LOG_DIR}/servercommanderovertelegram-log-shrink.XXXXXX")"
    if tail -c "${keep_bytes}" "${file_path}" > "${tmp_file}"; then
        : > "${file_path}"
        cat "${tmp_file}" > "${file_path}"
        printf '[%s] shrank log file from %s bytes to last %s bytes\n' "$(timestamp)" "${current_size}" "${keep_bytes}" >> "${file_path}"
    fi
    rm -f "${tmp_file}"
}

prune_run_logs() {
    if is_non_negative_integer "${RUN_LOG_MAX_AGE_DAYS}" && [[ ${RUN_LOG_MAX_AGE_DAYS} -gt 0 ]]; then
        find "${LOG_DIR}" -maxdepth 1 -type f -name 'servercommanderovertelegram-run.*.log' -mtime "+${RUN_LOG_MAX_AGE_DAYS}" -delete 2>/dev/null || true
    fi

    if ! is_non_negative_integer "${RUN_LOG_MAX_FILES}" || [[ ${RUN_LOG_MAX_FILES} -eq 0 ]]; then
        return
    fi

    mapfile -t logs_to_remove < <(
        find "${LOG_DIR}" -maxdepth 1 -type f -name 'servercommanderovertelegram-run.*.log' -printf '%T@ %p\n' 2>/dev/null \
            | sort -nr \
            | tail -n +"$((RUN_LOG_MAX_FILES + 1))" \
            | cut -d ' ' -f 2-
    )

    if [[ ${#logs_to_remove[@]} -gt 0 ]]; then
        rm -f "${logs_to_remove[@]}"
    fi
}

maintain_logs() {
    shrink_log_file "${LOG_FILE}" "${LOG_MAX_BYTES}" "${LOG_KEEP_BYTES}"
    prune_run_logs
}

log_maintenance_loop() {
    if ! is_non_negative_integer "${LOG_SHRINK_INTERVAL_SECONDS}" || [[ ${LOG_SHRINK_INTERVAL_SECONDS} -eq 0 ]]; then
        return
    fi

    while true; do
        sleep "${LOG_SHRINK_INTERVAL_SECONDS}"
        maintain_logs
    done
}

signal_name() {
    local signal_number="$1"

    case "${signal_number}" in
        1) echo "SIGHUP" ;;
        2) echo "SIGINT" ;;
        3) echo "SIGQUIT" ;;
        6) echo "SIGABRT" ;;
        9) echo "SIGKILL" ;;
        11) echo "SIGSEGV" ;;
        13) echo "SIGPIPE" ;;
        14) echo "SIGALRM" ;;
        15) echo "SIGTERM" ;;
        *) echo "signal ${signal_number}" ;;
    esac
}

log_crash_context() {
    local run_log_file="$1"
    local exit_code="$2"

    if [[ ${exit_code} -gt 128 ]]; then
        local signal_number=$((exit_code - 128))
        local signal_text
        signal_text="$(signal_name "${signal_number}")"
        log_line "bot crashed after receiving ${signal_text} (exit code ${exit_code})"
        if [[ ${signal_number} -eq 9 ]]; then
            log_line "SIGKILL often means the process was killed externally or by the OOM killer"
        fi
    else
        log_line "bot crashed with exit code ${exit_code}"
    fi

    if [[ -f "${run_log_file}" ]]; then
        log_line "last ${RUN_LOG_TAIL_LINES} lines before crash:"
        tail -n "${RUN_LOG_TAIL_LINES}" "${run_log_file}" | while IFS= read -r line; do
            log_line "process: ${line}"
        done
    fi
}

if [[ ! -x "${BIN_PATH}" ]]; then
    log_line "binary ${BIN_PATH} is missing or not executable"
    exit 1
fi

maintain_logs
log_maintenance_loop &
maintenance_pid=$!
trap 'kill "${maintenance_pid}" 2>/dev/null || true' EXIT

while true; do
    maintain_logs
    run_log_file="$(mktemp "${LOG_DIR}/servercommanderovertelegram-run.XXXXXX.log")"
    log_line "starting bot process"
    "${BIN_PATH}" 2>&1 | tee -a "${LOG_FILE}" "${run_log_file}"
    exit_code=${PIPESTATUS[0]}

    if [[ ${exit_code} -eq 0 ]]; then
        rm -f "${run_log_file}"
        log_line "bot stopped gracefully"
        exit 0
    fi

    log_crash_context "${run_log_file}" "${exit_code}"
    rm -f "${run_log_file}"
    log_line "restarting in ${RESTART_DELAY_SECONDS}s"
    sleep "${RESTART_DELAY_SECONDS}"
done
