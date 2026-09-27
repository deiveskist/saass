"""Monta o agente (create_react_agent do LangGraph) com as 5 tools do
ticketlens já convertidas, e roda uma investigação completa de
ticket — do "recebi o ticket X" até o `summarize_investigation` final.
"""

from __future__ import annotations

import os

from langchain_anthropic import ChatAnthropic
from langgraph.prebuilt import create_react_agent

from .mcp_wrapper import GetTools, apply
from .tools_bridge import build_langchain_tools

INVESTIGATION_PROMPT = """\
You are a support engineering copilot investigating ticket #{ticket_id}.

Steps to follow:
1. Call get_ticket to read the ticket's title, description and comments.
2. Call search_related_tickets using a query derived from the ticket's
   title or error message, to see if this has happened before.
3. If the ticket mentions a specific repository or service and a rough
   timeframe, call get_recent_deploys and/or search_logs around when the
   ticket was created to look for a correlated cause.
4. Once you have enough evidence, call summarize_investigation with your
   hypothesis, a confidence level (low/medium/high), the evidence you
   gathered (as short bullet points), and a suggested next step.

Only call summarize_investigation once, as your final action.
"""


async def _get_server_command() -> tuple[str, list[str]]:
    server_command = os.environ.get("MCP_SERVER_BINARY")
    if not server_command:
        raise RuntimeError(
            "MCP_SERVER_BINARY not set — point it at the compiled ticketlens binary"
        )
    return server_command, []


async def build_agent():
    """Descobre as tools do servidor Go (via GetTools) e monta um agente
    ReAct do LangGraph já com elas — nenhuma tool é hardcoded aqui, se uma
    6ª tool for adicionada no Go, ela aparece automaticamente."""
    server_command, server_args = await _get_server_command()

    tool_specs = await apply(server_command, server_args, GetTools())
    tools = build_langchain_tools(server_command, server_args, tool_specs)

    model = ChatAnthropic(model="claude-sonnet-4-6", temperature=0)
    return create_react_agent(model, tools)


async def investigate(ticket_id: str) -> str:
    """Roda uma investigação completa e devolve a última mensagem do
    agente (o resumo estruturado, se ele seguiu o prompt corretamente)."""
    agent = await build_agent()
    prompt = INVESTIGATION_PROMPT.format(ticket_id=ticket_id)

    result = await agent.ainvoke({"messages": [{"role": "user", "content": prompt}]})
    return result["messages"][-1].content
