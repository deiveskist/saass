package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id"`
}

type LogSearchResult struct {
	TotalMatches int        `json:"total_matches"`
	Logs         []LogEntry `json:"logs"`
}

func RegisterSearchLogs(s *server.MCPServer) {
	tool := mcp.NewTool("search_logs",
		mcp.WithDescription("Search application logs by time range and a query term or correlation ID"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Free-text search term or correlation/trace ID")),
		mcp.WithString("start_time", mcp.Required(), mcp.Description("RFC3339 timestamp")),
		mcp.WithString("end_time", mcp.Required(), mcp.Description("RFC3339 timestamp")),
		mcp.WithString("service", mcp.Description("Optional service/component name to scope the search")),
		mcp.WithNumber("limit", mcp.Description("Max log lines to return"), mcp.DefaultNumber(100)),
	)
	s.AddTool(tool, handleSearchLogs)
}

// datadogSearchRequest segue o formato esperado por POST /api/v2/logs/events/search.
type datadogSearchRequest struct {
	Filter struct {
		Query string `json:"query"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"filter"`
	Page struct {
		Limit int `json:"limit"`
	} `json:"page"`
}

type datadogSearchResponse struct {
	Data []struct {
		Attributes struct {
			Timestamp  string         `json:"timestamp"`
			Service    string         `json:"service"`
			Status     string         `json:"status"`
			Message    string         `json:"message"`
			Attributes map[string]any `json:"attributes"`
		} `json:"attributes"`
	} `json:"data"`
}

func handleSearchLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	startTime, err := req.RequireString("start_time")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	endTime, err := req.RequireString("end_time")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	service := req.GetString("service", "")
	limit := int(req.GetFloat("limit", 100))

	apiKey := os.Getenv("DD_API_KEY")
	appKey := os.Getenv("DD_APP_KEY")
	if apiKey == "" || appKey == "" {
		return mcp.NewToolResultError("DD_API_KEY / DD_APP_KEY not set"), nil
	}

	fullQuery := query
	if service != "" {
		fullQuery = fmt.Sprintf("service:%s %s", service, query)
	}

	var body datadogSearchRequest
	body.Filter.Query = fullQuery
	body.Filter.From = startTime
	body.Filter.To = endTime
	body.Page.Limit = limit

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.datadoghq.com/api/v2/logs/events/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("DD-API-KEY", apiKey)
	httpReq.Header.Set("DD-APPLICATION-KEY", appKey)
	// TODO: se a org usar site diferente do padrão (ex. datadoghq.eu),
	// trocar o host acima por env var DD_SITE.

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("datadog request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultError(fmt.Sprintf("datadog returned %d: %s", resp.StatusCode, string(respBody))), nil
	}

	var ddResp datadogSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&ddResp); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	logs := make([]LogEntry, 0, len(ddResp.Data))
	for _, item := range ddResp.Data {
		traceID := ""
		if v, ok := item.Attributes.Attributes["trace_id"].(string); ok {
			traceID = v
		}
		logs = append(logs, LogEntry{
			Timestamp: item.Attributes.Timestamp,
			Service:   item.Attributes.Service,
			Level:     item.Attributes.Status,
			Message:   item.Attributes.Message,
			TraceID:   traceID,
		})
	}

	result := LogSearchResult{TotalMatches: len(logs), Logs: logs}

	payload, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(payload)), nil
}
