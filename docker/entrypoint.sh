#!/bin/sh
set -eu

CONFIG_PATH="${CONFIG_PATH:-/app/config.yaml}"

quote_yaml() {
  printf "'%s'" "$(printf "%s" "$1" | sed "s/'/''/g")"
}

write_line() {
  printf "%s\n" "$1" >> "$CONFIG_PATH"
}

generate_config() {
  : > "$CONFIG_PATH"

  LISTEN_VALUE="${LISTEN:-:8080}"
  AUTH_DIR_VALUE="${AUTH_DIR:-/app/auths}"
  BACKEND_DOMAIN_VALUE="${BACKEND_DOMAIN:-chatgpt.com}"
  LOG_LEVEL_VALUE="${LOG_LEVEL:-info}"
  REFRESH_INTERVAL_VALUE="${REFRESH_INTERVAL:-3000}"
  MAX_RETRY_VALUE="${MAX_RETRY:-2}"
  ENABLE_HEALTHY_RETRY_VALUE="${ENABLE_HEALTHY_RETRY:-true}"
  HEALTH_CHECK_INTERVAL_VALUE="${HEALTH_CHECK_INTERVAL:-300}"
  HEALTH_CHECK_MAX_FAILURES_VALUE="${HEALTH_CHECK_MAX_FAILURES:-2}"
  HEALTH_CHECK_CONCURRENCY_VALUE="${HEALTH_CHECK_CONCURRENCY:-5}"
  REFRESH_CONCURRENCY_VALUE="${REFRESH_CONCURRENCY:-50}"
  STARTUP_ASYNC_LOAD_VALUE="${STARTUP_ASYNC_LOAD:-true}"
  ENABLE_HTTP2_VALUE="${ENABLE_HTTP2:-false}"
  MAX_CONNS_PER_HOST_VALUE="${MAX_CONNS_PER_HOST:-512}"
  MAX_IDLE_CONNS_VALUE="${MAX_IDLE_CONNS:-1024}"
  MAX_IDLE_CONNS_PER_HOST_VALUE="${MAX_IDLE_CONNS_PER_HOST:-512}"
  DB_ENABLED_VALUE="${DB_ENABLED:-true}"
  DB_DRIVER_VALUE="${DB_DRIVER:-postgres}"
  DB_HOST_VALUE="${DB_HOST:-postgres}"
  DB_PORT_VALUE="${DB_PORT:-5432}"
  DB_USER_VALUE="${DB_USER:-codex}"
  DB_PASSWORD_VALUE="${DB_PASSWORD:-codex}"
  DB_NAME_VALUE="${DB_NAME:-codex_proxy}"
  DB_SSLMODE_VALUE="${DB_SSLMODE:-disable}"

  write_line "# 自动生成于容器启动时；如需自定义，请挂载 /app/config.yaml 覆盖"
  write_line "listen: $(quote_yaml "$LISTEN_VALUE")"
  write_line "auth-dir: $(quote_yaml "$AUTH_DIR_VALUE")"
  write_line "backend-domain: $(quote_yaml "$BACKEND_DOMAIN_VALUE")"
  write_line "log-level: $(quote_yaml "$LOG_LEVEL_VALUE")"
  write_line "refresh-interval: $REFRESH_INTERVAL_VALUE"
  write_line "max-retry: $MAX_RETRY_VALUE"
  write_line "enable-healthy-retry: $ENABLE_HEALTHY_RETRY_VALUE"
  write_line "health-check-interval: $HEALTH_CHECK_INTERVAL_VALUE"
  write_line "health-check-max-failures: $HEALTH_CHECK_MAX_FAILURES_VALUE"
  write_line "health-check-concurrency: $HEALTH_CHECK_CONCURRENCY_VALUE"
  write_line "refresh-concurrency: $REFRESH_CONCURRENCY_VALUE"
  write_line "startup-async-load: $STARTUP_ASYNC_LOAD_VALUE"
  write_line "enable-http2: $ENABLE_HTTP2_VALUE"
  write_line "max-conns-per-host: $MAX_CONNS_PER_HOST_VALUE"
  write_line "max-idle-conns: $MAX_IDLE_CONNS_VALUE"
  write_line "max-idle-conns-per-host: $MAX_IDLE_CONNS_PER_HOST_VALUE"
  write_line "db-enabled: $DB_ENABLED_VALUE"

  if [ "$DB_ENABLED_VALUE" = "true" ]; then
    write_line "db-driver: $(quote_yaml "$DB_DRIVER_VALUE")"
    write_line "db-host: $(quote_yaml "$DB_HOST_VALUE")"
    write_line "db-port: $DB_PORT_VALUE"
    write_line "db-user: $(quote_yaml "$DB_USER_VALUE")"
    write_line "db-password: $(quote_yaml "$DB_PASSWORD_VALUE")"
    write_line "db-name: $(quote_yaml "$DB_NAME_VALUE")"
    write_line "db-sslmode: $(quote_yaml "$DB_SSLMODE_VALUE")"
  fi

  if [ -n "${BASE_URL:-}" ]; then
    write_line "base-url: $(quote_yaml "$BASE_URL")"
  fi

  if [ -n "${PROXY_URL:-}" ]; then
    write_line "proxy-url: $(quote_yaml "$PROXY_URL")"
  fi

  if [ -n "${API_KEYS:-}" ]; then
    write_line "api-keys:"
    OLD_IFS=$IFS
    IFS=','
    for key in $API_KEYS; do
      trimmed_key="$(printf "%s" "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
      if [ -n "$trimmed_key" ]; then
        write_line "  - $(quote_yaml "$trimmed_key")"
      fi
    done
    IFS=$OLD_IFS
  fi
}

mkdir -p /app/auths

if [ ! -s "$CONFIG_PATH" ]; then
  echo "未检测到外部配置，正在生成容器内默认配置: $CONFIG_PATH"
  generate_config
else
  echo "检测到外部配置，直接使用: $CONFIG_PATH"
fi

exec /app/codex-proxy -config "$CONFIG_PATH"
