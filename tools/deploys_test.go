package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestHandleGetRecentDeploys_EnrichesWithPRAndFiles(t *testing.T) {
	cleanup := withMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/commits") && r.Method == http.MethodGet:
			respondJSON(w, []map[string]any{
				{
					"sha": "abc123",
					"commit": map[string]any{
						"message": "fix: rotate SAML cert",
						"author":  map[string]any{"name": "Deives", "date": "2026-09-19T12:00:00Z"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/commits/abc123"):
			respondJSON(w, map[string]any{
				"files": []map[string]any{{"filename": "auth/saml.go"}},
			})
		case strings.HasSuffix(r.URL.Path, "/commits/abc123/pulls"):
			respondJSON(w, []map[string]any{{"number": 512}})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	result, err := handleGetRecentDeploys(context.Background(), callTool(t, map[string]any{
		"repository": "acme/backend",
		"since":      "2026-09-18T00:00:00Z",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %+v", result.Content)
	}

	var out DeploysResult
	unmarshalToolResult(t, result, &out)

	if len(out.Deploys) != 1 {
		t.Fatalf("expected 1 deploy, got %d", len(out.Deploys))
	}
	d := out.Deploys[0]
	if d.PRNumber != 512 {
		t.Errorf("expected pr_number 512, got %d", d.PRNumber)
	}
	if len(d.FilesChanged) != 1 || d.FilesChanged[0] != "auth/saml.go" {
		t.Errorf("expected files_changed to include auth/saml.go, got %+v", d.FilesChanged)
	}
}

func TestHandleGetRecentDeploys_MissingGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	result, err := handleGetRecentDeploys(context.Background(), callTool(t, map[string]any{
		"repository": "acme/backend",
		"since":      "2026-09-18T00:00:00Z",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error when GITHUB_TOKEN is missing")
	}
}
