package chatbot

import "testing"

func TestParseJevCommand_notAJevMessage(t *testing.T) {
	_, ok := ParseJevCommand("@gemini is this urgent?")
	if ok {
		t.Fatal("expected non-jev message to be ignored")
	}
}

func TestParseJevCommand_noulUsesQuestionText(t *testing.T) {
	cmd, ok := ParseJevCommand("@jev 這則訊息急嗎")
	if !ok {
		t.Fatal("expected jev yes/no command")
	}
	if cmd.Kind != JevNoul {
		t.Fatalf("kind=%q", cmd.Kind)
	}
	if cmd.Instructions != "這則訊息急嗎" {
		t.Fatalf("instructions=%q", cmd.Instructions)
	}
	if len(cmd.Options) != 0 {
		t.Fatalf("options=%v", cmd.Options)
	}
	if cmd.UsageError != "" {
		t.Fatalf("usage=%q", cmd.UsageError)
	}
}

func TestParseJevCommand_pickAndRateAreNotNoul(t *testing.T) {
	pick, ok := ParseJevCommand("@jev-pick 帳務 | 技術 | 業務")
	if !ok || pick.Kind != JevChoice {
		t.Fatalf("pick ok=%v kind=%q", ok, pick.Kind)
	}
	rate, ok := ParseJevCommand("@jev-rate 平靜 | 有點煩 | 非常生氣")
	if !ok || rate.Kind != JevScore {
		t.Fatalf("rate ok=%v kind=%q", ok, rate.Kind)
	}
}

func TestParseJevCommand_optionsWithoutQuestionUseDefaultInstructions(t *testing.T) {
	pick, _ := ParseJevCommand("@jev-pick 帳務 | 技術 | 業務")
	if pick.Instructions != DefaultChoiceInstructions {
		t.Fatalf("pick instructions=%q", pick.Instructions)
	}
	if got := pick.Options; len(got) != 3 || got[0] != "帳務" || got[1] != "技術" || got[2] != "業務" {
		t.Fatalf("pick options=%v", got)
	}

	rate, _ := ParseJevCommand("@jev-rate 平靜 | 有點煩 | 非常生氣")
	if rate.Instructions != DefaultScoreInstructions {
		t.Fatalf("rate instructions=%q", rate.Instructions)
	}
	if got := rate.Options; len(got) != 3 || got[0] != "平靜" || got[2] != "非常生氣" {
		t.Fatalf("rate options=%v", got)
	}
}

func TestParseJevCommand_colonSplitsQuestionFromOptions(t *testing.T) {
	pick, _ := ParseJevCommand("@jev-pick 該誰處理: 帳務 | 技術 | 業務")
	if pick.Instructions != "該誰處理" {
		t.Fatalf("instructions=%q", pick.Instructions)
	}
	if got := pick.Options; len(got) != 3 || got[0] != "帳務" {
		t.Fatalf("options=%v", got)
	}

	rate, _ := ParseJevCommand("@jev-rate 有多生氣：平靜 | 有點煩 | 非常生氣")
	if rate.Instructions != "有多生氣" {
		t.Fatalf("fullwidth colon instructions=%q", rate.Instructions)
	}
}

func TestParseJevCommand_usageErrors(t *testing.T) {
	noul, ok := ParseJevCommand("@jev   ")
	if !ok || noul.UsageError == "" {
		t.Fatalf("empty noul should be a usage error, got %+v ok=%v", noul, ok)
	}

	pick, ok := ParseJevCommand("@jev-pick only-one")
	if !ok || pick.UsageError == "" {
		t.Fatalf("single pick option should be a usage error, got %+v ok=%v", pick, ok)
	}

	rate, ok := ParseJevCommand("@jev-rate 平靜")
	if !ok || rate.UsageError == "" {
		t.Fatalf("single rate level should be a usage error, got %+v ok=%v", rate, ok)
	}
}
