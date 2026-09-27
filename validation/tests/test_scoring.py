"""Testa o scoring e a consistência interna de cada cenário, sem LLM:

- a resposta "gabarito" (derivada do próprio ground truth) tem que passar;
- culpar o red herring do cenário tem que reprovar;
- nenhuma conclusão (agente não chamou summarize_investigation) reprova;
- confiança fora do esperado reprova.

Se alguém editar um cenário e deixar o `root_cause` inconsistente com o
`accept_any`, estes testes pegam antes de gastar tokens com o agente.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

VALIDATION_DIR = Path(__file__).parent.parent
sys.path.insert(0, str(VALIDATION_DIR))
from scoring import score  # noqa: E402

SCENARIOS = sorted((VALIDATION_DIR / "scenarios").glob("*.json"))


def _load(path: Path) -> dict:
    return json.loads(path.read_text())


def _gold_summary(scenario: dict) -> dict:
    gt = scenario["ground_truth"]
    return {
        "ticket_id": str(scenario["ticket"]["id"]),
        "hypothesis": gt["root_cause"],
        "confidence": gt["expected_confidence"][0],
        "evidence": list(gt["required_evidence"]),
        "suggested_next_step": "",
    }


def test_there_are_scenarios():
    assert len(SCENARIOS) >= 5


@pytest.mark.parametrize("path", SCENARIOS, ids=lambda p: p.stem)
def test_gold_answer_passes(path: Path):
    scenario = _load(path)
    result = score(_gold_summary(scenario), scenario["ground_truth"])
    assert result["passed"], result["reason"]


@pytest.mark.parametrize("path", SCENARIOS, ids=lambda p: p.stem)
def test_blaming_red_herring_fails(path: Path):
    scenario = _load(path)
    red_herring = scenario["ground_truth"]["must_not_blame"][0]
    summary = _gold_summary(scenario)
    summary["hypothesis"] = f"The deploy {red_herring} broke it. " + summary["hypothesis"]
    result = score(summary, scenario["ground_truth"])
    assert not result["passed"]
    assert result["blamed_red_herring"]


@pytest.mark.parametrize("path", SCENARIOS, ids=lambda p: p.stem)
def test_required_evidence_exists_in_scenario_data(path: Path):
    """Toda evidência exigida tem que existir nos dados que o mock serve,
    senão o cenário seria impossível de resolver."""
    scenario = _load(path)
    data = json.dumps(
        [scenario["ticket"], scenario["related_tickets"], scenario["deploys"], scenario["logs"]]
    ).lower()
    for item in scenario["ground_truth"]["required_evidence"]:
        assert item.lower() in data, f"{item!r} not present in scenario data"


@pytest.mark.parametrize("path", SCENARIOS, ids=lambda p: p.stem)
def test_every_scenario_cites_a_public_source(path: Path):
    scenario = _load(path)
    assert scenario["based_on"], "each scenario must be grounded in a real public case"
    assert all(src["url"].startswith("https://") for src in scenario["based_on"])


def test_no_summary_fails():
    scenario = _load(SCENARIOS[0])
    result = score(None, scenario["ground_truth"])
    assert not result["passed"]


def test_wrong_confidence_fails_on_calibration_scenario():
    scenario = _load(next(p for p in SCENARIOS if "insufficient" in p.stem))
    summary = _gold_summary(scenario)
    summary["confidence"] = "high"
    result = score(summary, scenario["ground_truth"])
    assert not result["passed"]
    assert not result["confidence_ok"]


def test_generic_answer_does_not_match_root_cause():
    scenario = _load(next(p for p in SCENARIOS if "saml" in p.stem))
    summary = _gold_summary(scenario)
    summary["hypothesis"] = "Something is wrong with authentication, needs more investigation."
    result = score(summary, scenario["ground_truth"])
    assert not result["root_cause_match"]
    assert not result["passed"]
