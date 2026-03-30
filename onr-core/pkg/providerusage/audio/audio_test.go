package audio

import (
	"math"
	"testing"
)

func TestDetectFormatWAV(t *testing.T) {
	data := append([]byte("RIFF"), []byte{0, 0, 0, 0}...)
	data = append(data, []byte("WAVE")...)
	if got, want := DetectFormat(data), FormatWAV; got != want {
		t.Fatalf("DetectFormat got %q want %q", got, want)
	}
}

func TestDetectFormatUnknown(t *testing.T) {
	if got, want := DetectFormat([]byte("abc")), FormatUnknown; got != want {
		t.Fatalf("DetectFormat got %q want %q", got, want)
	}
}

func TestEstimatedTokensFromDuration(t *testing.T) {
	if got, want := EstimatedTokensFromDuration(1.5), 32; got != want {
		t.Fatalf("EstimatedTokensFromDuration got %d want %d", got, want)
	}
}

func TestSumEstimatedTokensFromPayloads_Empty(t *testing.T) {
	got, err := SumEstimatedTokensFromPayloads(nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestSumDurationsFromPayloads_Empty(t *testing.T) {
	got, err := SumDurationsFromPayloads(nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if math.Abs(got) > 1e-9 {
		t.Fatalf("got %v want 0", got)
	}
}

func TestDurationFromBytesOrDefault(t *testing.T) {
	if got, want := DurationFromBytesOrDefault([]byte("bad"), 1.0), 1.0; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSumEstimatedTokensFromPayloadsOrDefault(t *testing.T) {
	got := SumEstimatedTokensFromPayloadsOrDefault([][]byte{[]byte("bad")}, 1.0)
	if got != 21 {
		t.Fatalf("got %d want 21", got)
	}
}
