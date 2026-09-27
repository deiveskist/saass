package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type RelatedTicket struct {
	TicketID          string  `json:"ticket_id"`
	Title             string  `json:"title"`
	SimilarityScore   float64 `json:"similarity_score"`
	ResolutionSummary *string `json:"resolution_summary"`
	ResolvedAt        *string `json:"resolved_at"`
}

type RelatedTicketsResult struct {
	Results []RelatedTicket `json:"results"`
}

func RegisterSearchRelatedTickets(s *server.MCPServer) {
	tool := mcp.NewTool("search_related_tickets",
		mcp.WithDescription("Find past tickets with similar symptoms, prioritizing ones with a recorded resolution"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Error message, symptom description, or keywords")),
		mcp.WithString("exclude_ticket_id", mcp.Description("Ticket ID to exclude from results (the current one)")),
		mcp.WithNumber("limit", mcp.DefaultNumber(5)),
	)
	s.AddTool(tool, handleSearchRelatedTickets)
}

type zendeskSearchResponse struct {
	Results []struct {
		ID      int64  `json:"id"`
		Subject string `json:"subject"`
		Status  string `json:"status"`
	} `json:"results"`
}

func handleSearchRelatedTickets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	excludeID := req.GetString("exclude_ticket_id", "")
	limit := int(req.GetFloat("limit", 5))

	env, err := loadZendeskEnv()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// A Zendesk Search API só faz full-text simples — sem ranking
	// semântico. Isso é suficiente pro v1; embeddings ficam pra depois
	// que houver volume real de tickets fechados pra indexar.
	searchQuery := fmt.Sprintf("type:ticket %s", query)
	endpoint := fmt.Sprintf(
		"https://%s.zendesk.com/api/v2/search.json?query=%s&sort_by=updated_at&sort_order=desc",
		env.Subdomain, url.QueryEscape(searchQuery),
	)

	searchResp, err := zendeskGet[zendeskSearchResponse](ctx, env, endpoint)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("zendesk search failed: %v", err)), nil
	}

	results := make([]RelatedTicket, 0, limit)
	for _, r := range searchResp.Results {
		ticketID := fmt.Sprintf("%d", r.ID)
		if ticketID == excludeID {
			continue
		}
		if len(results) >= limit {
			break
		}

		var resolutionSummary *string
		// Só busca o comentário de fechamento pra tickets já resolvidos —
		// evita gastar chamada em tickets abertos, que não têm resolução ainda.
		if r.Status == "solved" || r.Status == "closed" {
			if body, err := fetchLastComment(ctx, env, ticketID); err == nil {
				resolutionSummary = &body
			}
		}

		results = append(results, RelatedTicket{
			TicketID:          ticketID,
			Title:             r.Subject,
			SimilarityScore:   0, // Zendesk não expõe score de relevância; ordem já vem por updated_at
			ResolutionSummary: resolutionSummary,
			ResolvedAt:        nil,
		})
	}

	payload, err := json.Marshal(RelatedTicketsResult{Results: results})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(payload)), nil
}
