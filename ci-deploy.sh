#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$SCRIPT_DIR/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-$SCRIPT_DIR/.env}"
LEGACY_COMPOSE_FILE="${LEGACY_COMPOSE_FILE:-$SCRIPT_DIR/docker-compose.legacy.yml}"
IMAGE_TAR=""
IMAGE_TAG=""
WAIT_HEALTH=0
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-180}"

compose_cmd=()

log() {
  printf '[ci-deploy] %s\n' "$*"
}

die() {
  log "$*"
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  ./ci-deploy.sh [--image-tar /path/to/image.tar.gz] [--image-tag new-api:main-<sha>] [--wait]
EOF
}

detect_compose() {
  if docker compose version >/dev/null 2>&1; then
    compose_cmd=(docker compose)
  elif command -v docker-compose >/dev/null 2>&1; then
    compose_cmd=(docker-compose)
  else
    die "Docker Compose 未安装"
  fi
}

extract_container_env() {
  local container="$1"
  local key="$2"

  docker inspect "$container" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
    | awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, "", $0); print; exit }'
}

extract_legacy_env() {
  local key="$1"
  local source_file="$LEGACY_COMPOSE_FILE"

  [[ -f "$source_file" ]] || source_file="$COMPOSE_FILE"
  [[ -f "$source_file" ]] || return 0

  awk -v key="$key" '
    $0 ~ "^[[:space:]]*-[[:space:]]*" key "=" {
      line = $0
      sub(/^[[:space:]]*-[[:space:]]*/, "", line)
      sub(key "=", "", line)
      sub(/[[:space:]]*#.*/, "", line)
      sub(/^[[:space:]]+/, "", line)
      sub(/[[:space:]]+$/, "", line)
      print line
      exit
    }
    $0 ~ "^[[:space:]]*" key ":[[:space:]]*" {
      line = $0
      sub("^[[:space:]]*" key ":[[:space:]]*", "", line)
      sub(/[[:space:]]*#.*/, "", line)
      gsub(/^["'"'"']|["'"'"']$/, "", line)
      sub(/^[[:space:]]+/, "", line)
      sub(/[[:space:]]+$/, "", line)
      print line
      exit
    }
  ' "$source_file"
}

extract_running_port() {
  local port_line

  port_line="$(docker port new-api 3000/tcp 2>/dev/null | head -n 1 || true)"
  if [[ -n "$port_line" ]]; then
    printf '%s\n' "${port_line##*:}"
    return 0
  fi

  [[ -f "$LEGACY_COMPOSE_FILE" ]] || return 0
  awk '
    $0 ~ /:[0-9]+:3000"/ {
      line = $0
      sub(/.*"/, "", line)
      sub(/:3000".*/, "", line)
      print line
      exit
    }
  ' "$LEGACY_COMPOSE_FILE"
}

is_placeholder() {
  [[ "$1" == *'${'* ]]
}

upsert_env() {
  local key="$1"
  local value="$2"
  local tmp_file

  tmp_file="$(mktemp)"
  if [[ -f "$ENV_FILE" ]]; then
    awk -v key="$key" -v value="$value" '
      BEGIN { updated = 0 }
      $0 ~ "^" key "=" { print key "=" value; updated = 1; next }
      { print }
      END { if (updated == 0) print key "=" value }
    ' "$ENV_FILE" >"$tmp_file"
  else
    printf '%s=%s\n' "$key" "$value" >"$tmp_file"
  fi
  mv "$tmp_file" "$ENV_FILE"
}

ensure_env() {
  local value

  [[ -f "$ENV_FILE" ]] && return 0

  log "未找到 .env，开始从当前运行态引导最小配置"
  : >"$ENV_FILE"
  chmod 600 "$ENV_FILE"

  value="$(docker inspect new-api --format '{{.Config.Image}}' 2>/dev/null || true)"
  [[ -n "$value" ]] && upsert_env "NEW_API_IMAGE" "$value"

  value="$(extract_running_port || true)"
  [[ -n "$value" ]] && upsert_env "NEW_API_PORT" "$value"

  for key in SQL_DSN REDIS_CONN_STRING TZ ERROR_LOG_ENABLED BATCH_UPDATE_ENABLED SESSION_SECRET CRYPTO_SECRET; do
    value="$(extract_container_env "new-api" "$key" || true)"
    if [[ -z "$value" ]]; then
      value="$(extract_legacy_env "$key" || true)"
      is_placeholder "$value" && value=""
    fi
    [[ -n "$value" ]] && upsert_env "$key" "$value"
  done

  for key in POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB; do
    value="$(extract_container_env "postgres" "$key" || true)"
    if [[ -z "$value" ]]; then
      value="$(extract_legacy_env "$key" || true)"
      is_placeholder "$value" && value=""
    fi
    [[ -n "$value" ]] && upsert_env "$key" "$value"
  done
}

load_image_tar() {
  [[ -z "$IMAGE_TAR" ]] && return 0
  [[ -f "$IMAGE_TAR" ]] || die "镜像文件不存在: $IMAGE_TAR"

  log "导入镜像文件 $IMAGE_TAR"
  case "$IMAGE_TAR" in
    *.tar.gz|*.tgz) gzip -dc "$IMAGE_TAR" | docker load >/dev/null ;;
    *.tar) docker load -i "$IMAGE_TAR" >/dev/null ;;
    *) die "仅支持 .tar / .tar.gz / .tgz" ;;
  esac
}

wait_for_health() {
  local deadline status
  deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    status="$(docker inspect new-api --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
    case "$status" in
      healthy|running)
        log "new-api 状态: $status"
        return 0
        ;;
      unhealthy|exited|dead)
        die "new-api 状态异常: $status"
        ;;
      *)
        sleep 5
        ;;
    esac
  done
  die "等待 new-api 健康检查超时"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image-tar) IMAGE_TAR="$2"; shift 2 ;;
    --image-tag|--image) IMAGE_TAG="$2"; shift 2 ;;
    --wait) WAIT_HEALTH=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数: $1" ;;
  esac
done

detect_compose
ensure_env
load_image_tar

[[ -n "$IMAGE_TAG" ]] && upsert_env "NEW_API_IMAGE" "$IMAGE_TAG"

log "启动 compose 服务"
"${compose_cmd[@]}" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --remove-orphans

[[ "$WAIT_HEALTH" -eq 1 ]] && wait_for_health

log "部署完成"
