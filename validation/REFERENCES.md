# Referências e fontes

Todas as fontes externas usadas na pesquisa e construção deste projeto,
organizadas por parte do sistema. Cada uma tem o link direto; nenhum
conteúdo dessas fontes foi copiado — só a arquitetura, os conceitos ou o
caso real foram usados como base.

## Benchmarks e pesquisa acadêmica de RCA (root cause analysis)

Usados pra desenhar a metodologia de `validation/` — especialmente a ideia
de checar a evidência citada, não só a resposta final, e de incluir um
cenário de calibração contra alucinação.

- **OpenRCA** (Microsoft, ICLR 2025) — benchmark de RCA com LLM sobre 335 casos reais de 3 sistemas enterprise
  https://github.com/microsoft/OpenRCA
- **OpenRCA 2.0** — supervisão do processo causal, não só do rótulo final
  https://arxiv.org/html/2606.27154v2
- **RCAEval** — 735 casos em 3 sistemas de microsserviços open-source
  https://www.researchgate.net/publication/392040582_RCAEval_A_Benchmark_for_Root_Cause_Analysis_of_Microservice_Systems_with_Telemetry_Data
- **AIOpsLab** (Microsoft) — framework holístico pra avaliar agentes de IA em nuvens autônomas
  https://arxiv.org/html/2606.29193v1 (citado dentro do artigo do multi-dataset benchmark)
- **CUJBench** — diagnóstico de falha cross-modal, do browser ao backend
  https://arxiv.org/html/2604.23455
- **A Multi-Dataset Benchmark for Evaluating LLM Agents in Microservice Failure Diagnosis**
  https://arxiv.org/pdf/2606.29193
- **From General Agents to RCA Experts: A Self-Evolving Harness for RCA**
  https://arxiv.org/pdf/2608.25661
- **How Far Can Root Cause Analysis Go on Real-World Telemetry Data?**
  https://arxiv.org/html/2607.13548v1
- **How Root Cause Analysis AI Agents Work in Production** (síntese de vários benchmarks, incluindo R2Act)
  https://www.augmentcode.com/guides/root-cause-analysis-ai-agents

## Coleções de postmortems públicos

Fonte pra futuros cenários de validação além dos 7 já escritos.

- **danluu/post-mortems** — a coleção original de postmortems públicos
  https://github.com/danluu/post-mortems
- **icco/postmortems** — expande a coleção acima com metadados e categorias
  https://github.com/icco/postmortems

## Casos reais por trás de cada cenário de validação

Ver `validation/scenarios/*.json` (campo `based_on`) para o mapeamento exato.

- **Cloudflare — outage de 2 de julho de 2019** (regex com backtracking catastrófico na WAF) — cenário 02
  https://blog.cloudflare.com/details-of-the-cloudflare-outage-on-july-2-2019
- **Let's Encrypt — expiração da DST Root CA X3** (setembro de 2021) — cenário 03
  https://letsencrypt.org/docs/dst-root-ca-x3-expiration-september-2021/
- **cPanel KB — DST Root CA X3 Expiration** — cenário 03
  https://support.cpanel.net/hc/en-us/articles/4409759316759-DST-Root-CA-X3-Expiration-and-Let-s-Encrypt
- **Citrix KB — SAML fails on NetScaler após rotação de certificado do Azure AD** — cenário 01
  https://support.citrix.com/external/article/CTX696606/saml-authentication-fails-on-netscaler-a.html
- **Atlassian KB — "Signature validation failed" em SAML na Confluence** — cenário 01
  https://confluence.atlassian.com/kb/received-invalid-saml-response-signature-validation-failed-saml-response-rejected-938041884.html
- **Fórum Palo Alto — SAML login issue com token-signing certificate** — cenário 01
  https://live.paloaltonetworks.com/t5/globalprotect-discussions/saml-login-issue-with-token-signing-certificate/m-p/564648
- **GitHub golang-jwt/jwt issue #98 — "Token used before issued"** — cenário 04
  https://github.com/golang-jwt/jwt/issues/98
- **Fórum Auth0 — clock skew e JWT "Cannot handle token prior to"** — cenário 04
  https://community.auth0.com/t/wp-auth0-wordpress-plugin-jwt-token-problems/11019
- **Fórum Netlify — Stripe webhook falhando após deploy** — cenário 05
  https://answers.netlify.com/t/stripe-webhook-not-working-after-deploy-on-netlify/90970
- **Fórum Django — "No signatures found matching" com DRF** — cenário 05
  https://forum.djangoproject.com/t/stripe-no-signatures-found-matching-the-expected-signature-for-payload-using-django-rest-framework-and-request-body/42482
- **dev.to — Debugging Stripe Webhook Signature Verification Errors in Production** — cenário 05
  https://dev.to/nerdincode/debugging-stripe-webhook-signature-verification-errors-in-production-1h7c
- **Status page da Apollo GraphQL — esgotamento de conexões do Postgres durante rollout** — cenário 06
  https://status.apollographql.com/incidents/1wdxpmrqbldc
- **Netdata — guia sobre "too many connections" no Postgres** — cenário 06
  https://www.netdata.cloud/guides/postgres/postgres-too-many-connections/

## Arquitetura do agente (LangGraph + MCP)

- **esxr/langgraph-mcp** — base do `agent/` (Strategy Pattern do `mcp_wrapper.py`: `MCPSessionFunction`, `GetTools`, `RunTool`)
  https://github.com/esxr/langgraph-mcp
- **NxtGenCodeBase/mcp-agent-orchestrator** — referência de arquitetura mais completa (FastMCP + LangGraph + Ollama + Streamlit)
  https://github.com/NxtGenCodeBase/mcp-agent-orchestrator
- **danmas0n/multi-agent-with-mcp** — exemplo de time de agentes usando MCP
  https://github.com/danmas0n/multi-agent-with-mcp
- **von-development/awesome-LangGraph** — índice curado do ecossistema LangGraph
  https://github.com/von-development/awesome-LangGraph
- **DZone — Building Intelligent Agents With MCP and LangGraph**
  https://dzone.com/articles/intelligent-agents-mcp-langgraph
- **langchain-mcp-adapters** (LangChain/LangGraph oficial) — citado como a lib padrão de mercado para conectar MCP a agentes LangGraph

## Boilerplates avaliados para a interface web

- **nextjs/saas-starter** — escolhido como base de `web/` (Next.js + Postgres/Drizzle + Stripe + shadcn/ui)
  https://github.com/nextjs/saas-starter
- **ixartz/SaaS-Boilerplate** — alternativa avaliada, mais completa (multi-tenancy, i18n)
  (considerado na pesquisa inicial de boilerplates)
- **LastSaaS** (jonradoff) — alternativa em Go+React com MCP nativo, avaliada e não escolhida
  (considerado na pesquisa inicial de boilerplates)

## Observabilidade

- **Elastic Common Schema (ECS)** — formato de campos (`@timestamp`, `service.name`, `log.level`, `trace.id`) usado na integração com Elasticsearch em `tools/logs.go`
  https://www.elastic.co/guide/en/ecs/current/index.html
