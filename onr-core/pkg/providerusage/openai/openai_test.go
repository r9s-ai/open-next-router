package openai

import "testing"

func TestImageCountFromResponseBody(t *testing.T) {
	got, ok, err := ImageCountFromResponseBody([]byte(`{"data":[{"b64_json":"x"},{"b64_json":"y"}]}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != 2 {
		t.Fatalf("got %v want 2", got)
	}
}

func TestImageCountFromResponseBody_Empty(t *testing.T) {
	got, ok, err := ImageCountFromResponseBody([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false, got quantity=%v", got)
	}
}

func TestAudioUsageSecondsFromResponseBody(t *testing.T) {
	got, ok, err := AudioUsageSecondsFromResponseBody([]byte(`{"text":"hello","usage":{"type":"duration","seconds":4}}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != 4 {
		t.Fatalf("got %v want 4", got)
	}
}

func TestAudioUsageSecondsFromResponseBody_Missing(t *testing.T) {
	got, ok, err := AudioUsageSecondsFromResponseBody([]byte(`{"text":"hello translated"}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false, got quantity=%v", got)
	}
}

func TestCompletedWebSearchCallsFromResponseBody(t *testing.T) {
	got, ok, err := CompletedWebSearchCallsFromResponseBody([]byte(`{
		"output":[
			{"type":"web_search_call","status":"completed"},
			{"type":"web_search_call","status":"failed"},
			{"type":"message","status":"completed"},
			{"type":"web_search_call","status":"completed"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != 2 {
		t.Fatalf("got %v want 2", got)
	}
}

func TestCompletedWebSearchCallsFromResponseBody_Missing(t *testing.T) {
	got, ok, err := CompletedWebSearchCallsFromResponseBody([]byte(`{"output":[{"type":"message","status":"completed"}]}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false, got quantity=%v", got)
	}
}

func TestAudioInputSeconds(t *testing.T) {
	got, ok := AudioInputSeconds("duration", 0, 3)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != 3 {
		t.Fatalf("got %v want 3", got)
	}
}

func TestAudioInputMTokens_FromDuration(t *testing.T) {
	got, ok := AudioInputMTokens("duration", 0, 60)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := 1250.0 / 1000000.0
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAudioInputSecondsOrPayloads_UsageWins(t *testing.T) {
	got := AudioInputSecondsOrPayloads(nil, "duration", 0, 4)
	if got != 4 {
		t.Fatalf("got %v want 4", got)
	}
}

func TestAudioInputMTokensOrPayloads_UsageWins(t *testing.T) {
	got := AudioInputMTokensOrPayloads(nil, "duration", 0, 60)
	want := 1250.0 / 1000000.0
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTextKCharacters(t *testing.T) {
	got := TextKCharacters("hello世界")
	if got != 0.007 {
		t.Fatalf("got %v want 0.007", got)
	}
}

func TestTextMTokens(t *testing.T) {
	got := TextMTokens("hello", func(text string) int {
		if text != "hello" {
			t.Fatalf("unexpected text %q", text)
		}
		return 1234
	})
	if got != 0.001234 {
		t.Fatalf("got %v want 0.001234", got)
	}
}

func TestAudioTranslationDerivedQuantities(t *testing.T) {
	got := AudioTranslationDerivedQuantities(nil, "duration", 0, 60, "hello", func(text string) int {
		if text != "hello" {
			t.Fatalf("unexpected text %q", text)
		}
		return 1234
	})
	if got.InputSeconds != 60 {
		t.Fatalf("InputSeconds=%v want 60", got.InputSeconds)
	}
	if got.InputMTokens != 0.00125 {
		t.Fatalf("InputMTokens=%v want 0.00125", got.InputMTokens)
	}
	if got.OutputKCharacters != 0.005 {
		t.Fatalf("OutputKCharacters=%v want 0.005", got.OutputKCharacters)
	}
	if got.OutputMTokens != 0.001234 {
		t.Fatalf("OutputMTokens=%v want 0.001234", got.OutputMTokens)
	}
}

func TestAudioSpeechDerivedQuantities(t *testing.T) {
	wav := []byte{
		'R', 'I', 'F', 'F', 0x34, 0x00, 0x00, 0x00, 'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ', 0x10, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
		0x40, 0x1F, 0x00, 0x00, 0x80, 0x3E, 0x00, 0x00, 0x02, 0x00, 0x10, 0x00,
		'd', 'a', 't', 'a', 0x10, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00,
		0x04, 0x00, 0x05, 0x00, 0x06, 0x00, 0x07, 0x00,
	}
	got := AudioSpeechDerivedQuantities("hello", wav, func(text string) int {
		if text != "hello" {
			t.Fatalf("unexpected text %q", text)
		}
		return 1234
	})
	if got.InputKCharacters != 0.005 {
		t.Fatalf("InputKCharacters=%v want 0.005", got.InputKCharacters)
	}
	if got.InputMTokens != 0.001234 {
		t.Fatalf("InputMTokens=%v want 0.001234", got.InputMTokens)
	}
	if got.OutputSeconds <= 0 {
		t.Fatalf("OutputSeconds=%v want >0", got.OutputSeconds)
	}
	if got.OutputMTokens <= 0 {
		t.Fatalf("OutputMTokens=%v want >0", got.OutputMTokens)
	}
}
