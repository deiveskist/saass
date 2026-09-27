package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// withMockZendesk sobe um httptest.Server, aponta zendeskAPIBase pra ele
// e devolve uma função de cleanup que restaura o valor original — chame
// sempre com `defer` logo após a chamada.
func withMockZendesk(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	server := httptest.NewServer(handler)
	original := zendeskAPIBase
	zendeskAPIBase = server.URL + "/api-%s" // subdomain vira parte inócua do path

	t.Setenv("ZENDESK_SUBDOMAIN", "acme")
	t.Setenv("ZENDESK_EMAIL", "eng@acme.com")
	t.Setenv("ZENDESK_API_TOKEN", "fake-token")

	return func() {
		zendeskAPIBase = original
		server.Close()
	}
}

func callTool(t *testing.T, args map[string]any) mcp.CallToolRequest {
	t.Helper()
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: args},
	}
}

func TestHandleGetTicket_Success(t *testing.T) {
	cleanup := withMockZendesk(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/tickets/42.json"):
			json.NewEncoder(w).Encode(map[string]any{
				"ticket": map[string]any{
					"id": 42, "subject": "Login fails after SSO redirect",
					"description": "Customer reports 500 after SSO", "status": "open",
					"priority": "high", "created_at": "2026-09-20T10:00:00Z",
					"tags": []string{"sso", "prod"},
				},
			})
		case strings.Contains(r.URL.Path, "/tickets/42/comments.json"):
			json.NewEncoder(w).Encode(map[string]any{
				"comments": []map[string]any{
					{"author_id": 7, "plain_body": "Investigating now", "created_at": "2026-09-20T10:05:00Z"},
				},
				"users": []map[string]any{
					{"id": 7, "name": "Priya (Support Eng)"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	result, err := handleGetTicket(context.Background(), callTool(t, map[string]any{"ticket_id": "42"}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got tool error: %+v", result.Content)
	}

	var ticket Ticket
	unmarshalToolResult(t, result, &ticket)

	if ticket.Title != "Login fails after SSO redirect" {
		t.Errorf("unexpected title: %q", ticket.Title)
	}
	if len(ticket.Comments) != 1 || ticket.Comments[0].Author != "Priya (Support Eng)" {
		t.Errorf("expected author resolved via side-loading, got: %+v", ticket.Comments)
	}
}

func TestHandleGetTicket_ZendeskDown(t *testing.T) {
	cleanup := withMockZendesk(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	result, err := handleGetTicket(context.Background(), callTool(t, map[string]any{"ticket_id": "42"}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	// O contrato importante do agente: falha de integração vira resultado
	// MCP de erro, nunca um erro Go que derrubaria o processo do servidor.
	if !result.IsError {
		t.Fatalf("expected tool-level error when Zendesk returns 500, got success")
	}
}

func TestHandleGetTicket_MissingTicketID(t *testing.T) {
	result, err := handleGetTicket(context.Background(), callTool(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected validation error when ticket_id is missing")
	}
}

// unmarshalToolResult extrai o texto do primeiro content block e decodifica
// como JSON — repetido em todos os testes de tool, então fica aqui.
func unmarshalToolResult(t *testing.T, result *mcp.CallToolResult, out any) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("tool result has no content")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if err := json.Unmarshal([]byte(textContent.Text), out); err != nil {
		t.Fatalf("failed to unmarshal tool result: %v\nraw: %s", err, textContent.Text)
	}
}
