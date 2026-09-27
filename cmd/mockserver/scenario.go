package main

// Modo cenário: em vez dos dados fixos de main.go, serve os dados de UM
// cenário de validação (validation/scenarios/*.json). Usado por
// validation/run_validation.py, que sobe um mockserver por cenário.
//
// Filtros realistas o suficiente pra não entregar a resposta de graça:
// deploys respeitam since/until e logs respeitam a janela de tempo e o
// filtro service:X da query, como as APIs reais fariam.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type scenarioComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type scenarioTicket struct {
	ID          int64             `json:"id"`
	Subject     string            `json:"subject"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"`
	CreatedAt   string            `json:"created_at"`
	Tags        []string          `json:"tags"`
	Comments    []scenarioComment `json:"comments"`
}

type scenarioRelated struct {
	ID         int64  `json:"id"`
	Subject    string `json:"subject"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
}

type scenarioDeploy struct {
	SHA     string   `json:"sha"`
	Message string   `json:"message"`
	Author  string   `json:"author"`
	Date    string   `json:"date"`
	Files   []string `json:"files"`
	PR      int      `json:"pr"`
}

type scenarioLog struct {
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id"`
}

type scenario struct {
	ID             string            `json:"id"`
	Ticket         scenarioTicket    `json:"ticket"`
	RelatedTickets []scenarioRelated `json:"related_tickets"`
	Deploys        []scenarioDeploy  `json:"deploys"`
	Logs           []scenarioLog     `json:"logs"`
}

func loadScenario(path string) (*scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s scenario
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &s, nil
}

func parseTime(value string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, value)
	return t, err == nil
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(strings.TrimSuffix(r.PathValue("id"), ".json"), 10, 64)
	return id
}

func (s *scenario) findRelated(id int64) *scenarioRelated {
	for i := range s.RelatedTickets {
		if s.RelatedTickets[i].ID == id {
			return &s.RelatedTickets[i]
		}
	}
	return nil
}

func (s *scenario) findDeploy(sha string) *scenarioDeploy {
	for i := range s.Deploys {
		if s.Deploys[i].SHA == sha {
			return &s.Deploys[i]
		}
	}
	return nil
}

func registerScenarioMocks(mux *http.ServeMux, s *scenario) {
	// --- Zendesk ---
	mux.HandleFunc("GET /zendesk/{subdomain}/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if id == s.Ticket.ID {
			writeJSON(w, map[string]any{"ticket": map[string]any{
				"id": s.Ticket.ID, "subject": s.Ticket.Subject, "description": s.Ticket.Description,
				"status": s.Ticket.Status, "priority": s.Ticket.Priority,
				"created_at": s.Ticket.CreatedAt, "tags": s.Ticket.Tags,
			}})
			return
		}
		if rel := s.findRelated(id); rel != nil {
			writeJSON(w, map[string]any{"ticket": map[string]any{
				"id": rel.ID, "subject": rel.Subject, "description": rel.Subject,
				"status": rel.Status, "priority": "normal", "tags": []string{},
			}})
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("GET /zendesk/{subdomain}/tickets/{id}/comments.json", func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if id == s.Ticket.ID {
			comments := make([]map[string]any, 0, len(s.Ticket.Comments))
			users := make([]map[string]any, 0, len(s.Ticket.Comments))
			for i, c := range s.Ticket.Comments {
				authorID := int64(i + 1)
				comments = append(comments, map[string]any{"author_id": authorID, "plain_body": c.Body, "created_at": c.CreatedAt})
				users = append(users, map[string]any{"id": authorID, "name": c.Author})
			}
			writeJSON(w, map[string]any{"comments": comments, "users": users})
			return
		}
		if rel := s.findRelated(id); rel != nil && rel.Resolution != "" {
			writeJSON(w, map[string]any{
				"comments": []map[string]any{{"author_id": 900, "plain_body": rel.Resolution, "created_at": ""}},
				"users":    []map[string]any{{"id": 900, "name": "Support Engineer"}},
			})
			return
		}
		writeJSON(w, map[string]any{"comments": []any{}, "users": []any{}})
	})

	mux.HandleFunc("GET /zendesk/{subdomain}/search.json", func(w http.ResponseWriter, r *http.Request) {
		results := make([]map[string]any, 0, len(s.RelatedTickets))
		for _, rel := range s.RelatedTickets {
			results = append(results, map[string]any{"id": rel.ID, "subject": rel.Subject, "status": rel.Status})
		}
		writeJSON(w, map[string]any{"results": results})
	})

	mux.HandleFunc("GET /zendesk/{subdomain}/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"user": map[string]any{"name": "Support Engineer"}})
	})

	// --- GitHub ---
	mux.HandleFunc("GET /github/repos/{owner}/{repo}/commits", func(w http.ResponseWriter, r *http.Request) {
		since, hasSince := parseTime(r.URL.Query().Get("since"))
		until, hasUntil := parseTime(r.URL.Query().Get("until"))
		out := []map[string]any{}
		for _, d := range s.Deploys {
			date, ok := parseTime(d.Date)
			if !ok || (hasSince && date.Before(since)) || (hasUntil && date.After(until)) {
				continue
			}
			out = append(out, map[string]any{
				"sha":    d.SHA,
				"commit": map[string]any{"message": d.Message, "author": map[string]any{"name": d.Author, "date": d.Date}},
			})
		}
		writeJSON(w, out)
	})

	mux.HandleFunc("GET /github/repos/{owner}/{repo}/commits/{sha}/pulls", func(w http.ResponseWriter, r *http.Request) {
		d := s.findDeploy(r.PathValue("sha"))
		if d == nil || d.PR == 0 {
			writeJSON(w, []any{})
			return
		}
		writeJSON(w, []map[string]any{{"number": d.PR}})
	})

	mux.HandleFunc("GET /github/repos/{owner}/{repo}/commits/{sha}", func(w http.ResponseWriter, r *http.Request) {
		d := s.findDeploy(r.PathValue("sha"))
		if d == nil {
			http.NotFound(w, r)
			return
		}
		files := make([]map[string]any, 0, len(d.Files))
		for _, f := range d.Files {
			files = append(files, map[string]any{"filename": f})
		}
		writeJSON(w, map[string]any{"files": files})
	})

	// --- Datadog ---
	mux.HandleFunc("POST /datadog/api/v2/logs/events/search", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Filter struct {
				Query string `json:"query"`
				From  string `json:"from"`
				To    string `json:"to"`
			} `json:"filter"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		from, hasFrom := parseTime(body.Filter.From)
		to, hasTo := parseTime(body.Filter.To)

		serviceFilter := ""
		for _, token := range strings.Fields(body.Filter.Query) {
			if strings.HasPrefix(token, "service:") {
				serviceFilter = strings.TrimPrefix(token, "service:")
			}
		}

		data := []map[string]any{}
		for _, l := range s.Logs {
			ts, ok := parseTime(l.Timestamp)
			if !ok || (hasFrom && ts.Before(from)) || (hasTo && ts.After(to)) {
				continue
			}
			if serviceFilter != "" && l.Service != serviceFilter {
				continue
			}
			data = append(data, map[string]any{"attributes": map[string]any{
				"timestamp": l.Timestamp, "service": l.Service, "status": l.Level,
				"message": l.Message, "attributes": map[string]any{"trace_id": l.TraceID},
			}})
		}
		writeJSON(w, map[string]any{"data": data})
	})
}
