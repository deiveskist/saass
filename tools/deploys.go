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

	endpoint := fmt.Sprintf(githubAPIBase+"/repos/%s/commits?since=%s", repository, since)
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

	// Limita quantos commits recebem a chamada detalhada (files_changed +
	// PR associado). Numa investigação real a janela de tempo já é curta
	// (ex.: últimas 24-48h), então isso raramente corta resultado relevante
	// — só protege contra um "since" largo demais estourar o rate limit.
	const maxDetailed = 15

	deploys := make([]Deploy, 0, len(commits))
	for i, c := range commits {
		deploy := Deploy{
			CommitSHA:    c.SHA,
			Title:        c.Commit.Message,
			Author:       c.Commit.Author.Name,
			MergedAt:     c.Commit.Author.Date,
			FilesChanged: []string{},
		}

		if i < maxDetailed {
			if files, err := fetchCommitFiles(ctx, token, repository, c.SHA); err == nil {
				deploy.FilesChanged = files
			}
			if prNumber, err := fetchAssociatedPR(ctx, token, repository, c.SHA); err == nil {
				deploy.PRNumber = prNumber
			}
		}

		deploys = append(deploys, deploy)
	}

	payload, err := json.Marshal(DeploysResult{Deploys: deploys})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(payload)), nil
}

// fetchCommitFiles busca os arquivos alterados num commit específico.
func fetchCommitFiles(ctx context.Context, token, repository, sha string) ([]string, error) {
	type commitDetail struct {
		Files []struct {
			Filename string `json:"filename"`
		} `json:"files"`
	}
	var detail commitDetail
	if err := githubGet(ctx, token, fmt.Sprintf(githubAPIBase+"/repos/%s/commits/%s", repository, sha), &detail); err != nil {
		return nil, err
	}
	files := make([]string, 0, len(detail.Files))
	for _, f := range detail.Files {
		files = append(files, f.Filename)
	}
	return files, nil
}

// fetchAssociatedPR usa o endpoint de "pulls associated with a commit" pra
// achar o número da PR que trouxe esse commit (0 se for commit direto na
// branch, sem PR — comum em hotfix ou squash manual).
func fetchAssociatedPR(ctx context.Context, token, repository, sha string) (int, error) {
	type pr struct {
		Number int `json:"number"`
	}
	var prs []pr
	if err := githubGet(ctx, token, fmt.Sprintf(githubAPIBase+"/repos/%s/commits/%s/pulls", repository, sha), &prs); err != nil {
		return 0, err
	}
	if len(prs) == 0 {
		return 0, nil
	}
	return prs[0].Number, nil
}

// githubGet centraliza GET autenticado + decode JSON contra a GitHub API,
// no mesmo espírito do zendeskGet em ticket.go.
func githubGet(ctx context.Context, token, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github returned %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
