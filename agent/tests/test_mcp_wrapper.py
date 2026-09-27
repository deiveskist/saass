"""Testa o mcp_wrapper (GetTools/RunTool) contra o binário real do
supportability-mcp — sem LLM e sem credenciais reais, usando o mesmo
cmd/mockserver (Go) que o resto do projeto já usa pra validação local.

Pré-requisitos pra rodar:
    cd .. && go build -o /tmp/supportability-mcp . && go build -o /tmp/mockserver ./cmd/mockserver
    /tmp/mockserver -addr :9090 &
    MCP_SERVER_BINARY=/tmp/supportability-mcp pytest
"""

from __future__ import annotations

import os

import pytest

from supportability_agent.mcp_wrapper import GetTools, RunTool, apply

pytestmark = pytest.mark.skipif(
    not os.environ.get("MCP_SERVER_BINARY"),
    reason="MCP_SERVER_BINARY not set — run the mock server + Go binary first (see module docstring)",
)


def _server_command() -> str:
    return os.environ["MCP_SERVER_BINARY"]


def _mock_env() -> dict[str, str]:
    """As mesmas env vars de scripts/dev-local.sh — apontam o servidor Go
    pro mock em vez das APIs reais."""
    return {
        "ZENDESK_API_BASE": "http://localhost:9090/zendesk/%s",
        "GITHUB_API_BASE": "http://localhost:9090/github",
        "DATADOG_API_BASE": "http://localhost:9090/datadog",
        "ZENDESK_SUBDOMAIN": "acme",
        "ZENDESK_EMAIL": "a@b.com",
        "ZENDESK_API_TOKEN": "fake",
        "GITHUB_TOKEN": "fake",
        "DD_API_KEY": "fake",
        "DD_APP_KEY": "fake",
    }


async def test_get_tools_lists_all_five():
    specs = await apply(_server_command(), [], GetTools(), extra_env=_mock_env())
    names = {spec["name"] for spec in specs}
    assert names == {
        "get_ticket",
        "search_related_tickets",
        "get_recent_deploys",
        "search_logs",
        "summarize_investigation",
    }


async def test_get_tools_schema_has_required_fields():
    specs = await apply(_server_command(), [], GetTools(), extra_env=_mock_env())
    get_ticket = next(s for s in specs if s["name"] == "get_ticket")
    assert "ticket_id" in get_ticket["parameters"]["required"]


async def test_run_tool_get_ticket_against_mock():
    result_text = await apply(
        _server_command(),
        [],
        RunTool("get_ticket", {"ticket_id": "42"}),
        extra_env=_mock_env(),
    )
    assert "SSO redirect" in result_text  # vem do cmd/mockserver
    assert '"ticket_id":"42"' in result_text.replace(" ", "")


async def test_run_tool_missing_required_arg_raises():
    with pytest.raises(RuntimeError):
        await apply(
            _server_command(),
            [],
            RunTool("get_ticket", {}),
            extra_env=_mock_env(),
        )


async def test_run_tool_summarize_investigation_round_trips_evidence():
    result_text = await apply(
        _server_command(),
        [],
        RunTool(
            "summarize_investigation",
            {
                "ticket_id": "42",
                "hypothesis": "SAML cert rotation broke SSO",
                "confidence": "high",
                "evidence": ["log error trace-mock-789"],
            },
        ),
        extra_env=_mock_env(),
    )
    assert "SAML cert rotation" in result_text
    assert "trace-mock-789" in result_text
