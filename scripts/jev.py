from typing import Any
from typing import Dict
from typing import List
from typing import Mapping
from typing import Optional


class JevUsageError(ValueError):
    pass


def build_jev_questions(kind: str, instructions: str, options: Optional[List[str]]) -> Dict[str, Any]:
    if kind == "noul":
        return {
            "judgment": {
                "type": "noul",
                "instructions": instructions,
            },
        }
    if kind == "choice":
        return {
            "judgment": {
                "type": "choice",
                "instructions": instructions,
                "criteria": {option: None for option in options or []},
            },
        }
    if kind == "score":
        return {
            "judgment": {
                "type": "score",
                "instructions": instructions,
                "criteria": list(options or []),
            },
        }
    raise JevUsageError(f"Unsupported Jev kind: {kind}")


def evaluate_jev(client: Any, state: str, kind: str, instructions: str, options: Optional[List[str]]) -> str:
    questions = build_jev_questions(kind, instructions, options)
    response = client.system_one(state=state, questions=questions)
    answer = response.answers["judgment"]
    return format_jev_content(kind, judgment_to_mapping(kind, answer))


def judgment_to_mapping(kind: str, answer: Any) -> Dict[str, Any]:
    if kind == "noul":
        return {"noul": _field(answer, "noul")}
    if kind == "choice":
        return {
            "choice": _field(answer, "choice"),
            "confidence": _field(answer, "confidence"),
            "probabilities": _field(answer, "probabilities"),
        }
    if kind == "score":
        return {
            "score": _field(answer, "score"),
            "confidence": _field(answer, "confidence"),
            "legend": _field(answer, "legend"),
        }
    raise JevUsageError(f"Unsupported Jev kind: {kind}")


def _field(answer: Any, name: str) -> Any:
    if isinstance(answer, Mapping):
        return answer[name]
    return getattr(answer, name)


def format_jev_content(kind: str, answer: Mapping[str, Any]) -> str:
    if kind == "noul":
        noul = float(answer["noul"])
        yes_pct = _percent(noul)
        if noul >= 0.5:
            return f"Yes ({yes_pct}%)"
        return f"No ({yes_pct}% yes)"
    if kind == "choice":
        winner = str(answer["choice"])
        probabilities = answer.get("probabilities") or {}
        winner_pct = _percent(probabilities.get(winner, 0))
        lines = [f"*{winner}* ({winner_pct}%)"]
        for label, prob in probabilities.items():
            lines.append(f"- {label}: {_percent(prob)}%")
        return "\n".join(lines)
    if kind == "score":
        score = float(answer["score"])
        legend = answer.get("legend") or {}
        probabilities = answer.get("probabilities") or {}
        level = _legend_label(legend, score)
        header = f"{score:.2f} · {level}" if level else f"{score:.2f}"
        lines = [f"*{header}*"]
        for key in _ordered_score_keys(legend, probabilities):
            label = legend.get(key, legend.get(str(key), key))
            prob = probabilities.get(key, probabilities.get(str(key), 0))
            lines.append(f"- {label}: {_percent(prob)}%")
        return "\n".join(lines)
    raise JevUsageError(f"Unsupported Jev kind: {kind}")


def _percent(value: Any) -> int:
    return int(round(float(value) * 100))


def _ordered_score_keys(legend: Mapping[Any, Any], probabilities: Mapping[Any, Any]) -> List[Any]:
    keys = list(legend.keys()) if legend else list(probabilities.keys())
    try:
        return sorted(keys, key=lambda key: int(key))
    except (TypeError, ValueError):
        return keys


def _legend_label(legend: Mapping[Any, Any], score: float) -> str:
    nearest = int(round(score))
    if nearest in legend:
        return str(legend[nearest])
    as_str = str(nearest)
    if as_str in legend:
        return str(legend[as_str])
    return ""
