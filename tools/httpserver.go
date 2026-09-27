package tools

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ServeHTTP sobe um servidor REST fino sobre as mesmas 5 tools do MCP —
// existe só pra dar ao frontend (Next.js) um jeito simples de chamar as
// tools sem falar o protocolo MCP inteiro (que é pensado pra um agente de
// LLM, não pra um browser). O agente Python continua falando com o
// servidor via stdio normalmente; isso é uma porta adicional, não uma
// substituição.
//
// Toda rota exige o header Authorization: Bearer <INTERNAL_API_KEY> —
// essas tools tocam dados de cliente (tickets, logs), não podem ficar
// abertas na rede.
func ServeHTTP(addr string) error {
	apiKey := os.Getenv("INTERNAL_API_KEY")
	if apiKey == "" {
		log.Fatal("INTERNAL_API_KEY must be set to run in HTTP mode")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tools/get_ticket", wrapHandler(apiKey, handleGetTicket))
	mux.HandleFunc("/api/tools/search_related_tickets", wrapHandler(apiKey, handleSearchRelatedTickets))
	mux.HandleFunc("/api/tools/get_recent_deploys", wrapHandler(apiKey, handleGetRecentDeploys))
	mux.HandleFunc("/api/tools/search_logs", wrapHandler(apiKey, handleSearchLogs))
	mux.HandleFunc("/api/tools/summarize_investigation", wrapHandler(apiKey, handleSummarizeInvestigation))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, corsMiddleware(mux))
}

// toolHandler é a assinatura comum de todo handleXxx em tools/*.go —
// reaproveitá-la aqui evita duplicar lógica de negócio: a rota REST chama
// exatamente o mesmo código que o transporte MCP/stdio chamaria.
type toolHandler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// wrapHandler adapta um handler de tool MCP pra rota HTTP: decodifica o
// body JSON como os argumentos da tool, monta um mcp.CallToolRequest
// (o mesmo tipo que o servidor MCP de verdade constrói) e devolve o
// conteúdo já como JSON puro — sem o envelope de content blocks do MCP,
// que só faz sentido pro lado do agente/LLM.
func wrapHandler(apiKey string, handler toolHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !validAuth(r, apiKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var args map[string]any
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		result, err := handler(r.Context(), mcp.CallToolRequest{
			Params: mcp.CallToolParams{Arguments: args},
		})
		if err != nil {
			// Erro Go de verdade (bug, não erro de integração) — os
			// handlers já tratam erro de API externa via IsError abaixo.
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if result.IsError {
			w.WriteHeader(http.StatusBadGateway) // erro de integração upstream (Zendesk/GitHub/Datadog)
		}

		// O texto de cada tool já é um JSON válido (ou {"error": "..."})
		// — repassamos como está, sem re-serializar.
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(mcp.TextContent); ok {
				w.Write([]byte(tc.Text))
				return
			}
		}
		w.Write([]byte(`{}`))
	}
}

func validAuth(r *http.Request, apiKey string) bool {
	auth := r.Header.Get("Authorization")
	return strings.TrimPrefix(auth, "Bearer ") == apiKey && auth != ""
}

// corsMiddleware libera só o necessário pro dashboard Next.js chamar essa
// API a partir do browser em dev local. Em produção, restrinja
// ALLOWED_ORIGIN a um domínio real em vez de "*".
func corsMiddleware(next http.Handler) http.Handler {
	origin := os.Getenv("ALLOWED_ORIGIN")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
