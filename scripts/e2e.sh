#!/usr/bin/env bash
# Sobe um Postgres isolado (projeto compose "truco-e2e") + o backend servindo o build do
# frontend, roda o Playwright e derruba tudo (inclusive o volume do banco de e2e).
set -euo pipefail
cd "$(dirname "$0")/.."

PROJECT=truco-e2e
export POSTGRES_PORT="${E2E_POSTGRES_PORT:-55432}"
HTTP_PORT="${E2E_HTTP_PORT:-8090}"

tmp=$(mktemp -d)
server=""
cleanup() {
  [ -n "$server" ] && kill "$server" 2>/dev/null || true
  [ -n "$server" ] && wait "$server" 2>/dev/null || true
  docker compose -p "$PROJECT" down -v >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT

docker compose -p "$PROJECT" up -d --wait postgres

(cd frontend && pnpm build)
(cd backend && go build -o "$tmp/server" ./cmd/server)

log="$tmp/server.log"
export DATABASE_URL="postgres://truco:truco@localhost:${POSTGRES_PORT}/truco?sslmode=disable"
export HTTP_ADDR=":${HTTP_PORT}"
export APP_BASE_URL="http://localhost:${HTTP_PORT}"
export COOKIE_SECURE=false
# o cadastro limita 5/hora por IP: cada jogador do e2e manda um X-Forwarded-For próprio
export TRUST_PROXY=true
export ALLOWED_EMAIL_DOMAINS="e2e.test"
export MAIL_DRIVER=log
export LOG_FORMAT=json
export LOG_LEVEL=info
export MIGRATE_ON_START=true
export STATIC_DIR="$PWD/frontend/dist"
# 5 s é o mínimo aceito pela config (a spec dizia 3 s)
export TIMER_PLAY_SECONDS=5 TIMER_VOTE_SECONDS=5 TIMER_CHOICE_SECONDS=5 TIMER_BURN_SECONDS=5
export TIMER_ABSENT_SECONDS=2 HAND_PAUSE_SECONDS=1
export RANKING_MIN_MATCHES=1

"$tmp/server" serve >"$log" 2>&1 &
server=$!

ok=""
for _ in $(seq 1 60); do
  if curl -fsS "http://localhost:${HTTP_PORT}/healthz" >/dev/null 2>&1; then ok=1; break; fi
  kill -0 "$server" 2>/dev/null || break
  sleep 1
done
if [ -z "$ok" ]; then
  echo "servidor não subiu; log:" >&2
  cat "$log" >&2
  exit 1
fi

(cd frontend && E2E_BASE_URL="http://localhost:${HTTP_PORT}" E2E_SERVER_LOG="$log" pnpm exec playwright test "$@")
