package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestHandleSearchRelatedTickets_ExcludesCurrentAndFetchesResolution(t *testing.T) {
	cleanup := withMockZendesk(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/search.json"):
			respondJSON(w, map[string]any{
				"results": []map[string]any{
					{"id": 42, "subject": "current ticket, should be excluded", "status": "open"},
					{"id": 10, "subject": "SSO redirect loop", "status": "solved"},
					{"id": 11, "subject": "SSO cert expired", "status": "open"},
				},
			})
		case strings.Contains(r.URL.Path, "/tickets/10/comments.json"):
			respondJSON(w, map[string]any{
				"comments": []map[string]any{
					{"author_id": 1, "plain_body": "Fixed by rotating the SAML cert", "created_at": "2026-09-18T00:00:00Z"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	result, err := handleSearchRelatedTickets(context.Background(), callTool(t, map[string]any{
		"query":             "SSO redirect",
		"exclude_ticket_id": "42",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %+v", result.Content)
	}

	var out RelatedTicketsResult
	unmarshalToolResult(t, result, &out)

	if len(out.Results) != 2 {
		t.Fatalf("expected 2 results (current ticket excluded), got %d: %+v", len(out.Results), out.Results)
	}
	if out.Results[0].TicketID != "10" || out.Results[0].ResolutionSummary == nil {
		t.Errorf("expected first result (solved) to carry a resolution summary, got: %+v", out.Results[0])
	}
	if out.Results[1].TicketID != "11" || out.Results[1].ResolutionSummary != nil {
		t.Errorf("expected second result (open) to have no resolution summary, got: %+v", out.Results[1])
	}
}

func TestHandleSearchRelatedTickets_MissingQuery(t *testing.T) {
	result, err := handleSearchRelatedTickets(context.Background(), callTool(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected validation error when query is missing")
	}
}
