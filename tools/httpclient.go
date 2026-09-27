package tools

import (
	"encoding/base64"
	"net/http"
	"os"
	"time"
)

// httpClient é compartilhado por todas as tools que chamam API externa.
// Timeout curto de propósito: numa investigação o agente chama várias
// tools em sequência, uma API lenta não pode travar tudo.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// Base URLs como vars (não const) de propósito: os testes substituem
// esses valores por endereços de httptest.Server, evitando qualquer
// chamada de rede real durante `go test`. Em runtime normal (não-teste),
// cada uma pode ser sobrescrita por env var — é assim que o servidor real
// aponta pro cmd/mockserver em vez do Zendesk/GitHub/Datadog de verdade,
// pra validação local sem nenhuma credencial real. zendeskAPIBase mantém
// o %s (vira parte do path no mock, não precisa ser um host de verdade).
var (
	zendeskAPIBase = envOr("ZENDESK_API_BASE", "https://%s.zendesk.com/api/v2")
	githubAPIBase  = envOr("GITHUB_API_BASE", "https://api.github.com")
	datadogAPIBase = envOr("DATADOG_API_BASE", "https://api.datadoghq.com")
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// zendeskBasicAuth monta o header Authorization no formato que a API do
// Zendesk espera: Basic base64("email/token:api_token").
func zendeskBasicAuth(email, apiToken string) string {
	raw := email + "/token:" + apiToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}
