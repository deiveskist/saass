//go:build integration

// Este arquivo só compila com `go test -tags=integration`. Ele chama as
// APIs reais do Zendesk/GitHub/Datadog usando as credenciais já
// configuradas no ambiente (as mesmas do README) — não usa mocks.
//
// Cada subteste roda de forma independente e pula sozinho se a variável
// de ambiente correspondente não estiver setada, então dá pra testar uma
// tool de cada vez sem precisar ter todas as credenciais ao mesmo tempo.
//
// Exemplos:
//
//	# só testar get_ticket contra um ticket real:
//	MANUAL_TICKET_ID=12345 \
//	ZENDESK_SUBDOMAIN=suaempresa ZENDESK_EMAIL=voce@empresa.com ZENDESK_API_TOKEN=xxx \
//	go test -tags=integration ./tools/ -run TestManual_GetTicket -v
//
//	# testar tudo de uma vez (precisa de todas as env vars):
//	MANUAL_TICKET_ID=12345 \
//	MANUAL_RELATED_QUERY="login timeout" \
//	MANUAL_REPO=suaorg/seurepo MANUAL_SINCE=2026-09-20T00:00:00Z \
//	MANUAL_LOG_QUERY="status:error" \
//	ZENDESK_SUBDOMAIN=... ZENDESK_EMAIL=... ZENDESK_API_TOKEN=... \
//	GITHUB_TOKEN=... DD_API_KEY=... DD_APP_KEY=... \
//	go test -tags=integration ./tools/ -run TestManual -v
package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// printResult decodifica e imprime o resultado formatado — o objetivo aqui
// é ler o output com os próprios olhos, não fazer assertions rígidas
// contra dados reais e variáveis.
func printResult(t *testing.T, label string, result any) {
	t.Helper()
	pretty, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("\n=== %s ===\n%s\n", label, string(pretty))
}

func TestManual_GetTicket(t *testing.T) {
	ticketID := os.Getenv("MANUAL_TICKET_ID")
	if ticketID == "" {
		t.Skip("MANUAL_TICKET_ID not set — skipping")
	}

	result, err := handleGetTicket(context.Background(), callTool(t, map[string]any{
		"ticket_id": ticketID,
	}))
	if err != nil {
		t.Fatalf("Go error (não deveria acontecer mesmo com API real fora do ar): %v", err)
	}
	printResult(t, "get_ticket", result)
	if result.IsError {
		t.Errorf("tool retornou erro — veja o log acima pra entender por quê")
	}
}

func TestManual_SearchRelatedTickets(t *testing.T) {
	query := os.Getenv("MANUAL_RELATED_QUERY")
	if query == "" {
		t.Skip("MANUAL_RELATED_QUERY not set — skipping")
	}

	result, err := handleSearchRelatedTickets(context.Background(), callTool(t, map[string]any{
		"query":             query,
		"exclude_ticket_id": os.Getenv("MANUAL_TICKET_ID"), // opcional
	}))
	if err != nil {
		t.Fatalf("Go error: %v", err)
	}
	printResult(t, "search_related_tickets", result)
	if result.IsError {
		t.Errorf("tool retornou erro — veja o log acima")
	}
}

func TestManual_GetRecentDeploys(t *testing.T) {
	repo := os.Getenv("MANUAL_REPO")
	if repo == "" {
		t.Skip("MANUAL_REPO not set — skipping")
	}
	since := os.Getenv("MANUAL_SINCE")
	if since == "" {
		// default: últimas 48h, formato RFC3339 exigido pela GitHub API
		since = time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	}

	result, err := handleGetRecentDeploys(context.Background(), callTool(t, map[string]any{
		"repository": repo,
		"since":      since,
	}))
	if err != nil {
		t.Fatalf("Go error: %v", err)
	}
	printResult(t, "get_recent_deploys", result)
	if result.IsError {
		t.Errorf("tool retornou erro — veja o log acima")
	}
}

func TestManual_SearchLogs(t *testing.T) {
	query := os.Getenv("MANUAL_LOG_QUERY")
	if query == "" {
		t.Skip("MANUAL_LOG_QUERY not set — skipping")
	}

	now := time.Now()
	result, err := handleSearchLogs(context.Background(), callTool(t, map[string]any{
		"query":      query,
		"start_time": now.Add(-1 * time.Hour).Format(time.RFC3339),
		"end_time":   now.Format(time.RFC3339),
	}))
	if err != nil {
		t.Fatalf("Go error: %v", err)
	}
	printResult(t, "search_logs", result)
	if result.IsError {
		t.Errorf("tool retornou erro — veja o log acima")
	}
}
