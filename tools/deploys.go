package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Deploy struct {
	CommitSHA    string   `json:"commit_sha"`
	PRNumber     int      `json:"pr_number"`
	Title        string   `json:"title"`
	Author       string   `json:"author"`
	MergedAt     string   `json:"merged_at"`
	FilesChanged []string `json:"files_changed"`
}

type DeploysResult struct {
	Deploys []Deploy `json:"deploys"`
}

func RegisterGetRecentDeploys(s *server.MCPServer) {
	tool := mcp.NewTool("get_recent_deploys",
		mcp.WithDescription("List recent deploys or merged PRs for a repository within a time window"),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repo identifier, e.g. org/repo")),
		mcp.WithString("since", mcp.Required(), mcp.Description("RFC3339 timestamp — look back from this point")),
		mcp.WithString("until", mcp.Description("RFC3339 timestamp, defaults to now")),
	)
	s.AddTool(tool, handleGetRecentDeploys)
}

type githubCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

func handleGetRecentDeploys(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repository, err := req.RequireString("repository")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	since, err := req.RequireString("since")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	until := req.GetString("until", "")

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return mcp.NewToolResultError("GITHUB_TOKEN not set"), nil
	}

	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/commits?since=%s", repository, since)
	if until != "" {
		endpoint += "&until=" + until
	}
	// v1 fica só na branch default do repo (sem &sha=), que já é o caso
	// mais comum pra correlacionar "o que foi pro ar antes do ticket abrir".

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("github request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultError(fmt.Sprintf("github returned %d: %s", resp.StatusCode, string(body))), nil
	}

	var commits []githubCommit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	deploys := make([]Deploy, 0, len(commits))
	for _, c := range commits {
		deploys = append(deploys, Deploy{
			CommitSHA: c.SHA,
			// PRNumber e FilesChanged ficam vazios no v1: pegar isso exigiria
			// uma chamada extra por commit (/commits/{sha} ou /search/issues)
			// — custo alto de rate-limit pra um dado "bom de ter", não essencial.
			// TODO: preencher quando o volume de investigações justificar o custo.
			PRNumber:     0,
			Title:        c.Commit.Message,
			Author:       c.Commit.Author.Name,
			MergedAt:     c.Commit.Author.Date,
			FilesChanged: []string{},
		})
	}

	payload, err := json.Marshal(DeploysResult{Deploys: deploys})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(payload)), nil
}
