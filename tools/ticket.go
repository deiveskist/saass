package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Comment corresponde a um item de "comments" no retorno de get_ticket
// (ver mcp-tools-schema.md).
type Comment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// Ticket é o formato de retorno de get_ticket, espelhando o schema salvo.
type Ticket struct {
	TicketID    string    `json:"ticket_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	CreatedAt   string    `json:"created_at"`
	Customer    string    `json:"customer"`
	Comments    []Comment `json:"comments"`
	Tags        []string  `json:"tags"`
}

// RegisterGetTicket declara a tool "get_ticket" e liga seu handler ao
// servidor MCP. Esse é o padrão a repetir nos outros arquivos deste pacote:
//  1. montar o mcp.Tool com nome/descrição/params (bate com o JSON schema)
//  2. escrever um handler que nunca retorna erro Go bruto — sempre um
//     resultado MCP, usando IsError para falhas, pra não derrubar o agente.
func RegisterGetTicket(s *server.MCPServer) {
	tool := mcp.NewTool("get_ticket",
		mcp.WithDescription("Fetch a support ticket by ID, including its comment history"),
		mcp.WithString("ticket_id",
			mcp.Required(),
			mcp.Description("External ticket ID (e.g. Zendesk/Jira ID)"),
		),
	)

	s.AddTool(tool, handleGetTicket)
}

func handleGetTicket(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ticketID, err := req.RequireString("ticket_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ticket, err := fetchTicketFromZendesk(ctx, ticketID)
	if err != nil {
		// Erro de integração vira { "error": "..." } pro agente conseguir
		// seguir investigando com as outras tools, em vez de travar tudo.
		return mcp.NewToolResultError(fmt.Sprintf("failed to fetch ticket %s: %v", ticketID, err)), nil
	}

	payload, err := json.Marshal(ticket)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(payload)), nil
}

// zendeskTicketResponse espelha só os campos que usamos do payload real
// da API (https://developer.zendesk.com/api-reference/ticketing/tickets/tickets/#show-ticket).
type zendeskTicketResponse struct {
	Ticket struct {
		ID          int64    `json:"id"`
		Subject     string   `json:"subject"`
		Description string   `json:"description"`
		Status      string   `json:"status"`
		Priority    string   `json:"priority"`
		CreatedAt   string   `json:"created_at"`
		Tags        []string `json:"tags"`
	} `json:"ticket"`
}

type zendeskCommentsResponse struct {
	Comments []struct {
		AuthorID  int64  `json:"author_id"`
		Body      string `json:"plain_body"`
		CreatedAt string `json:"created_at"`
	} `json:"comments"`
	// Users vem preenchido graças a ?include=users na URL — a Zendesk
	// "side-loada" os usuários citados nos comentários numa única resposta,
	// evitando 1 chamada por autor.
	Users []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"users"`
}

// authorNameCache guarda id -> nome resolvido durante o processo inteiro
// (não só um ticket). Útil porque, numa mesma investigação, o mesmo
// support engineer costuma aparecer em vários comentários e tickets
// relacionados. Protegido por mutex pois o servidor MCP pode atender
// chamadas concorrentes do agente.
var (
	authorNameCache   = map[int64]string{}
	authorNameCacheMu sync.RWMutex
)

func cacheAuthorNames(users []struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}) {
	authorNameCacheMu.Lock()
	defer authorNameCacheMu.Unlock()
	for _, u := range users {
		authorNameCache[u.ID] = u.Name
	}
}

// resolveAuthorName tenta o cache primeiro; se o autor não veio side-loaded
// nesta resposta (ex.: org com o side-loading desabilitado), cai pra uma
// chamada individual a /users/{id}.json e guarda o resultado pra próxima vez.
func resolveAuthorName(ctx context.Context, env *zendeskEnv, authorID int64) string {
	authorNameCacheMu.RLock()
	name, ok := authorNameCache[authorID]
	authorNameCacheMu.RUnlock()
	if ok {
		return name
	}

	type zendeskUserResponse struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	resp, err := zendeskGet[zendeskUserResponse](ctx, env,
		fmt.Sprintf("https://%s.zendesk.com/api/v2/users/%d.json", env.Subdomain, authorID))
	if err != nil {
		// Não trava a investigação por causa de um nome — cai pro ID cru.
		return fmt.Sprintf("user:%d", authorID)
	}

	authorNameCacheMu.Lock()
	authorNameCache[authorID] = resp.User.Name
	authorNameCacheMu.Unlock()
	return resp.User.Name
}

