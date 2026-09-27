"""Testa a extração da conclusão do agente a partir das mensagens do
LangGraph, sem chamar LLM (mensagens montadas à mão)."""

from __future__ import annotations

import sys
from pathlib import Path

from langchain_core.messages import AIMessage, HumanMessage, ToolMessage

sys.path.insert(0, str(Path(__file__).parent.parent))
from run_validation import extract_summary  # noqa: E402


def test_picks_last_summarize_call():
    messages = [
        HumanMessage(content="investigate 5001"),
        AIMessage(content="", tool_calls=[{"name": "get_ticket", "args": {"ticket_id": "5001"}, "id": "1"}]),
        ToolMessage(content="{}", tool_call_id="1"),
        AIMessage(content="", tool_calls=[{"name": "summarize_investigation", "args": {"hypothesis": "draft"}, "id": "2"}]),
        ToolMessage(content="{}", tool_call_id="2"),
        AIMessage(content="", tool_calls=[{"name": "summarize_investigation", "args": {"hypothesis": "final"}, "id": "3"}]),
    ]
    assert extract_summary(messages) == {"hypothesis": "final"}


def test_returns_none_when_agent_never_concludes():
    messages = [
        HumanMessage(content="investigate 5001"),
        AIMessage(content="I think it is the certificate."),
    ]
    assert extract_summary(messages) is None
