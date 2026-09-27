#!/usr/bin/env bash
# Sobe o mock server (Zendesk/GitHub/Datadog fake) e o servidor Go real
# (modo --http) apontando pra ele — tudo local, sem nenhuma credencial
# real. É a base pra rodar a interface web (web/) contra dados canned e
# validar o fluxo inteiro antes de mexer com APIs de verdade.
#
# Uso: ./scripts/dev-local.sh
# Ctrl+C encerra os dois processos (trap abaixo).
set -euo pipefail
cd "$(dirname "$0")/.."

MOCK_PORT=9090
SERVER_PORT=8080
INTERNAL_API_KEY=${INTERNAL_API_KEY:-dev-local-key}

echo "==> Buildando mockserver e supportability-mcp..."
go build -o /tmp/supportability-mockserver ./cmd/mockserver
go build -o /tmp/supportability-mcp .

echo "==> Subindo mock server em :$MOCK_PORT"
/tmp/supportability-mockserver -addr ":$MOCK_PORT" &
MOCK_PID=$!

cleanup() {
  echo "==> Encerrando processos..."
  kill "$MOCK_PID" "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

sleep 1

echo "==> Subindo servidor Go (modo HTTP) em :$SERVER_PORT, apontando pro mock"
INTERNAL_API_KEY="$INTERNAL_API_KEY" \
ZENDESK_API_BASE="http://localhost:$MOCK_PORT/zendesk/%s" \
GITHUB_API_BASE="http://localhost:$MOCK_PORT/github" \
DATADOG_API_BASE="http://localhost:$MOCK_PORT/datadog" \
ZENDESK_SUBDOMAIN=acme ZENDESK_EMAIL=dev@local.test ZENDESK_API_TOKEN=dev \
GITHUB_TOKEN=dev DD_API_KEY=dev DD_APP_KEY=dev \
/tmp/supportability-mcp --http ":$SERVER_PORT" &
SERVER_PID=$!

sleep 1

echo ""
echo "Pronto:"
echo "  Mock server:  http://localhost:$MOCK_PORT/healthz"
echo "  Go server:    http://localhost:$SERVER_PORT/healthz"
echo ""
echo "Agora, em outro terminal:"
echo "  docker compose up -d          # sobe o Postgres local"
echo "  cd web"
echo "  cp .env.example .env          # ajuste se necessário"
echo "  npm run db:migrate"
echo "  npm run dev"
echo "  # crie sua conta em /sign-up — NÃO rode 'npm run db:seed', ele chama"
echo "  # a API real do Stripe pra criar produtos e vai falhar sem chave real"
echo ""
echo "Pressione Ctrl+C para encerrar mock server + servidor Go."

wait
