package main

import (
	"flag"
	"log"

	"github.com/mark3labs/mcp-go/server"

	"github.com/deives/supportability-mcp/tools"
)

func main() {
	httpAddr := flag.String("http", "", "if set (e.g. :8080), serve tools as REST over HTTP instead of MCP/stdio")
	flag.Parse()

	if *httpAddr != "" {
		// Modo REST: usado pelo dashboard Next.js (web/), que não fala o
		// protocolo MCP. O agente Python continua usando o modo stdio
		// abaixo — os dois transportes chamam exatamente as mesmas tools.
		if err := tools.ServeHTTP(*httpAddr); err != nil {
			log.Fatalf("http server error: %v", err)
		}
		return
	}

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
