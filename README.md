# supportability-mcp

Servidor MCP em Go com as 5 tools do MVP v1 do Supportability AI Agent.
Compilado e testado com `go build ./...` (Go 1.23) — só falta plugar as
chamadas reais de API onde tem `TODO`.

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
| `DD_API_KEY`, `DD_APP_KEY` | `search_logs` |

O servidor fala MCP via **stdio** — é assim que o agente Python vai
invocá-lo, como subprocesso, sem precisar expor porta HTTP no v1.

## Estrutura

```
.
├── main.go              # registra as 5 tools e sobe o servidor stdio
├── tools/
│   ├── ticket.go             # get_ticket — implementação de referência completa
│   ├── logs.go                # search_logs (stub — plugar Datadog/Sentry)
│   ├── related_tickets.go     # search_related_tickets (stub)
│   ├── deploys.go             # get_recent_deploys (stub — plugar GitHub API)
│   └── summarize.go           # summarize_investigation (sem API externa)
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

**Único gap real: nada foi testado contra credenciais de verdade ainda.**
Os testes em `tools/*_test.go` cobrem toda a lógica de parsing, erro e
enriquecimento contra servidores HTTP mockados (`go test ./...`, 8 testes,
sem nenhuma chamada de rede real) — isso valida o código, mas não substitui
testar contra as APIs reais. Antes de confiar no resultado com dados reais:
1. Gerar as credenciais reais (Zendesk API token, GitHub PAT, Datadog API+APP key)
2. Rodar o servidor e chamar cada tool manualmente contra 1 ticket conhecido
3. Comparar o `summarize_investigation` gerado pelo agente com a conclusão
   que um engenheiro chegou de fato — esse é o gate antes de vender
   (critério: bater em 60-70% dos casos, ver `/areas/supportability-agent-saas.md`)

**Se a organização usa outra stack:** Jira em vez de Zendesk, ou Sentry em
vez de Datadog — só `ticket.go`/`related_tickets.go` e `logs.go` precisam
mudar; `main.go` e os schemas continuam iguais.

## Próximo passo depois de plugar as APIs reais

Rodar contra 15-20 tickets reais e comparar o `summarize_investigation`
gerado com a conclusão que um engenheiro chegou de fato — esse é o gate
antes de qualquer decisão de vender (ver `/areas/supportability-agent-saas.md`).
