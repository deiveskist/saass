"""Converte as tool specs devolvidas por `GetTools` (JSON Schema simples,
como o `mcp.NewTool(...)` do lado Go declara) em `StructuredTool` do
LangChain, prontas pro `create_react_agent` do LangGraph.

Escrito do zero pra este projeto (não faz parte do esxr/langgraph-mcp) —
como as 5 tools do supportability-mcp só usam tipos simples (string,
number, array de string), um conversor minimalista aqui evita puxar uma
dependência inteira (`langchain-mcp-adapters`) só por causa dessa etapa.
"""

from __future__ import annotations

from typing import Any

from langchain_core.tools import StructuredTool
from pydantic import BaseModel, Field, create_model

from .mcp_wrapper import RunTool, apply

_JSON_SCHEMA_TYPE_MAP: dict[str, Any] = {
    "string": str,
    "number": float,
    "integer": int,
    "boolean": bool,
    "array": list,
    "object": dict,
}


def _schema_to_pydantic(tool_name: str, schema: dict[str, Any]) -> type[BaseModel]:
    """Monta um modelo Pydantic a partir do `inputSchema` JSON de uma tool.
    Campos fora de `required` viram opcionais (default None)."""
    properties: dict[str, Any] = schema.get("properties", {})
    required = set(schema.get("required", []))

    fields: dict[str, Any] = {}
    for prop_name, prop_schema in properties.items():
        py_type = _JSON_SCHEMA_TYPE_MAP.get(prop_schema.get("type", "string"), str)
        description = prop_schema.get("description", "")
        if prop_name in required:
            fields[prop_name] = (py_type, Field(..., description=description))
        else:
            fields[prop_name] = (py_type | None, Field(None, description=description))

    return create_model(f"{tool_name}_Args", **fields)  # type: ignore[call-overload]


def build_langchain_tools(
    server_command: str,
    server_args: list[str],
    tool_specs: list[dict[str, Any]],
) -> list[StructuredTool]:
    """Transforma cada tool spec numa StructuredTool cujo `coroutine` abre
    uma sessão MCP nova (via `apply`/`RunTool`) e devolve o resultado."""
    tools: list[StructuredTool] = []

    for spec in tool_specs:
        args_model = _schema_to_pydantic(spec["name"], spec["parameters"])

        async def _run(_spec: dict[str, Any] = spec, **kwargs: Any) -> str:
            # kwargs com valor None (campo opcional não preenchido pelo
            # LLM) não deve virar {"campo": null} pra tool Go — ela usa
            # req.GetString/GetFloat com default, não espera null.
            clean_args = {k: v for k, v in kwargs.items() if v is not None}
            return await apply(server_command, server_args, RunTool(_spec["name"], clean_args))

        tools.append(
            StructuredTool.from_function(
                coroutine=_run,
                name=spec["name"],
                description=spec["description"],
                args_schema=args_model,
            )
        )

    return tools
