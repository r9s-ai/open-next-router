package metrics

import (
	"fmt"
	"io"
)

type RequestMetrics struct {
	Provider         string
	Model            string
	BaseURL          string
	Stream           bool
	ElapsedSec       float64
	TextChars        int
	ImageCount       int
	Status           string
	ExceptionMessage string
}

func Print(w io.Writer, m RequestMetrics) {
	elapsed := m.ElapsedSec
	if elapsed <= 0 {
		elapsed = 1e-9
	}
	tps := float64(m.TextChars) / elapsed

	fmt.Fprintln(w, "\n=== Request Metrics ===")
	fmt.Fprintf(w, "provider: %s\n", m.Provider)
	fmt.Fprintf(w, "model: %s\n", m.Model)
	fmt.Fprintf(w, "base_url: %s\n", m.BaseURL)
	fmt.Fprintf(w, "stream: %t\n", m.Stream)
	fmt.Fprintf(w, "elapsed_sec: %.3f\n", m.ElapsedSec)
	fmt.Fprintf(w, "text_chars: %d\n", m.TextChars)
	fmt.Fprintf(w, "text_tps: %.2f chars/sec\n", tps)
	fmt.Fprintf(w, "status: %s\n", m.Status)
	if m.ImageCount > 0 {
		fmt.Fprintf(w, "images: %d\n", m.ImageCount)
	}
	if m.ExceptionMessage != "" {
		fmt.Fprintf(w, "exception: %s\n", m.ExceptionMessage)
	}
}
