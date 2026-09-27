# ticketlens-agent

Agente Python (LangGraph) que orquestra as 5 tools do `ticketlens`
(servidor Go na raiz do repo) pra investigar um ticket de suporte sozinho,
do início ao fim.

Pasta **totalmente separada** do Go (`../tools`, `../main.go`) e do
frontend (`../web`) — tem seu próprio ambiente Python, suas próprias
dependências, e fala com o servidor Go só via **stdio** (o mesmo
protocolo MCP), nunca importando código Go diretamente.

## Base arquitetural

O wrapper genérico de MCP (`mcp_wrapper.py`) segue o **Strategy Pattern**
do projeto [esxr/langgraph-mcp](https://github.com/esxr/langgraph-mcp)
(MIT) — uma classe abstrata `MCPSessionFunction` com implementações
concretas (`GetTools`, `RunTool`) plugadas numa função `apply()` que abre
a sessão stdio, roda a operação e fecha.

**Simplificado em relação ao original:** o projeto de referência resolve
um problema mais difícil — rotear entre *vários* servidores MCP
desconhecidos, indexando suas tools num vector DB (Milvus) pra decidir
qual servidor usar. Aqui isso não existe porque só há **um** servidor,
com exatamente 5 tools conhecidas — então não precisamos de router, de
índice vetorial, nem do node de roteamento do projeto original. O grafo
do agente em si usa `create_react_agent` do próprio LangGraph em vez de
reimplementar o loop de decisão na mão.

## Estrutura

```
agent/
├── pyproject.toml
├── .env.example
├── src/ticketlens_agent/
│   ├── mcp_wrapper.py    # Strategy pattern: MCPSessionFunction, GetTools, RunTool, apply()
│   ├── tools_bridge.py    # converte as tool specs do MCP em StructuredTool do LangChain
│   ├── investigate.py     # monta o agente (create_react_agent) e roda uma investigação
│   └── cli.py             # `python -m ticketlens_agent.cli <ticket_id>`
└── tests/
    └── test_mcp_wrapper.py  # testa contra o binário Go real (com o mock server como backend)
```

## Rodando

```bash
cd agent
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
cp .env.example .env   # ajuste MCP_SERVER_BINARY e a chave do LLM
python -m ticketlens_agent.cli 42
```

`MCP_SERVER_BINARY` deve apontar pro binário já compilado do Go
(`go build -o ticketlens .` na raiz do repo) — o agente sobe esse
binário como subprocesso via stdio a cada investigação, e repassa as
credenciais (`ZENDESK_*`, `GITHUB_TOKEN`, `DD_*`/`ELASTICSEARCH_*`) do seu
próprio ambiente pra ele.

## Testando sem credenciais reais

Os testes usam o mesmo `cmd/mockserver` (Go) que o resto do projeto já
tem — sobe o mock, aponta o binário do ticketlens pra ele (as
mesmas env vars `ZENDESK_API_BASE`/`GITHUB_API_BASE`/`DATADOG_API_BASE` de
`../scripts/dev-local.sh`), e valida que o `GetTools`/`RunTool` do
wrapper conversam corretamente com o processo Go de verdade — sem
precisar de LLM nem de credencial nenhuma:

```bash
pip install -e ".[test]"
MCP_SERVER_BINARY=/tmp/ticketlens pytest
```
