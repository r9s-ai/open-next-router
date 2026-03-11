package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/config"
	"github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/gemini"
)

func TestExecuteContextRequiresAPIKey(t *testing.T) {
	t.Setenv("ONR_API_KEY", "")
	t.Setenv("ONR_BASE_URL", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := ExecuteContext(context.Background(), []string{"openai", "chat_completions", "hello"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ONR_API_KEY environment variable is not set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompletionCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := ExecuteContext(context.Background(), []string{"completion", "--shell", "zsh"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteContext error: %v", err)
	}
	if !strings.Contains(stdout.String(), "compdef") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestOpenAIChatVerbose(t *testing.T) {
	orig := openAIChatFunc
	openAIChatFunc = func(ctx context.Context, cfg config.ClientConfig, prompt, model string) (string, error) {
		return "hello", nil
	}
	defer func() { openAIChatFunc = orig }()

	t.Setenv("ONR_API_KEY", "k")
	t.Setenv("ONR_BASE_URL", "http://localhost:3300")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := ExecuteContext(context.Background(), []string{"openai", "chat_completions", "hello", "-v"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteContext error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "provider: openai/chat_completions") || !strings.Contains(out, "status: ok") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestGeminiChatsWritesImage(t *testing.T) {
	orig := geminiMultimodalChatFunc
	geminiMultimodalChatFunc = func(ctx context.Context, cfg config.ClientConfig, prompt, model string, modalities []string) (<-chan gemini.StreamEvent, <-chan error) {
		out := make(chan gemini.StreamEvent, 2)
		errc := make(chan error, 1)
		out <- gemini.StreamEvent{Type: "text", Text: "hello"}
		out <- gemini.StreamEvent{Type: "image", ImageBytes: []byte{1, 2, 3}, ImageMIMEType: "image/png"}
		close(out)
		errc <- nil
		close(errc)
		return out, errc
	}
	defer func() { geminiMultimodalChatFunc = orig }()

	t.Setenv("ONR_API_KEY", "k")
	t.Setenv("ONR_BASE_URL", "http://localhost:3300")

	dir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := ExecuteContext(context.Background(), []string{"gemini", "chats", "draw", "--image-output-dir", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteContext error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "gemini_1.png")); statErr != nil {
		t.Fatalf("expected image file: %v", statErr)
	}
	if !strings.Contains(stdout.String(), "[image saved]") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestSanitizeError(t *testing.T) {
	err := sanitizeError(errors.New("boom"))
	if err == nil || !strings.Contains(err.Error(), "request failed: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
