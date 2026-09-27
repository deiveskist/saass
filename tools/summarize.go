package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type InvestigationSummary struct {
	TicketID          string   `json:"ticket_id"`
	Hypothesis        string   `json:"hypothesis"`
	Confidence        string   `json:"confidence"` // low | medium | high
	Evidence          []string `json:"evidence"`
	SuggestedNextStep string   `json:"suggested_next_step"`
	RelatedTicketIDs  []string `json:"related_ticket_ids"`
	RelatedDeploySHA  *string  `json:"related_deploy_sha"`
}

func RegisterSummarizeInvestigation(s *server.MCPServer) {
	tool := mcp.NewTool("summarize_investigation",
		mcp.WithDescription("Produce a structured investigation summary from gathered evidence"),
		mcp.WithString("ticket_id", mcp.Required()),
		mcp.WithString("hypothesis", mcp.Required(), mcp.Description("Most likely root cause")),
		mcp.WithString("confidence", mcp.Required(), mcp.Enum("low", "medium", "high")),
		mcp.WithArray("evidence", mcp.Required(), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithString("suggested_next_step"),
	)
	s.AddTool(tool, handleSummarizeInvestigation)
}

// Este handler não chama API nenhuma: só valida e estrutura o que o
// próprio LLM já concluiu, garantindo formato de saída consistente pro
// humano que vai ler o resultado (é por isso que ela é a última tool
// chamada numa investigação, nunca a primeira).
func handleSummarizeInvestigation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ticketID, err := req.RequireString("ticket_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	hypothesis, err := req.RequireString("hypothesis")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	confidence, err := req.RequireString("confidence")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	summary := InvestigationSummary{
		TicketID:         ticketID,
		Hypothesis:       hypothesis,
		Confidence:       confidence,
		Evidence:         []string{},
		RelatedTicketIDs: []string{},
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(payload)), nil
}
