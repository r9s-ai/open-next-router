package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/r9s-ai/open-next-router/onr-admin/internal/providersource"
)

func TestRootCmdHasUpdateSubcommands(t *testing.T) {
	t.Parallel()

	root := newRootCmd()
	if _, _, err := root.Find([]string{"update"}); err != nil {
		t.Fatalf("find update subcommand: %v", err)
	}
	for _, sub := range []string{"onr", "onr-admin", "providers", "all"} {
		if _, _, err := root.Find([]string{"update", sub}); err != nil {
			t.Fatalf("find update %s subcommand: %v", sub, err)
		}
	}
}

func TestUpdateCmdRejectsAllFlag(t *testing.T) {
	t.Parallel()

	root := newRootCmd()
	root.SetArgs([]string{"update", "--all"})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "--all") || !strings.Contains(err.Error(), "update all") {
		t.Fatalf("err=%q", err.Error())
	}
}

func TestRunUpdateAll_Order(t *testing.T) {
	origResolve := resolveUpdateReleaseTagFn
	origOnr := runUpdateOnrFn
	origProviders := runUpdateProvidersFn
	origAdmin := runUpdateOnrAdminFn
	defer func() {
		resolveUpdateReleaseTagFn = origResolve
		runUpdateOnrFn = origOnr
		runUpdateProvidersFn = origProviders
		runUpdateOnrAdminFn = origAdmin
	}()

	order := make([]string, 0, 3)
	resolveUpdateReleaseTagFn = func(ctx context.Context, version string, repo string, client *http.Client) (string, error) {
		return "v1.2.3", nil
	}
	runUpdateOnrFn = func(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
		order = append(order, "onr")
		return updateResult{applyHint: true}, nil
	}
	runUpdateProvidersFn = func(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
		order = append(order, "providers")
		return updateResult{}, nil
	}
	runUpdateOnrAdminFn = func(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
		order = append(order, "onr-admin")
		return updateResult{}, nil
	}

	var out bytes.Buffer
	err := runUpdateTarget(context.Background(), "all", updateOptions{}, defaultUpdateDeps(), &out)
	if err != nil {
		t.Fatalf("runUpdateTarget: %v", err)
	}
	if got, want := strings.Join(order, ","), "onr,providers,onr-admin"; got != want {
		t.Fatalf("order=%q want=%q", got, want)
	}
	if !strings.Contains(out.String(), "summary") {
		t.Fatalf("output=%q", out.String())
	}
}

func TestRunUpdateAll_PartialFailure(t *testing.T) {
	origResolve := resolveUpdateReleaseTagFn
	origOnr := runUpdateOnrFn
	origProviders := runUpdateProvidersFn
	origAdmin := runUpdateOnrAdminFn
	defer func() {
		resolveUpdateReleaseTagFn = origResolve
		runUpdateOnrFn = origOnr
		runUpdateProvidersFn = origProviders
		runUpdateOnrAdminFn = origAdmin
	}()

	resolveUpdateReleaseTagFn = func(ctx context.Context, version string, repo string, client *http.Client) (string, error) {
		return "v1.2.3", nil
	}
	order := make([]string, 0, 3)
	runUpdateOnrFn = func(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
		order = append(order, "onr")
		return updateResult{}, nil
	}
	runUpdateProvidersFn = func(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
		order = append(order, "providers")
		return updateResult{}, errors.New("boom")
	}
	runUpdateOnrAdminFn = func(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
		order = append(order, "onr-admin")
		return updateResult{}, nil
	}

	var out bytes.Buffer
	err := runUpdateTarget(context.Background(), "all", updateOptions{}, defaultUpdateDeps(), &out)
	if err == nil {
		t.Fatalf("expected error")
	}
	if got, want := strings.Join(order, ","), "onr,providers,onr-admin"; got != want {
		t.Fatalf("order=%q want=%q", got, want)
	}
	if !strings.Contains(out.String(), "status=failed") {
		t.Fatalf("output=%q", out.String())
	}
}

