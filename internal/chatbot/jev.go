package chatbot

import (
	"strings"
)

type JevKind string

const (
	JevNoul   JevKind = "noul"
	JevChoice JevKind = "choice"
	JevScore  JevKind = "score"

	jevNoulTrigger = "@jev "
	jevPickTrigger = "@jev-pick "
	jevRateTrigger = "@jev-rate "

	DefaultChoiceInstructions = "Which option best fits the previous message?"
	DefaultScoreInstructions  = "Rate the previous message using these levels."

	JevNoulUsage = "Usage: @jev {yes/no question} (judges the previous message)"
	JevPickUsage = "Usage: @jev-pick {option} | {option} | ... or @jev-pick {question}: {option} | {option} | ..."
	JevRateUsage = "Usage: @jev-rate {level} | {level} | ... or @jev-rate {question}: {level} | {level} | ..."
	JevNoState   = "No previous message to judge."
)

type JevCommand struct {
	Kind         JevKind
	Instructions string
	Options      []string
	UsageError   string
}

func ParseJevCommand(msg string) (JevCommand, bool) {
	if strings.Contains(msg, jevPickTrigger) {
		return parseJevWithOptions(
			strings.ReplaceAll(msg, jevPickTrigger, ""),
			JevChoice,
			DefaultChoiceInstructions,
			JevPickUsage,
		), true
	}
	if strings.Contains(msg, jevRateTrigger) {
		return parseJevWithOptions(
			strings.ReplaceAll(msg, jevRateTrigger, ""),
			JevScore,
			DefaultScoreInstructions,
			JevRateUsage,
		), true
	}
	if strings.Contains(msg, jevNoulTrigger) {
		instructions := strings.TrimSpace(strings.ReplaceAll(msg, jevNoulTrigger, ""))
		cmd := JevCommand{Kind: JevNoul, Instructions: instructions}
		if instructions == "" {
			cmd.UsageError = JevNoulUsage
		}
		return cmd, true
	}
	return JevCommand{}, false
}

func parseJevWithOptions(rest string, kind JevKind, defaultInstructions, usage string) JevCommand {
	rest = strings.TrimSpace(rest)
	instructions := defaultInstructions
	optionsPart := rest

	colonIdx := indexQuestionColon(rest)
	if colonIdx >= 0 {
		left := strings.TrimSpace(rest[:colonIdx])
		right := strings.TrimSpace(rest[colonIdx+len(questionColonAt(rest, colonIdx)):])
		if left != "" {
			instructions = left
		}
		optionsPart = right
	}

	options := splitJevOptions(optionsPart)
	cmd := JevCommand{
		Kind:         kind,
		Instructions: instructions,
		Options:      options,
	}
	if len(options) < 2 {
		cmd.UsageError = usage
	}
	return cmd
}

func questionColonAt(s string, idx int) string {
	if strings.HasPrefix(s[idx:], "：") {
		return "："
	}
	return ":"
}

func indexQuestionColon(s string) int {
	ascii := strings.Index(s, ":")
	full := strings.Index(s, "：")
	switch {
	case ascii < 0:
		return full
	case full < 0:
		return ascii
	case full < ascii:
		return full
	default:
		return ascii
	}
}

func splitJevOptions(part string) []string {
	raw := strings.Split(part, "|")
	options := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			options = append(options, item)
		}
	}
	return options
}
