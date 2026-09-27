"""Roda os cenários de validation/scenarios contra o sistema real.

Para cada cenário, sobe `mockserver -scenario <arquivo>` (dados do
cenário), aponta o binário do ticketlens pra ele e:

  --check   Sem LLM e sem custo. Chama as 4 tools de dados exatamente como
            o agente chamaria e confirma que toda `required_evidence` do
            ground truth é alcançável pelas tools. Garante que cada
            cenário é resolvível antes de gastar tokens.

  (padrão)  Roda o agente LangGraph de verdade (precisa ANTHROPIC_API_KEY),
            captura a chamada `summarize_investigation`, pontua com
            scoring.py e imprime o relatório contra o gate de 60%.

Uso (a partir da raiz do repo):
    go build -o /tmp/ticketlens . && go build -o /tmp/ticketlens-mockserver ./cmd/mockserver
    pip install -e agent
    MCP_SERVER_BINARY=/tmp/ticketlens MOCKSERVER_BINARY=/tmp/ticketlens-mockserver \
        python validation/run_validation.py --check
"""

from __future__ import annotations

import argparse
import asyncio
import json
import os
import subprocess
import sys
import time
import urllib.request
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

from ticketlens_agent.mcp_wrapper import RunTool, apply

sys.path.insert(0, str(Path(__file__).parent))
from scoring import score  # noqa: E402

SCENARIOS_DIR = Path(__file__).parent / "scenarios"
RESULTS_DIR = Path(__file__).parent / "results"
MOCK_PORT = 9090
PASS_GATE = 0.6

# As mesmas variáveis de scripts/dev-local.sh: o servidor Go fala com o
# mock em vez das APIs reais. apply() repassa os.environ pro subprocesso.
MOCK_ENV = {
    "ZENDESK_API_BASE": f"http://localhost:{MOCK_PORT}/zendesk/%s",
    "GITHUB_API_BASE": f"http://localhost:{MOCK_PORT}/github",
    "DATADOG_API_BASE": f"http://localhost:{MOCK_PORT}/datadog",
    "ZENDESK_SUBDOMAIN": "acme",
    "ZENDESK_EMAIL": "validation@acme.test",
    "ZENDESK_API_TOKEN": "fake",
    "GITHUB_TOKEN": "fake",
    "LOGS_PROVIDER": "datadog",
    "DD_API_KEY": "fake",
    "DD_APP_KEY": "fake",
}


def load_scenarios(only: str | None) -> list[tuple[Path, dict[str, Any]]]:
    paths = sorted(SCENARIOS_DIR.glob("*.json"))
    if only:
        paths = [p for p in paths if only in p.stem]
    return [(p, json.loads(p.read_text())) for p in paths]


