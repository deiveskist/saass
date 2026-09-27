package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"

	"github.com/deives/supportability-mcp/tools"
)

func main() {
	s := server.NewMCPServer(
		"supportability-agent",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	// Cada Register* injeta suas próprias dependências (clientes de API),
	// lidas de variáveis de ambiente. Isso mantém main.go só como fiação.
	tools.RegisterGetTicket(s)
	tools.RegisterSearchLogs(s)
	tools.RegisterSearchRelatedTickets(s)
	tools.RegisterGetRecentDeploys(s)
	tools.RegisterSummarizeInvestigation(s)

	// stdio é o transporte certo pro v1: o agente Python roda o binário
	// como subprocesso e fala MCP por stdin/stdout. Sem necessidade de
	// porta HTTP exposta ainda.
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("mcp server error: %v", err)
	}
}
