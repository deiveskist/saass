package tools

import (
	"encoding/base64"
	"net/http"
	"time"
)

// httpClient é compartilhado por todas as tools que chamam API externa.
// Timeout curto de propósito: numa investigação o agente chama várias
// tools em sequência, uma API lenta não pode travar tudo.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// zendeskBasicAuth monta o header Authorization no formato que a API do
// Zendesk espera: Basic base64("email/token:api_token").
func zendeskBasicAuth(email, apiToken string) string {
	raw := email + "/token:" + apiToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}