func TestNormalizeRuntimeTag(t *testing.T) {
	t.Parallel()

	tag, err := normalizeRuntimeTag("1.2.3")
	if err != nil {
		t.Fatalf("normalizeRuntimeTag: %v", err)
	}
	if tag != "v1.2.3" {
		t.Fatalf("tag=%q", tag)
	}
	if _, err := normalizeRuntimeTag("onr-core/v1.2.3"); err == nil {
		t.Fatalf("expect error")
	}
}

func TestRuntimeBinaryAssetName(t *testing.T) {
	t.Parallel()

	asset, err := runtimeBinaryAssetName("v1.2.3", "linux", "amd64")
	if err != nil {
		t.Fatalf("runtimeBinaryAssetName: %v", err)
	}
	if got, want := asset, "open-next-router_1.2.3_linux_x86_64.tar.gz"; got != want {
		t.Fatalf("asset=%q want=%q", got, want)
	}
}

func TestFindChecksumEntry(t *testing.T) {
	t.Parallel()

	checksum, err := findChecksumEntry("abc123  dist/open-next-router_1.2.3_linux_x86_64.tar.gz\n", "open-next-router_1.2.3_linux_x86_64.tar.gz")
	if err != nil {
		t.Fatalf("findChecksumEntry: %v", err)
	}
	if checksum != "abc123" {
		t.Fatalf("checksum=%q", checksum)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := verifyFileSHA256(path, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"); err != nil {
		t.Fatalf("verifyFileSHA256: %v", err)
	}
	if err := verifyFileSHA256(path, "deadbeef"); err == nil {
		t.Fatalf("expect mismatch error")
	}
}

func TestResolveBinaryTargetPath_PreferFlag(t *testing.T) {
	t.Parallel()

	called := false
	got, err := resolveBinaryTargetPath("/tmp/onr", "onr", func(file string) (string, error) {
		called = true
		return "", nil
	})
	if err != nil {
		t.Fatalf("resolveBinaryTargetPath: %v", err)
	}
	if called {
		t.Fatalf("lookPath should not be called")
	}
	if got != "/tmp/onr" {
		t.Fatalf("got=%q", got)
	}
}

func TestResolveUpdateProvidersSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	providersDir := filepath.Join(configDir, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";
provider "openai" {
  defaults { upstream_config { base_url = "https://api.openai.com"; } }
}
`), 0o600); err != nil {
		t.Fatalf("write provider: %v", err)
	}
	onrConf := filepath.Join(configDir, "onr.conf")
	if err := os.WriteFile(onrConf, []byte(`
syntax "next-router/0.1";
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("write onr.conf: %v", err)
	}
	cfgPath := filepath.Join(dir, "onr.yaml")
	cfg := "auth:\n  api_key: \"x\"\nproviders:\n  dir: \"" + onrConf + "\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	got, err := resolveUpdateProvidersSource(updateOptions{cfgPath: cfgPath})
	if err != nil {
		t.Fatalf("resolveUpdateProvidersSource: %v", err)
	}
	if got.SourcePath != onrConf || got.EditablePath != providersDir || !got.SourceIsFile || got.EditableIsFile {
		t.Fatalf("unexpected source info: %+v", got)
	}
}

func TestResolveUpdateProvidersSource_AllowsMissingOverrideDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "providers")
	got, err := resolveUpdateProvidersSource(updateOptions{providersDir: dir})
	if err != nil {
		t.Fatalf("resolveUpdateProvidersSource: %v", err)
	}
	if got.SourcePath != dir || got.EditablePath != dir || got.SourceIsFile || got.EditableIsFile {
		t.Fatalf("unexpected source info: %+v", got)
	}
}

func TestUpdateMergedProviderSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourcePath := filepath.Join(root, "providers.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults { upstream_config { base_url = "https://old.example.com"; } }
}
`), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	files := map[string][]byte{
		"openai.conf": []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com/v2"; }
    auth { auth_bearer; }
  }

  match api = "chat.completions" {
    upstream { set_path "/v1/chat/completions"; }
    response { resp_passthrough; }
  }
}
`),
	}
	changed, unchanged, err := updateMergedProviderSource(providersource.Info{
		SourcePath:     sourcePath,
		SourceIsFile:   true,
		EditablePath:   sourcePath,
		EditableIsFile: true,
	}, []string{"openai.conf"}, files, false, func() time.Time { return time.Unix(0, 0) })
	if err != nil {
		t.Fatalf("updateMergedProviderSource: %v", err)
	}
	if changed != 1 || unchanged != 0 {
		t.Fatalf("changed=%d unchanged=%d", changed, unchanged)
	}
	body, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !strings.Contains(string(body), `https://api.openai.com/v2`) {
		t.Fatalf("updated source missing new base_url: %s", string(body))
	}
}

func TestUpdateCompositeProviderSource_MixedInlineAndIncludedDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	sourcePath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults { upstream_config { base_url = "https://old.example.com"; } }
}

include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "anthropic.conf"), []byte(`
syntax "next-router/0.1";

provider "anthropic" {
  defaults { upstream_config { base_url = "https://old.anthropic.example.com"; } }
}
`), 0o600); err != nil {
		t.Fatalf("write anthropic source: %v", err)
	}

	files := map[string][]byte{
		"openai.conf": []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults { upstream_config { base_url = "https://api.openai.com"; } }
}
`),
		"anthropic.conf": []byte(`
syntax "next-router/0.1";

provider "anthropic" {
  defaults { upstream_config { base_url = "https://api.anthropic.com"; } }
}
`),
		"gemini.conf": []byte(`
syntax "next-router/0.1";

provider "gemini" {
  defaults { upstream_config { base_url = "https://generativelanguage.googleapis.com"; } }
}
`),
	}
	changed, unchanged, err := updateCompositeProviderSource(providersource.Info{
		SourcePath:     sourcePath,
		SourceIsFile:   true,
		EditablePath:   "",
		EditableIsFile: false,
	}, []string{"openai.conf", "anthropic.conf", "gemini.conf"}, files, false, func() time.Time { return time.Unix(0, 0) })
	if err != nil {
		t.Fatalf("updateCompositeProviderSource: %v", err)
	}
	if changed != 3 || unchanged != 0 {
		t.Fatalf("changed=%d unchanged=%d", changed, unchanged)
	}
	body, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !strings.Contains(string(body), `https://api.openai.com`) {
		t.Fatalf("openai not updated in merged source: %s", string(body))
	}
	anthropicBody, err := os.ReadFile(filepath.Join(providersDir, "anthropic.conf"))
	if err != nil {
		t.Fatalf("read anthropic source: %v", err)
	}
	if !strings.Contains(string(anthropicBody), `https://api.anthropic.com`) {
		t.Fatalf("anthropic not updated in providers dir: %s", string(anthropicBody))
	}
	geminiBody, err := os.ReadFile(filepath.Join(providersDir, "gemini.conf"))
	if err != nil {
		t.Fatalf("read gemini source: %v", err)
	}
	if !strings.Contains(string(geminiBody), `https://generativelanguage.googleapis.com`) {
		t.Fatalf("gemini not created in providers dir: %s", string(geminiBody))
	}
}

func TestExtractProviderFilesFromConfigArchive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "cfg.tar.gz")
	if err := writeTestTarGz(archivePath, map[string]string{
		"config/providers/openai.conf": `syntax "next-router/0.1";`,
		"config/keys.example.yaml":     "x",
	}); err != nil {
		t.Fatalf("writeTestTarGz: %v", err)
	}
	files, err := extractProviderFilesFromConfigArchive(archivePath)
	if err != nil {
		t.Fatalf("extractProviderFilesFromConfigArchive: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files=%v", files)
	}
	if string(files["openai.conf"]) != `syntax "next-router/0.1";` {
		t.Fatalf("content=%q", string(files["openai.conf"]))
	}
}