class MockServer:
    """Sobe o mockserver em modo cenário e derruba ao sair do `with`."""

    def __init__(self, scenario_path: Path):
        self.scenario_path = scenario_path
        self.proc: subprocess.Popen | None = None

    def __enter__(self) -> "MockServer":
        binary = os.environ.get("MOCKSERVER_BINARY")
        if not binary:
            raise RuntimeError("MOCKSERVER_BINARY not set (go build -o /tmp/ticketlens-mockserver ./cmd/mockserver)")
        self.proc = subprocess.Popen(
            [binary, "-addr", f":{MOCK_PORT}", "-scenario", str(self.scenario_path)],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        deadline = time.time() + 5
        while time.time() < deadline:
            try:
                urllib.request.urlopen(f"http://localhost:{MOCK_PORT}/healthz", timeout=0.5)
                return self
            except OSError:
                time.sleep(0.1)
        self.__exit__(None, None, None)
        raise RuntimeError("mockserver did not become healthy")

    def __exit__(self, *exc: object) -> None:
        if self.proc:
            self.proc.terminate()
            self.proc.wait(timeout=5)


def _server() -> str:
    binary = os.environ.get("MCP_SERVER_BINARY")
    if not binary:
        raise RuntimeError("MCP_SERVER_BINARY not set (go build -o /tmp/ticketlens .)")
    return binary


async def _call(tool: str, args: dict[str, Any]) -> str:
    return await apply(_server(), [], RunTool(tool, args))


async def check_scenario(scenario: dict[str, Any]) -> dict[str, Any]:
    """Chama as tools de dados com parâmetros razoáveis e verifica se toda
    evidência exigida pelo ground truth aparece nos resultados."""
    ticket = scenario["ticket"]
    created = datetime.fromisoformat(ticket["created_at"].replace("Z", "+00:00"))
    iso = lambda dt: dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")  # noqa: E731

    outputs = [
        await _call("get_ticket", {"ticket_id": str(ticket["id"])}),
        await _call("search_related_tickets", {"query": ticket["subject"], "exclude_ticket_id": str(ticket["id"])}),
        await _call(
            "get_recent_deploys",
            {"repository": scenario["repository"], "since": iso(created - timedelta(hours=48)), "until": iso(created)},
        ),
        await _call(
            "search_logs",
            {"query": "*", "start_time": iso(created - timedelta(hours=2)), "end_time": iso(created + timedelta(hours=2))},
        ),
    ]
    blob = " ".join(outputs).lower()
    required = scenario["ground_truth"].get("required_evidence", [])
    missing = [item for item in required if item.lower() not in blob]
    return {"solvable": not missing, "missing_evidence": missing}


def extract_summary(messages: list[Any]) -> dict[str, Any] | None:
    """Pega os argumentos da última chamada a summarize_investigation feita
    pelo agente (é a conclusão estruturada que pontuamos)."""
    summary = None
    for message in messages:
        for call in getattr(message, "tool_calls", None) or []:
            if call.get("name") == "summarize_investigation":
                summary = call.get("args")
    return summary


async def run_agent_scenario(scenario: dict[str, Any]) -> dict[str, Any]:
    from ticketlens_agent.investigate import INVESTIGATION_PROMPT, build_agent

    agent = await build_agent()
    prompt = INVESTIGATION_PROMPT.format(ticket_id=scenario["ticket"]["id"])
    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": prompt}]},
        config={"recursion_limit": 30},
    )
    summary = extract_summary(result["messages"])
    tool_calls = sum(len(getattr(m, "tool_calls", None) or []) for m in result["messages"])
    return {"summary": summary, "tool_calls": tool_calls, **score(summary, scenario["ground_truth"])}


async def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--check", action="store_true", help="validate scenarios are solvable, no LLM")
    parser.add_argument("--only", help="run only scenarios whose file name contains this text")
    args = parser.parse_args()

    os.environ.update(MOCK_ENV)
    scenarios = load_scenarios(args.only)
    results: list[dict[str, Any]] = []

    for path, scenario in scenarios:
        with MockServer(path):
            try:
                if args.check:
                    outcome = await check_scenario(scenario)
                    ok = outcome["solvable"]
                else:
                    outcome = await run_agent_scenario(scenario)
                    ok = outcome["passed"]
            except Exception as exc:  # um cenário quebrado não derruba o relatório inteiro
                outcome, ok = {"error": f"{type(exc).__name__}: {exc}"}, False
        results.append({"scenario": scenario["id"], "ok": ok, **outcome})
        detail = outcome.get("reason") or outcome.get("missing_evidence") or outcome.get("error") or ""
        print(f"{'PASS' if ok else 'FAIL'}  {scenario['id']}  {detail if not ok else ''}")

    passed = sum(1 for r in results if r["ok"])
    rate = passed / len(results) if results else 0.0
    label = "solvable" if args.check else "passed"
    print(f"\n{passed}/{len(results)} {label} ({rate:.0%})")

    if not args.check:
        RESULTS_DIR.mkdir(exist_ok=True)
        out = RESULTS_DIR / f"run-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}.json"
        out.write_text(json.dumps(results, indent=2, default=str))
        print(f"Gate {PASS_GATE:.0%}: {'MET' if rate >= PASS_GATE else 'NOT MET'}. Details saved to {out}")
        return 0 if rate >= PASS_GATE else 1

    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
