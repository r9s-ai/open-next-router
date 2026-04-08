package audio

import "testing"

func TestBuildSpeechDerivedUsage_Fallback(t *testing.T) {
	got := BuildSpeechDerivedUsage([]byte("not-audio"), 1.0)
	if got == nil {
		t.Fatalf("expected fallback derived usage")
	}
	if got["audio_duration_seconds"] != 1.0 {
		t.Fatalf("audio_duration_seconds=%v want=1.0", got["audio_duration_seconds"])
	}
}

func TestBuildSpeechDerivedUsage_NoFallback(t *testing.T) {
	got := BuildSpeechDerivedUsage([]byte("not-audio"), 0)
	if got != nil {
		t.Fatalf("expected nil without fallback, got=%v", got)
	}
}