func TestRunUpdateOnrAdmin_WindowsUnsupported(t *testing.T) {
	t.Parallel()

	_, err := runUpdateOnrAdmin(context.Background(), "v1.2.3", updateOptions{}, updateDeps{goos: "windows"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "windows") {
		t.Fatalf("err=%q", err.Error())
	}
}

func TestRunUpdateTarget_ProvidersUnchanged_NoHint(t *testing.T) {
	origResolve := resolveUpdateReleaseTagFn
	origProviders := runUpdateProvidersFn
	defer func() {
		resolveUpdateReleaseTagFn = origResolve
		runUpdateProvidersFn = origProviders
	}()

	resolveUpdateReleaseTagFn = func(ctx context.Context, version string, repo string, client *http.Client) (string, error) {
		return "v1.2.3", nil
	}
	runUpdateProvidersFn = func(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
		_, _ = io.WriteString(out, "updated providers: changed=0 unchanged=1\n")
		return updateResult{applyHint: false}, nil
	}

	var out bytes.Buffer
	if err := runUpdateTarget(context.Background(), "providers", updateOptions{cfgPath: "onr.yaml"}, defaultUpdateDeps(), &out); err != nil {
		t.Fatalf("runUpdateTarget: %v", err)
	}
	if strings.Contains(out.String(), "hint: apply runtime changes") {
		t.Fatalf("output should not contain hint: %q", out.String())
	}
}

func TestBackupFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	bp, err := backupFile(src, func() time.Time { return time.Date(2026, 2, 26, 1, 2, 3, 0, time.UTC) })
	if err != nil {
		t.Fatalf("backupFile: %v", err)
	}
	if !strings.HasSuffix(bp, ".bak.20260226-010203") {
		t.Fatalf("backup path=%q", bp)
	}
	b, err := os.ReadFile(bp)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("backup content=%q", string(b))
	}
}

func TestWriteProviderFileIfChanged_Unchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dst := filepath.Join(dir, "openai.conf")
	content := []byte(`syntax "next-router/0.1";`)
	if err := os.WriteFile(dst, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	changed, err := writeProviderFileIfChanged(dst, content, true, func() time.Time {
		return time.Date(2026, 2, 26, 1, 2, 3, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("writeProviderFileIfChanged: %v", err)
	}
	if changed {
		t.Fatalf("changed should be false")
	}
	backups, err := filepath.Glob(dst + ".bak.*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("unexpected backups: %v", backups)
	}
}

func TestWriteProviderFileIfChanged_ChangedWithBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dst := filepath.Join(dir, "openai.conf")
	oldBody := []byte(`syntax "next-router/0.1";`)
	newBody := []byte(`syntax "next-router/0.2";`)
	if err := os.WriteFile(dst, oldBody, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	changed, err := writeProviderFileIfChanged(dst, newBody, true, func() time.Time {
		return time.Date(2026, 2, 26, 4, 5, 6, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("writeProviderFileIfChanged: %v", err)
	}
	if !changed {
		t.Fatalf("changed should be true")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(newBody) {
		t.Fatalf("dst=%q want=%q", string(got), string(newBody))
	}
	backupPath := dst + ".bak.20260226-040506"
	b, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(b) != string(oldBody) {
		t.Fatalf("backup=%q want=%q", string(b), string(oldBody))
	}
}

func writeTestTarGz(dst string, files map[string]string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for name, body := range files {
		h := &tar.Header{
			Name:    name,
			Mode:    0o600,
			Size:    int64(len(body)),
			ModTime: time.Unix(0, 0),
		}
		if err := tw.WriteHeader(h); err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return err
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = gw.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return f.Close()
}
