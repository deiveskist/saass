"""Pontua o resultado do `summarize_investigation` do agente contra o
ground truth de um cenário.

Quatro critérios, inspirados nos benchmarks de RCA com LLM (OpenRCA,
RCAEval, AIOpsLab). Eles mostram que acertar a resposta não basta: muitos
acertos vêm de raciocínio errado. Por isso checamos também a evidência
citada, e não só a hipótese.

1. root_cause_match: a hipótese+evidência bate com pelo menos um grupo de
   `accept_any` (todas as palavras do grupo presentes, sem diferenciar
   maiúsculas).
2. evidence_score: fração de `required_evidence` que aparece na evidência
   citada pelo agente (prova que ele olhou os dados certos).
3. blamed_red_herring: a hipótese culpa algum item de `must_not_blame`
   (um deploy ou causa que é distração no cenário).
4. confidence_ok: a confiança declarada está em `expected_confidence`.

Passa se: root_cause_match E sem red herring E evidence_score >= 0.5 E
confidence_ok.

Limitação conhecida: casamento por palavras-chave é grosseiro. Um juiz LLM
(comparando a hipótese com `root_cause` em linguagem natural) é o próximo
passo; as palavras-chave ficam como checagem barata e determinística.
"""

from __future__ import annotations

from typing import Any

PASSING_EVIDENCE_SCORE = 0.5


def score(summary: dict[str, Any] | None, ground_truth: dict[str, Any]) -> dict[str, Any]:
    if summary is None:
        return {
            "passed": False,
            "reason": "agent never called summarize_investigation",
            "root_cause_match": False,
            "evidence_score": 0.0,
            "blamed_red_herring": [],
            "confidence_ok": False,
        }

    hypothesis = str(summary.get("hypothesis") or "")
    evidence_items = summary.get("evidence") or []
    evidence = " ".join(str(e) for e in evidence_items)
    next_step = str(summary.get("suggested_next_step") or "")

    searchable = f"{hypothesis} {evidence} {next_step}".lower()
    root_cause_match = any(
        all(keyword.lower() in searchable for keyword in group)
        for group in ground_truth["accept_any"]
    )

    required = ground_truth.get("required_evidence") or []
    evidence_lower = evidence.lower()
    found = [item for item in required if item.lower() in evidence_lower]
    evidence_score = len(found) / len(required) if required else 1.0

    hypothesis_lower = hypothesis.lower()
    blamed = [item for item in ground_truth.get("must_not_blame", []) if item.lower() in hypothesis_lower]

    confidence = str(summary.get("confidence") or "").lower()
    confidence_ok = confidence in ground_truth.get("expected_confidence", ["low", "medium", "high"])

    passed = root_cause_match and not blamed and evidence_score >= PASSING_EVIDENCE_SCORE and confidence_ok

    reasons = []
    if not root_cause_match:
        reasons.append("hypothesis does not match the root cause")
    if blamed:
        reasons.append(f"blamed a red herring: {', '.join(blamed)}")
    if evidence_score < PASSING_EVIDENCE_SCORE:
        reasons.append(f"cited too little required evidence ({len(found)}/{len(required)})")
    if not confidence_ok:
        reasons.append(f"confidence '{confidence}' not in {ground_truth.get('expected_confidence')}")

    return {
        "passed": passed,
        "reason": "ok" if passed else "; ".join(reasons),
        "root_cause_match": root_cause_match,
        "evidence_score": round(evidence_score, 2),
        "evidence_found": found,
        "blamed_red_herring": blamed,
        "confidence_ok": confidence_ok,
    }
