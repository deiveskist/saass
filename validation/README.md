# validation/ — cenários reais pra validar o agente

Este é o gate de go/no-go do produto: **o agente acerta a causa raiz em
pelo menos 60% de casos realistas?** Cada cenário aqui é baseado num caso
público real (postmortem, fórum de suporte ou KB de fornecedor),
parafraseado e adaptado pro formato que o produto consome: um ticket de
suporte + tickets anteriores + deploys + logs.

## Os cenários

| # | Cenário | Baseado em | O que testa |
|---|---|---|---|
| 01 | SSO quebra pra um tenant após o IdP renovar o certificado SAML | Fórum Palo Alto, KBs da Citrix e Atlassian | Culpar uma causa externa, não o deploy nosso mais recente |
| 02 | 502 global após deploy de regra WAF com regex que faz backtracking | Postmortem da Cloudflare (julho de 2019) | Correlacionar pico de CPU com o deploy certo, em minutos |
| 03 | Parceiro legado falha no TLS depois que a raiz cruzada da CA expirou | Docs do Let's Encrypt, KB do cPanel | Reconhecer que *nenhum* deploy nosso é o culpado |
| 04 | 401 intermitente "token used before issued" por NTP bloqueado | Fórum Auth0, issue do golang-jwt | Juntar pistas fracas: um host só, aviso de NTP, deploy de infra |
| 05 | Pedidos pagos presos: refactor quebrou o raw body do webhook Stripe | Fóruns Netlify e Django, post no dev.to | Escolher entre duas causas conhecidas do mesmo erro usando o timing |
| 06 | Conexões do Postgres esgotadas durante um rollout com maxSurge 100% | Status page da Apollo, guia da Netdata | Explicar um incidente já resolvido (pedido de RCA) |
| 07 | "Está lento" sem evidência nenhuma de problema no servidor | Achados dos benchmarks de RCA com LLM | **Não alucinar**: a resposta certa é baixa confiança e pedir dados |

Todo cenário tem **distrações de propósito**: deploys irrelevantes perto
do horário do problema e tickets antigos com a mesma sintoma mas outra
causa. Sem isso, "culpe o último deploy" acertaria quase tudo e o teste
não provaria nada.

As URLs de cada fonte estão no campo `based_on` de cada JSON.

## Como a pontuação funciona (`scoring.py`)

Um cenário passa se as quatro condições forem verdadeiras:

1. **Causa raiz**: a hipótese bate com um dos grupos de palavras-chave de `accept_any`.
2. **Evidência**: o agente cita pelo menos metade de `required_evidence` (o SHA do deploy certo, a linha de log certa…).
3. **Sem red herring**: a hipótese não culpa nada de `must_not_blame`.
4. **Confiança calibrada**: a confiança declarada está em `expected_confidence`.

O critério 2 existe porque os benchmarks acadêmicos de RCA com LLM
mostram que muitas respostas certas vêm de raciocínio errado. Checar só a
resposta final superestima o agente.

**Limitação conhecida:** casar palavras-chave é grosseiro. O próximo passo
é um juiz LLM comparando a hipótese com o `root_cause` em linguagem
natural, mantendo as palavras-chave como checagem barata e determinística.

## Rodando

A partir da raiz do repo:

```bash
go build -o /tmp/ticketlens . && go build -o /tmp/ticketlens-mockserver ./cmd/mockserver
cd agent && python3 -m venv .venv && .venv/bin/pip install -e ".[test]" && cd ..

# 1. Sem LLM, sem custo: consistência dos cenários e do scoring
agent/.venv/bin/pytest validation/tests

# 2. Sem LLM: cada cenário é resolvível pelas tools reais?
MCP_SERVER_BINARY=/tmp/ticketlens MOCKSERVER_BINARY=/tmp/ticketlens-mockserver \
  agent/.venv/bin/python validation/run_validation.py --check

# 3. O teste de verdade: roda o agente em todos os cenários (precisa ANTHROPIC_API_KEY)
ANTHROPIC_API_KEY=... MCP_SERVER_BINARY=/tmp/ticketlens MOCKSERVER_BINARY=/tmp/ticketlens-mockserver \
  agent/.venv/bin/python validation/run_validation.py
```

O modo 3 salva o detalhe de cada cenário (hipótese, evidência citada,
número de chamadas de tool, motivo da falha) em `validation/results/`,
que fica fora do git. Rode mais de uma vez: LLMs variam entre execuções,
e um resultado único de 7 cenários tem muito ruído.

`--only 04` roda só os cenários cujo nome contém `04`.

## Por que não usar só os benchmarks acadêmicos

Pesquisei os benchmarks públicos antes de escrever estes cenários:

- **OpenRCA** (Microsoft, ICLR 2025), **RCAEval** e **AIOpsLab** (Microsoft):
  RCA em microsserviços com gigabytes de métricas, traces e logs,
  geralmente com falhas injetadas por chaos engineering.
- **danluu/post-mortems** e **icco/postmortems**: coleções de postmortems
  públicos, categorizados (config change, cascading failure, time…).

Os benchmarks medem outra tarefa: a entrada deles é telemetria bruta, e a
do nosso produto é **um ticket de suporte** com contexto ao redor. Por isso
estes cenários pegam três ideias deles (ground truth rotulado, checagem da
evidência além da resposta, e um caso de calibração contra alucinação) e
aplicam no formato de ticket. As coleções de postmortems são uma boa fonte
pra criar os próximos cenários.

## Próximos passos

- **Juiz LLM** no lugar das palavras-chave (ver limitação acima).
- **Mais cenários**, com meta de 15 a 20 pra resultados mais estáveis. As
  categorias de postmortem ainda não cobertas: falha em cascata, DNS,
  bugs de horário de verão/fuso, e limites de rate de API de terceiros.
- **Tickets reais anonimizados** de um design partner, que são a validação
  que mais pesa numa venda. Não use tickets do seu empregador atual.
