package tools

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Métricas expostas em /metrics (modo --http). Nomeadas com o prefixo
// ticketlens_ pra não colidir com métricas de outro serviço no
// mesmo scrape target.
var (
	toolCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ticketlens_tool_calls_total",
			Help: "Total de chamadas por tool, com o resultado (ok | tool_error | internal_error).",
		},
		[]string{"tool", "status"},
	)

	toolCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "ticketlens_tool_call_duration_seconds",
			Help: "Duração de cada chamada de tool, do início ao fim do handler.",
			// Buckets pensados pra chamadas de rede (Zendesk/GitHub/Datadog):
			// da casa de 10ms (cache/erro rápido) até timeouts (15s, ver
			// httpClient em httpclient.go).
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 15},
		},
		[]string{"tool"},
	)
)

// recordToolCall registra a duração e o resultado de uma chamada — chamada
// via defer no wrapHandler, então cobre inclusive o caminho de erro Go
// (panic recuperado não é o caso aqui, mas erro retornado sim).
func recordToolCall(tool, status string, durationSeconds float64) {
	toolCallsTotal.WithLabelValues(tool, status).Inc()
	toolCallDuration.WithLabelValues(tool).Observe(durationSeconds)
}
