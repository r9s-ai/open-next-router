package usageestimate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLI_BodyFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{
		"--model", "claude-opus-4-7",
		"--api", apiMessages,
		"--direction", "input",
		"--body", `{"messages":[{"role":"user","content":"hello"}]}`,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	assertCLIOutputContains(t, stdout.String(), "tokens=", "model=claude-opus-4-7", "api=claude.messages", "direction=estimate_input", "source=body")
}

func TestRunCLI_BodyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "request.json")
	if err := os.WriteFile(path, []byte(`{"messages":[{"role":"user","content":"hello from file"}]}`), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{
		"--model", "claude-opus-4-7",
		"--api", apiMessages,
		"--body-file", path,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	assertCLIOutputContains(t, stdout.String(), "tokens=", "source="+path)
}

func TestRunCLI_Stdin(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{
		"--model", "claude-opus-4-7",
		"--api", apiMessages,
	}, strings.NewReader(`{"messages":[{"role":"user","content":"hello from stdin"}]}`), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	assertCLIOutputContains(t, stdout.String(), "tokens=", "source=stdin")
}

func TestRunCLI_TextBody(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{
		"--model", "claude-opus-4-7",
		"--api", apiMessages,
		"--body", "plain text body",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	assertCLIOutputContains(t, stdout.String(), "tokens=", "source=body")
}

func TestRunCLI_RequiresModel(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{
		"--api", apiMessages,
		"--body", `{"messages":[]}`,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("code=%d want=2 stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "model is required") {
		t.Fatalf("stderr=%q want model error", stderr.String())
	}
}

func TestRunCLI_RejectsBodyAndBodyFileTogether(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{
		"--model", "claude-opus-4-7",
		"--body", "x",
		"--body-file", "request.json",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("code=%d want=2 stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--body and --body-file cannot be used together") {
		t.Fatalf("stderr=%q want body conflict error", stderr.String())
	}
}

func assertCLIOutputContains(t *testing.T, output string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(output, part) {
			t.Fatalf("output=%q want to contain %q", output, part)
		}
	}
}
