import unittest
from types import SimpleNamespace

from jev import (
    JevUsageError,
    build_jev_questions,
    evaluate_jev,
    format_jev_content,
)


class BuildJevQuestionsTest(unittest.TestCase):
    def test_noul_question_uses_instructions(t):
        questions = build_jev_questions("noul", "這則訊息急嗎", [])
        t.assertEqual(
            questions,
            {
                "judgment": {
                    "type": "noul",
                    "instructions": "這則訊息急嗎",
                },
            },
        )

    def test_choice_question_maps_options_as_labels(t):
        questions = build_jev_questions(
            "choice",
            "該誰處理",
            ["帳務", "技術", "業務"],
        )
        t.assertEqual(questions["judgment"]["type"], "choice")
        t.assertEqual(questions["judgment"]["instructions"], "該誰處理")
        t.assertEqual(
            questions["judgment"]["criteria"],
            {"帳務": None, "技術": None, "業務": None},
        )

    def test_score_question_keeps_option_order(t):
        questions = build_jev_questions(
            "score",
            "有多生氣",
            ["平靜", "有點煩", "非常生氣"],
        )
        t.assertEqual(
            questions["judgment"]["criteria"],
            ["平靜", "有點煩", "非常生氣"],
        )

    def test_unknown_kind_is_rejected(t):
        with t.assertRaises(JevUsageError):
            build_jev_questions("chat", "hello", [])


class FormatJevContentTest(unittest.TestCase):
    def test_noul_yes_lean(t):
        t.assertEqual(
            format_jev_content("noul", {"noul": 0.87}),
            "Yes (87%)",
        )

    def test_noul_no_lean(t):
        t.assertEqual(
            format_jev_content("noul", {"noul": 0.13}),
            "No (13% yes)",
        )

    def test_choice_shows_winner_then_all_percents(t):
        content = format_jev_content(
            "choice",
            {
                "choice": "技術",
                "confidence": 0.596,
                "probabilities": {
                    "帳務": 0.159,
                    "技術": 0.84,
                    "業務": 0.001,
                },
            },
        )
        t.assertEqual(
            content,
            "*技術* (84%)\n- 帳務: 16%\n- 技術: 84%\n- 業務: 0%",
        )

    def test_score_shows_value_and_nearest_level(t):
        content = format_jev_content(
            "score",
            {
                "score": 1.04,
                "confidence": 0.842,
                "legend": {
                    0: "平靜",
                    1: "有點煩",
                    2: "非常生氣",
                },
                "probabilities": {
                    0: 0.10,
                    1: 0.80,
                    2: 0.10,
                },
            },
        )
        t.assertEqual(
            content,
            "*1.04 · 有點煩*\n- 平靜: 10%\n- 有點煩: 80%\n- 非常生氣: 10%",
        )


class EvaluateJevTest(unittest.TestCase):
    def test_evaluate_jev_sends_state_and_formats_noul(t):
        captured = {}

        class FakeClient:
            def system_one(self, state, questions):
                captured["state"] = state
                captured["questions"] = questions
                return SimpleNamespace(
                    answers={
                        "judgment": SimpleNamespace(noul=0.87),
                    },
                )

        content = evaluate_jev(
            FakeClient(),
            "previous message",
            "noul",
            "這則訊息急嗎",
            [],
        )
        t.assertEqual(captured["state"], "previous message")
        t.assertEqual(captured["questions"]["judgment"]["type"], "noul")
        t.assertEqual(content, "Yes (87%)")


if __name__ == "__main__":
    unittest.main()
