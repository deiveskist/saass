# supportability-mcp

Servidor MCP em Go com as 5 tools do MVP v1 do Supportability AI Agent,
com integrações reais (Zendesk, GitHub, Datadog) e testes automatizados.
Falta só validar contra credenciais e tickets reais (ver seção de testes).

## Rodando localmente

```bash
go mod tidy   # baixa as dependências (precisa de internet normal)
go build -o supportability-mcp .

ZENDESK_SUBDOMAIN=suaempresa \
ZENDESK_EMAIL=voce@empresa.com \
ZENDESK_API_TOKEN=xxx \
GITHUB_TOKEN=ghp_xxx \
DD_API_KEY=xxx \
DD_APP_KEY=xxx \
./supportability-mcp
```

| Variável | Usada por |
|---|---|
| `ZENDESK_SUBDOMAIN`, `ZENDESK_EMAIL`, `ZENDESK_API_TOKEN` | `get_ticket`, `search_related_tickets` |
| `GITHUB_TOKEN` | `get_recent_deploys` |
| `LOGS_PROVIDER` | `search_logs` — `datadog` (default) ou `elasticsearch` |
| `DD_API_KEY`, `DD_APP_KEY` | `search_logs`, se `LOGS_PROVIDER=datadog` |
| `ELASTICSEARCH_URL`, `ELASTICSEARCH_INDEX` (default `logs-*`) | `search_logs`, se `LOGS_PROVIDER=elasticsearch` |
| `ELASTICSEARCH_API_KEY` **ou** `ELASTICSEARCH_USERNAME`+`ELASTICSEARCH_PASSWORD` | idem — o que estiver configurado ganha |

`search_logs` suporta dois backends, escolhidos por `LOGS_PROVIDER` — cada
instalação self-hosted configura uma vez conforme a stack que já usa, sem
precisar trocar código:

```bash
# Datadog (default)
DD_API_KEY=xxx DD_APP_KEY=xxx ./supportability-mcp

# Elasticsearch
LOGS_PROVIDER=elasticsearch \
ELASTICSEARCH_URL=https://es.suaempresa.com:9200 \
ELASTICSEARCH_API_KEY=xxx \
./supportability-mcp
```

O Elasticsearch é lido no formato ECS (Elastic Common Schema): `@timestamp`,
`service.name`, `log.level`, `message`, `trace.id` — os campos padrão que
Filebeat/APM/a maioria das integrações oficiais já produzem.

O servidor fala MCP via **stdio** por padrão — é assim que o agente Python
vai invocá-lo, como subprocesso. Também há um **modo REST** pro dashboard
web (`web/`), que não fala o protocolo MCP:

```bash
INTERNAL_API_KEY=um-segredo-seu ./supportability-mcp --http :8080
```

Isso expõe as mesmas 5 tools em `POST /api/tools/<nome>` (JSON in, JSON
out), protegidas por `Authorization: Bearer <INTERNAL_API_KEY>`, mais um
`GET /healthz`. Os dois transportes (stdio e HTTP) chamam exatamente as
mesmas funções — nenhuma lógica é duplicada entre eles.

### Métricas (Prometheus)

O modo `--http` expõe `GET /metrics` (sem auth — é assim que um Prometheus
real faz scrape; coloque atrás de firewall/rede privada se for exposto
publicamente):

- `supportability_mcp_tool_calls_total{tool, status}` — contador, `status`
  é `ok`, `tool_error` (erro de integração upstream) ou `internal_error`
- `supportability_mcp_tool_call_duration_seconds{tool}` — histograma de
  latência por tool

Útil já na fase de validação contra credenciais reais: mostra na hora se
alguma tool está lenta ou falhando mais que as outras, sem precisar ficar
lendo log.

**Ver isso num dashboard, sem configurar nada:** `docker compose up -d`
também sobe Prometheus (fazendo scrape de `:8080/metrics` a cada 10s) e
Grafana com um dashboard pré-carregado (taxa de chamadas, taxa de erro e
p95 de latência, todos por tool). Abra http://localhost:3001 — login
anônimo habilitado só pra isso ser zero-fricção em dev local; **não deixe
assim em produção** (`GF_AUTH_ANONYMOUS_ENABLED` no `docker-compose.yml`).
Só funciona com o servidor Go já rodando em `--http :8080` no host (ver
`scripts/dev-local.sh`) — o Prometheus dentro do container alcança o host
via `host.docker.internal` (`extra_hosts` cuida disso no Linux também).

### Rodando tudo localmente, sem nenhuma credencial real

Pra validar o sistema inteiro (Go MCP server + interface web) sem conta
no Zendesk/GitHub/Datadog nem no Stripe:

```bash
# Terminal 1 — mock server + servidor Go, os dois já conectados
./scripts/dev-local.sh

# Terminal 2 — Postgres local + interface web
docker compose up -d
cd web
cp .env.example .env
npm run db:migrate
npm run dev
```

