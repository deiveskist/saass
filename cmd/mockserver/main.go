// cmd/mockserver simula as 3 APIs externas (Zendesk, GitHub, Datadog) que
// as tools em tools/*.go chamam, servindo dados canned. Existe só pra
// validar o fluxo completo (Go MCP server + interface web) localmente,
// sem precisar de nenhuma credencial real — os testes em tools/*_test.go
// já cobrem a lógica de cada tool isoladamente; este mock serve pra ver o
// sistema inteiro funcionando de ponta a ponta.
//
// Uso: `go run ./cmd/mockserver` (porta 9090 por padrão, `-addr` pra mudar)
// e então aponte o servidor real pra ele:
//
//	ZENDESK_API_BASE=http://localhost:9090/zendesk/%s \
//	GITHUB_API_BASE=http://localhost:9090/github \
//	DATADOG_API_BASE=http://localhost:9090/datadog \
//	./supportability-mcp --http :8080
//
// Ver web/README.md ou README.md (raiz) para o passo a passo completo.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", ":9090", "endereço para o mock server escutar")
	scenarioPath := flag.String("scenario", "", "caminho de um validation/scenarios/*.json; se vazio, usa os dados fixos")
	flag.Parse()

	mux := http.NewServeMux()
	if *scenarioPath != "" {
		s, err := loadScenario(*scenarioPath)
		if err != nil {
			log.Fatalf("falha ao carregar cenário: %v", err)
		}
		registerScenarioMocks(mux, s)
		log.Printf("modo cenário: %s (ticket #%d)", s.ID, s.Ticket.ID)
	} else {
		registerZendeskMocks(mux)
		registerGitHubMocks(mux)
		registerDatadogMocks(mux)
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("mock server (Zendesk + GitHub + Datadog) escutando em %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, logRequests(mux)))
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Zendesk ---
//
// zendeskAPIBase real é "https://%s.zendesk.com/api/v2"; aqui vira
// "http://localhost:9090/zendesk/{subdomain}/..." — o ServeMux do Go não
// aceita misturar texto literal com um wildcard no mesmo segmento de path
// (não dá pra fazer "/zendesk-{subdomain}"), por isso o subdomain vira seu
// próprio segmento. O valor do subdomínio em si não importa pro mock.
func registerZendeskMocks(mux *http.ServeMux) {
	// {id} aqui casa com "42.json" inteiro (é um segmento só, sem barra
	// entre o número e o ".json") — por isso o handler tira o sufixo.
	mux.HandleFunc("GET /zendesk/{subdomain}/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimSuffix(r.PathValue("id"), ".json")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			id = 0
		}
		writeJSON(w, map[string]any{
			"ticket": map[string]any{
				"id":          id,
				"subject":     "Login fails after SSO redirect (mock)",
				"description": "Customer reports HTTP 500 right after being redirected back from the IdP.",
				"status":      "open",
				"priority":    "high",
				"created_at":  time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
				"tags":        []string{"sso", "prod", "mock"},
			},
		})
	})

	mux.HandleFunc("GET /zendesk/{subdomain}/tickets/{id}/comments.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"comments": []map[string]any{
				{"author_id": 1, "plain_body": "Investigating — seeing SAML assertion errors in the logs.", "created_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339)},
				{"author_id": 2, "plain_body": "Confirmed: cert rotated yesterday, updating the IdP metadata now.", "created_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339)},
			},
			"users": []map[string]any{
				{"id": 1, "name": "Priya (Support Eng)"},
				{"id": 2, "name": "Marcus (Platform Team)"},
			},
		})
	})

	mux.HandleFunc("GET /zendesk/{subdomain}/search.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []map[string]any{
				{"id": 9001, "subject": "SSO redirect loop after cert rotation (mock)", "status": "solved"},
				{"id": 9002, "subject": "SAML metadata out of sync (mock)", "status": "open"},
			},
		})
	})

	mux.HandleFunc("GET /zendesk/{subdomain}/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"user": map[string]any{"name": "Mock User"}})
	})
}

// --- GitHub ---
func registerGitHubMocks(mux *http.ServeMux) {
	mux.HandleFunc("GET /github/repos/{owner}/{repo}/commits", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{
				"sha": "abc123mock",
				"commit": map[string]any{
					"message": "fix: rotate SAML signing certificate",
					"author":  map[string]any{"name": "Marcus", "date": time.Now().Add(-20 * time.Hour).Format(time.RFC3339)},
				},
			},
			{
				"sha": "def456mock",
				"commit": map[string]any{
					"message": "chore: bump auth library",
					"author":  map[string]any{"name": "Deives", "date": time.Now().Add(-30 * time.Hour).Format(time.RFC3339)},
				},
			},
		})
	})

	mux.HandleFunc("GET /github/repos/{owner}/{repo}/commits/{sha}/pulls", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{{"number": 512}})
	})

	// Precisa vir depois da rota "/pulls" acima — senão o pattern mais
	// genérico (sem sufixo) casaria primeiro. O ServeMux do Go 1.22+
	// resolve isso pela especificidade do path, não pela ordem de
	// registro, mas deixamos explícito aqui pra não depender disso.
	mux.HandleFunc("GET /github/repos/{owner}/{repo}/commits/{sha}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"files": []map[string]any{
				{"filename": "auth/saml.go"},
				{"filename": "auth/saml_test.go"},
			},
		})
	})
}

// --- Datadog ---
func registerDatadogMocks(mux *http.ServeMux) {
	mux.HandleFunc("POST /datadog/api/v2/logs/events/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"data": []map[string]any{
				{
					"attributes": map[string]any{
						"timestamp":  time.Now().Add(-90 * time.Minute).Format(time.RFC3339),
						"service":    "auth-service",
						"status":     "error",
						"message":    "SAML assertion validation failed: signature mismatch (mock)",
						"attributes": map[string]any{"trace_id": "trace-mock-789"},
					},
				},
				{
					"attributes": map[string]any{
						"timestamp":  time.Now().Add(-85 * time.Minute).Format(time.RFC3339),
						"service":    "auth-service",
						"status":     "info",
						"message":    "Retrying SAML validation with cached metadata (mock)",
						"attributes": map[string]any{"trace_id": "trace-mock-789"},
					},
				},
			},
		})
	})
}
