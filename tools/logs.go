package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

// datadogSearchRequest e datadogSearchResponse ficam na seção "--- Datadog ---"
// mais abaixo, junto com searchLogsDatadog.

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

	params := logSearchParams{query, startTime, endTime, service, limit}

	// LOGS_PROVIDER decide qual backend de logs o cliente usa — cada
	// instalação self-hosted configura isso uma vez conforme a stack de
	// observabilidade que já tem, sem precisar trocar código. "datadog" é
	// o default, pra não quebrar quem já está rodando com ele configurado.
	var (
		logs     []LogEntry
		provider = envOr("LOGS_PROVIDER", "datadog")
	)
	switch provider {
	case "elasticsearch":
		logs, err = searchLogsElasticsearch(ctx, params)
	case "datadog":
		logs, err = searchLogsDatadog(ctx, params)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown LOGS_PROVIDER %q (use \"datadog\" or \"elasticsearch\")", provider)), nil
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	payload, err := json.Marshal(LogSearchResult{TotalMatches: len(logs), Logs: logs})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(payload)), nil
}

// logSearchParams agrupa os parâmetros já validados da tool, comuns a
// qualquer backend de logs.
type logSearchParams struct {
	query     string
	startTime string
	endTime   string
	service   string
	limit     int
}

// --- Datadog ---

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

func searchLogsDatadog(ctx context.Context, p logSearchParams) ([]LogEntry, error) {
	apiKey := os.Getenv("DD_API_KEY")
	appKey := os.Getenv("DD_APP_KEY")
	if apiKey == "" || appKey == "" {
		return nil, fmt.Errorf("DD_API_KEY / DD_APP_KEY not set")
	}

	fullQuery := p.query
	if p.service != "" {
		fullQuery = fmt.Sprintf("service:%s %s", p.service, p.query)
	}

	var body datadogSearchRequest
	body.Filter.Query = fullQuery
	body.Filter.From = p.startTime
	body.Filter.To = p.endTime
	body.Page.Limit = p.limit

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		datadogAPIBase+"/api/v2/logs/events/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("DD-API-KEY", apiKey)
	httpReq.Header.Set("DD-APPLICATION-KEY", appKey)
	// TODO: se a org usar site diferente do padrão (ex. datadoghq.eu),
	// trocar o host acima por env var DD_SITE.

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("datadog request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("datadog returned %d: %s", resp.StatusCode, string(respBody))
	}

	var ddResp datadogSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&ddResp); err != nil {
		return nil, err
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
	return logs, nil
}

// --- Elasticsearch ---

// elasticsearchSearchRequest é a Query DSL padrão do Elasticsearch — busca
// por texto livre (query_string, aceita a mesma sintaxe tipo "status:error"
// que o Datadog usa) dentro de uma janela de tempo em @timestamp, o campo
// padrão do ECS (Elastic Common Schema).
type elasticsearchSearchRequest struct {
	Size  int `json:"size"`
	Query struct {
		Bool struct {
			Must   []map[string]any `json:"must"`
			Filter []map[string]any `json:"filter"`
		} `json:"bool"`
	} `json:"query"`
	Sort []map[string]string `json:"sort"`
}

type elasticsearchSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source struct {
				Timestamp string `json:"@timestamp"`
				Service   struct {
					Name string `json:"name"`
				} `json:"service"`
				Log struct {
					Level string `json:"level"`
				} `json:"log"`
				Message string `json:"message"`
				Trace   struct {
					ID string `json:"id"`
				} `json:"trace"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func searchLogsElasticsearch(ctx context.Context, p logSearchParams) ([]LogEntry, error) {
	baseURL := os.Getenv("ELASTICSEARCH_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("ELASTICSEARCH_URL not set")
	}
	index := envOr("ELASTICSEARCH_INDEX", "logs-*")

	var body elasticsearchSearchRequest
	body.Size = p.limit
	body.Query.Bool.Must = []map[string]any{
		{"query_string": map[string]any{"query": p.query}},
	}
	body.Query.Bool.Filter = []map[string]any{
		{"range": map[string]any{"@timestamp": map[string]any{"gte": p.startTime, "lte": p.endTime}}},
	}
	if p.service != "" {
		body.Query.Bool.Filter = append(body.Query.Bool.Filter, map[string]any{
			"match": map[string]any{"service.name": p.service},
		})
	}
	body.Sort = []map[string]string{{"@timestamp": "asc"}}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimSuffix(baseURL, "/") + "/" + index + "/_search"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := setElasticsearchAuth(httpReq); err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elasticsearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var esResp elasticsearchSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, err
	}

	logs := make([]LogEntry, 0, len(esResp.Hits.Hits))
	for _, h := range esResp.Hits.Hits {
		logs = append(logs, LogEntry{
			Timestamp: h.Source.Timestamp,
			Service:   h.Source.Service.Name,
			Level:     h.Source.Log.Level,
			Message:   h.Source.Message,
			TraceID:   h.Source.Trace.ID,
		})
	}
	return logs, nil
}

// setElasticsearchAuth aceita API Key (comum no Elastic Cloud) ou usuário/
// senha (comum em cluster self-hosted) — o que estiver configurado ganha;
// nenhum dos dois setado é um erro, já que a maioria dos clusters exige auth.
func setElasticsearchAuth(r *http.Request) error {
	if apiKey := os.Getenv("ELASTICSEARCH_API_KEY"); apiKey != "" {
		r.Header.Set("Authorization", "ApiKey "+apiKey)
		return nil
	}
	user := os.Getenv("ELASTICSEARCH_USERNAME")
	pass := os.Getenv("ELASTICSEARCH_PASSWORD")
	if user != "" && pass != "" {
		r.SetBasicAuth(user, pass)
		return nil
	}
	return fmt.Errorf("set ELASTICSEARCH_API_KEY or ELASTICSEARCH_USERNAME/ELASTICSEARCH_PASSWORD")
}
