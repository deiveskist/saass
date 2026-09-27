#!/usr/bin/env bash
# Popula o Elasticsearch local (docker-compose) com alguns logs de exemplo
# no formato ECS (Elastic Common Schema) — os mesmos campos que
# tools/logs.go espera ao ler de um cluster real (@timestamp, service.name,
# log.level, message, trace.id). Sem isso, um Elasticsearch recém-criado
# está vazio e search_logs sempre devolve zero resultados.
#
# Uso: ./monitoring/elasticsearch/seed-logs.sh
# (assume Elasticsearch em http://localhost:9200, como no docker-compose.yml)
set -euo pipefail

ES_URL=${ELASTICSEARCH_URL:-http://localhost:9200}
INDEX="logs-ticketlens-demo"

echo "==> Aguardando Elasticsearch ficar pronto em $ES_URL..."
until curl -sf "$ES_URL/_cluster/health" > /dev/null; do
  sleep 2
done

echo "==> Indexando logs de exemplo em '$INDEX'..."

now_minus() {
  # segundos atrás -> timestamp ISO8601, compatível com GNU e BSD date
  if date -v-1S >/dev/null 2>&1; then
    date -u -v-"${1}"S +"%Y-%m-%dT%H:%M:%SZ"
  else
    date -u -d "-${1} seconds" +"%Y-%m-%dT%H:%M:%SZ"
  fi
}

index_doc() {
  curl -s -o /dev/null -X POST "$ES_URL/$INDEX/_doc" \
    -H "Content-Type: application/json" \
    -d "$1"
}

index_doc "{\"@timestamp\":\"$(now_minus 5400)\",\"service\":{\"name\":\"auth-service\"},\"log\":{\"level\":\"error\"},\"message\":\"SAML assertion validation failed: signature mismatch\",\"trace\":{\"id\":\"trace-es-demo-1\"}}"
index_doc "{\"@timestamp\":\"$(now_minus 5300)\",\"service\":{\"name\":\"auth-service\"},\"log\":{\"level\":\"info\"},\"message\":\"Retrying SAML validation with cached metadata\",\"trace\":{\"id\":\"trace-es-demo-1\"}}"
index_doc "{\"@timestamp\":\"$(now_minus 3600)\",\"service\":{\"name\":\"api-gateway\"},\"log\":{\"level\":\"warn\"},\"message\":\"Upstream auth-service latency above threshold (820ms)\",\"trace\":{\"id\":\"trace-es-demo-2\"}}"

curl -s -X POST "$ES_URL/$INDEX/_refresh" > /dev/null

echo "==> Pronto. Teste com:"
echo "    LOGS_PROVIDER=elasticsearch ELASTICSEARCH_URL=$ES_URL ELASTICSEARCH_INDEX=$INDEX ..."