Abra http://localhost:3000, crie uma conta em `/sign-up` (não rode `npm
run db:seed` — ele chama a API real do Stripe pra criar produtos e vai
falhar sem chave real) e acesse `/dashboard/investigate`. Digite qualquer
ticket ID — o `cmd/mockserver` sempre devolve os mesmos dados canned (um
ticket de SSO/SAML, 2 tickets relacionados, 2 commits, 2 linhas de log),
suficiente pra ver a UI inteira funcionando ponta a ponta.

`cmd/mockserver` existe só pra desenvolvimento — não é parte do produto.
As 3 base URLs das tools (`ZENDESK_API_BASE`, `GITHUB_API_BASE`,
`DATADOG_API_BASE`) podem ser sobrescritas por env var em qualquer
ambiente, não só em teste — é assim que o `--http` aponta pro mock em vez
das APIs reais.

**Elasticsearch é diferente dos outros três: dá pra rodar de verdade em
vez de mockar.** O `docker compose up -d` já sobe um Elasticsearch real
(porta 9200). Pra testar `search_logs` contra ele:

```bash
./monitoring/elasticsearch/seed-logs.sh   # popula com 3 logs de exemplo (formato ECS)

LOGS_PROVIDER=elasticsearch \
ELASTICSEARCH_URL=http://localhost:9200 \
ELASTICSEARCH_INDEX=logs-supportability-demo \
... /tmp/supportability-mcp --http :8080
```

O datasource Elasticsearch já vem provisionado no Grafana (junto com o
Prometheus) — abra http://localhost:3001, aba Explore, escolha
"Elasticsearch" e busque em `logs-supportability-demo`.

## Estrutura

```
.
├── main.go              # registra as 5 tools e sobe o servidor stdio (ou --http)
├── cmd/mockserver/       # mock de Zendesk/GitHub/Datadog — só pra dev local
├── scripts/dev-local.sh # sobe mockserver + servidor Go juntos, prontos pra uso
├── docker-compose.yml    # Postgres, Elasticsearch, Prometheus e Grafana locais
├── monitoring/
│   ├── prometheus.yml            # scrape config
│   ├── elasticsearch/seed-logs.sh # popula o ES local com logs de exemplo
│   └── grafana/                   # datasources (Prometheus + Elasticsearch) e dashboard pré-provisionados
├── tools/
│   ├── httpclient.go          # client HTTP compartilhado + base URLs (substituíveis em teste)
│   ├── ticket.go               # get_ticket — Zendesk (ticket + comentários + resolução de autor)
│   ├── logs.go                 # search_logs — Datadog Logs Search API v2
│   ├── related_tickets.go      # search_related_tickets — Zendesk Search API
│   ├── deploys.go              # get_recent_deploys — GitHub Commits API
│   ├── summarize.go            # summarize_investigation (sem API externa)
│   ├── *_test.go                # testes unitários com HTTP mockado (httptest)
│   └── integration_test.go     # testes manuais contra APIs reais (build tag `integration`)
├── web/                 # interface (Next.js) — ver web/README.md
└── go.mod
```

## O que já funciona vs. o que falta

- ✅ Schema de cada tool bate exatamente com `mcp-tools-schema.md`
- ✅ Registro das tools no servidor, parsing de parâmetros, serialização JSON
- ✅ Erros de integração retornam `{ "error": "..." }` em vez de derrubar o processo
- ✅ `get_ticket` — ticket + comentários com nome de autor resolvido (side-loading `include=users`, com cache e fallback por chamada individual)
- ✅ `search_related_tickets` — busca full-text via Zendesk Search API, com resumo de resolução real (último comentário do ticket, truncado a 300 chars)
- ✅ `get_recent_deploys` — commits reais via GitHub, com `pr_number` (via "pulls associated with commit") e `files_changed` para os 15 commits mais recentes da janela
- ✅ `search_logs` — busca real via Datadog Logs Search API v2
- ✅ **As 5 tools do MVP v1 estão funcionalmente completas** — nenhum TODO de integração pendente

Os testes em `tools/*_test.go` cobrem toda a lógica de parsing, erro e
enriquecimento contra servidores HTTP mockados (`go test ./...`, 8 testes,
sem nenhuma chamada de rede real) — isso valida o código, mas não substitui
testar contra as APIs reais.

**Testando contra APIs reais quando você tiver as credenciais** — sem
precisar escrever nenhum script novo:

```bash
MANUAL_TICKET_ID=12345 \
ZENDESK_SUBDOMAIN=suaempresa ZENDESK_EMAIL=voce@empresa.com ZENDESK_API_TOKEN=xxx \
go test -tags=integration ./tools/ -run TestManual_GetTicket -v
```

Cada tool tem seu próprio `TestManual_*` em `tools/integration_test.go`
(atrás da build tag `integration`, então nunca roda por acidente no CI ou
no `go test ./...` normal). Cada um pula sozinho se sua env var não estiver
setada — dá pra testar uma API de cada vez, sem precisar ter todas as
credenciais ao mesmo tempo. O comentário no topo do arquivo lista as env
vars de cada teste.

Antes de confiar no resultado com dados reais:
1. Gerar as credenciais reais (Zendesk API token, GitHub PAT, Datadog API+APP key)
2. Rodar `TestManual_*` (acima) contra 1 ticket conhecido de cada vez
3. Comparar o `summarize_investigation` gerado pelo agente com a conclusão
   que um engenheiro chegou de fato — esse é o gate antes de vender
   (critério: bater em 60-70% dos casos, ver `/areas/supportability-agent-saas.md`)

**Se a organização usa outra stack:** Jira em vez de Zendesk, ou Sentry em
vez de Datadog — só `ticket.go`/`related_tickets.go` e `logs.go` precisam
mudar; `main.go` e os schemas continuam iguais.

