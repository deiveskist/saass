package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// respondJSON escreve payload como JSON na resposta — usado por todos os
// handlers mockados nos testes deste pacote.
func respondJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// withMockGitHub sobe um httptest.Server, aponta githubAPIBase pra ele e
// seta GITHUB_TOKEN. Segue o mesmo padrão de withMockZendesk (ticket_test.go).
func withMockGitHub(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	server := httptest.NewServer(handler)
	original := githubAPIBase
	githubAPIBase = server.URL

	t.Setenv("GITHUB_TOKEN", "fake-token")

	return func() {
		githubAPIBase = original
		server.Close()
	}
}

// withMockDatadog segue o mesmo padrão para DD_API_KEY/DD_APP_KEY.
func withMockDatadog(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	server := httptest.NewServer(handler)
	original := datadogAPIBase
	datadogAPIBase = server.URL

	t.Setenv("DD_API_KEY", "fake-api-key")
	t.Setenv("DD_APP_KEY", "fake-app-key")

	return func() {
		datadogAPIBase = original
		server.Close()
	}
}
