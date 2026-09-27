package tools

import (
	"context"
	"net/http"
	"testing"
)

func TestHandleSearchLogs_Success(t *testing.T) {
	cleanup := withMockDatadog(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-API-KEY") == "" || r.Header.Get("DD-APPLICATION-KEY") == "" {
			t.Errorf("expected Datadog auth headers to be set")
		}
		respondJSON(w, map[string]any{
			"data": []map[string]any{
				{
					"attributes": map[string]any{
						"timestamp": "2026-09-20T10:01:00Z",
						"service":   "auth-service",
						"status":    "error",
						"message":   "SAML assertion validation failed",
						"attributes": map[string]any{
							"trace_id": "trace-789",
						},
					},
				},
			},
		})
	})
	defer cleanup()

	result, err := handleSearchLogs(context.Background(), callTool(t, map[string]any{
		"query":      "SAML",
		"start_time": "2026-09-20T09:00:00Z",
		"end_time":   "2026-09-20T11:00:00Z",
		"service":    "auth-service",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %+v", result.Content)
	}

	var out LogSearchResult
	unmarshalToolResult(t, result, &out)

	if out.TotalMatches != 1 {
		t.Fatalf("expected 1 match, got %d", out.TotalMatches)
	}
	if out.Logs[0].TraceID != "trace-789" {
		t.Errorf("expected trace_id extracted from nested attributes, got %q", out.Logs[0].TraceID)
	}
}

func TestHandleSearchLogs_MissingCredentials(t *testing.T) {
	t.Setenv("DD_API_KEY", "")
	t.Setenv("DD_APP_KEY", "")

	result, err := handleSearchLogs(context.Background(), callTool(t, map[string]any{
		"query":      "SAML",
		"start_time": "2026-09-20T09:00:00Z",
		"end_time":   "2026-09-20T11:00:00Z",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error when Datadog credentials are missing")
	}
}