// zendeskEnv agrupa as credenciais lidas do ambiente. Falha cedo e com
// mensagem clara se alguma faltar, em vez de deixar a chamada HTTP
// estourar um erro genérico de auth mais na frente.
type zendeskEnv struct {
	Subdomain string
	Email     string
	APIToken  string
}

func loadZendeskEnv() (*zendeskEnv, error) {
	e := &zendeskEnv{
		Subdomain: os.Getenv("ZENDESK_SUBDOMAIN"),
		Email:     os.Getenv("ZENDESK_EMAIL"),
		APIToken:  os.Getenv("ZENDESK_API_TOKEN"),
	}
	if e.Subdomain == "" || e.Email == "" || e.APIToken == "" {
		return nil, fmt.Errorf("missing one of ZENDESK_SUBDOMAIN, ZENDESK_EMAIL, ZENDESK_API_TOKEN")
	}
	return e, nil
}

// fetchTicketFromZendesk busca o ticket e, em seguida, seus comentários,
// juntando os dois numa única struct Ticket (2 chamadas HTTP porque a API
// de show-ticket do Zendesk não inclui comentários por padrão).
func fetchTicketFromZendesk(ctx context.Context, ticketID string) (*Ticket, error) {
	env, err := loadZendeskEnv()
	if err != nil {
		return nil, err
	}

	base := fmt.Sprintf("https://%s.zendesk.com/api/v2", env.Subdomain)

	ticketResp, err := zendeskGet[zendeskTicketResponse](ctx, env, fmt.Sprintf("%s/tickets/%s.json", base, ticketID))
	if err != nil {
		return nil, fmt.Errorf("fetching ticket: %w", err)
	}

	commentsResp, err := zendeskGet[zendeskCommentsResponse](ctx, env, fmt.Sprintf("%s/tickets/%s/comments.json?include=users", base, ticketID))
	if err != nil {
		// Não falha a investigação inteira por causa dos comentários —
		// devolve o ticket sem histórico e deixa claro no campo.
		return &Ticket{
			TicketID:    ticketID,
			Title:       ticketResp.Ticket.Subject,
			Description: ticketResp.Ticket.Description,
			Status:      ticketResp.Ticket.Status,
			Priority:    ticketResp.Ticket.Priority,
			CreatedAt:   ticketResp.Ticket.CreatedAt,
			Tags:        ticketResp.Ticket.Tags,
			Comments:    []Comment{},
		}, nil
	}
	cacheAuthorNames(commentsResp.Users)

	comments := make([]Comment, 0, len(commentsResp.Comments))
	for _, c := range commentsResp.Comments {
		comments = append(comments, Comment{
			Author:    resolveAuthorName(ctx, env, c.AuthorID),
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		})
	}

	return &Ticket{
		TicketID:    ticketID,
		Title:       ticketResp.Ticket.Subject,
		Description: ticketResp.Ticket.Description,
		Status:      ticketResp.Ticket.Status,
		Priority:    ticketResp.Ticket.Priority,
		CreatedAt:   ticketResp.Ticket.CreatedAt,
		Tags:        ticketResp.Ticket.Tags,
		Comments:    comments,
	}, nil
}

// zendeskGet centraliza GET autenticado + decode JSON, usado por essa
// tool e por search_related_tickets. Genérico em T pra não repetir o
// boilerplate de request/response em cada endpoint do Zendesk.
func zendeskGet[T any](ctx context.Context, env *zendeskEnv, url string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", zendeskBasicAuth(env.Email, env.APIToken))
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zendesk returned %d: %s", resp.StatusCode, string(body))
	}

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &out, nil
}
