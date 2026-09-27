"""Wrapper genérico de sessão MCP, usando o Strategy Pattern.

Baseado na arquitetura de `mcp_wrapper.py` do projeto
esxr/langgraph-mcp (MIT): uma classe abstrata `MCPSessionFunction` com
implementações concretas plugadas numa função `apply()` que abre a sessão,
roda a operação e fecha — permite adicionar novas operações (além de
GetTools/RunTool) sem tocar na lógica de abrir/fechar sessão.

Simplificado em relação ao original: aqui só existe *um* servidor MCP
(o supportability-mcp em Go), então `apply()` não recebe um dicionário de
config de múltiplos servidores — só o comando pra rodar esse único
binário.
"""

from __future__ import annotations

import os
from abc import ABC, abstractmethod
from typing import Any

from mcp import ClientSession, StdioServerParameters, stdio_client


class MCPSessionFunction(ABC):
    """Contrato comum pra qualquer operação que precise de uma sessão MCP
    já inicializada."""

    @abstractmethod
    async def __call__(self, session: ClientSession) -> Any: ...


class GetTools(MCPSessionFunction):
    """Lista as tools do servidor MCP, no formato genérico
    {name, description, parameters} — `tools_bridge.py` converte isso pra
    StructuredTool do LangChain."""

    async def __call__(self, session: ClientSession) -> list[dict[str, Any]]:
        result = await session.list_tools()
        return [
            {
                "name": tool.name,
                "description": tool.description or "",
                "parameters": tool.input_schema or {},
            }
            for tool in result.tools
        ]


class RunTool(MCPSessionFunction):
    """Invoca uma tool específica com os argumentos dados e devolve o
    texto retornado (as tools do supportability-mcp sempre devolvem JSON
    como texto puro — ver mcp-tools-schema.md na raiz do repo)."""

    def __init__(self, tool_name: str, arguments: dict[str, Any]):
        self.tool_name = tool_name
        self.arguments = arguments

    async def __call__(self, session: ClientSession) -> str:
        result = await session.call_tool(self.tool_name, arguments=self.arguments)
        text = result.content[0].text if result.content else "{}"
        if result.is_error:
            raise RuntimeError(f"{self.tool_name} failed: {text}")
        return text


async def apply(
    server_command: str,
    server_args: list[str],
    fn: MCPSessionFunction,
    extra_env: dict[str, str] | None = None,
) -> Any:
    """Abre uma sessão stdio nova com o supportability-mcp, roda `fn` e
    fecha o processo. Um processo por chamada, de propósito — o servidor
    Go não guarda estado entre tools (ver tools/httpserver.go, que segue o
    mesmo raciocínio pro modo HTTP), então não há custo real em reabrir, e
    isso evita qualquer estado compartilhado escondido entre investigações.
    """
    params = StdioServerParameters(
        command=server_command,
        args=server_args,
        env={**os.environ, **(extra_env or {})},
    )
    try:
        async with stdio_client(params) as (read, write):
            async with ClientSession(read, write) as session:
                await session.initialize()
                return await fn(session)
    except BaseExceptionGroup as group:
        # stdio_client/ClientSession rodam dentro de task groups do anyio,
        # que embrulham qualquer exceção num ExceptionGroup. Desembrulhamos
        # pra quem chama (e o LangGraph, via tools_bridge) ver o erro real
        # da tool — ex.: "get_ticket failed: required argument ..." —
        # em vez de um grupo aninhado ilegível.
        raise _first_leaf(group) from group


def _first_leaf(exc: BaseException) -> BaseException:
    while isinstance(exc, BaseExceptionGroup) and exc.exceptions:
        exc = exc.exceptions[0]
    return exc
