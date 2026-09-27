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
- ✅ `get_ticket` — chamada real ao Zendesk (ticket + comentários, 2 requests)
- ✅ `search_related_tickets` — busca real via Zendesk Search API (full-text,
  sem ranking semântico; suficiente pro v1)
- ✅ `get_recent_deploys` — lista commits reais via GitHub Commits API
- ✅ `search_logs` — busca real via Datadog Logs Search API v2
- ⏳ Nenhuma chamada real foi testada contra credenciais de verdade ainda —
  só compila e passa no `go vet`. Teste com um ticket real antes de confiar
  no resultado.
- ⏳ `get_recent_deploys` não traz `pr_number` nem `files_changed` (custaria
  uma chamada extra por commit) — deixado como TODO no código
- ⏳ `search_related_tickets` não resolve o resumo de resolução automaticamente
  — sinaliza que está resolvido e deixa o agente chamar `get_ticket` se quiser
  os comentários de fechamento
- ⏳ Se a organização usa Jira em vez de Zendesk, ou Sentry em vez de Datadog,
  os arquivos `ticket.go`/`related_tickets.go` e `logs.go` são os únicos que
  precisam mudar — o resto (main.go, schemas) não depende da escolha

## Próximo passo depois de plugar as APIs reais

Rodar contra 15-20 tickets reais e comparar o `summarize_investigation`
gerado com a conclusão que um engenheiro chegou de fato — esse é o gate
antes de qualquer decisão de vender (ver `/areas/supportability-agent-saas.md`).
