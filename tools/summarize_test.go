package tools

import (
	"context"
	"testing"
)

func TestHandleSummarizeInvestigation_RoundTripsEvidenceAndNextStep(t *testing.T) {
	result, err := handleSummarizeInvestigation(context.Background(), callTool(t, map[string]any{
		"ticket_id":           "42",
		"hypothesis":          "SAML cert rotation broke SSO",
		"confidence":          "high",
		"evidence":            []any{"log error trace-789", "PR #512 rotated the cert"},
		"suggested_next_step": "Roll back the cert or update the IdP metadata",
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %+v", result.Content)
	}

	var out InvestigationSummary
	unmarshalToolResult(t, result, &out)

	// Esta é exatamente a regressão que o teste manual contra o mock server
	// pegou: evidence e suggested_next_step eram descartados silenciosamente
	// pelo handler, mesmo a tool declarando esses parâmetros no schema.
	if len(out.Evidence) != 2 {
		t.Errorf("expected 2 evidence items to round-trip, got %d: %+v", len(out.Evidence), out.Evidence)
	}
	if out.SuggestedNextStep == "" {
		t.Errorf("expected suggested_next_step to round-trip, got empty string")
	}
}

func TestHandleSummarizeInvestigation_MissingRequiredField(t *testing.T) {
	result, err := handleSummarizeInvestigation(context.Background(), callTool(t, map[string]any{
		"ticket_id": "42",
		// hypothesis e confidence faltando de propósito
	}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected validation error when hypothesis/confidence are missing")
	}
}
