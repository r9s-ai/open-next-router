package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-admin/internal/providersource"
	"github.com/r9s-ai/open-next-router/onr-admin/internal/store"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/config"
	"github.com/spf13/cobra"
)

const (
	defaultUpdateRepo = "r9s-ai/open-next-router"
)

type updateOptions struct {
	version string
	repo    string

	cfgPath      string
	providersDir string

	onrBinPath      string
	onrAdminBinPath string

	backup bool
}

type updateDeps struct {
	httpClient *http.Client
	lookPath   func(file string) (string, error)
	now        func() time.Time
	goos       string
	goarch     string
}

type updateResult struct {
	applyHint bool
}

var (
	resolveUpdateReleaseTagFn = resolveUpdateReleaseTag
	runUpdateOnrFn            = runUpdateOnr
	runUpdateOnrAdminFn       = runUpdateOnrAdmin
	runUpdateProvidersFn      = runUpdateProviders
)

func defaultUpdateDeps() updateDeps {
	return updateDeps{
		httpClient: &http.Client{Timeout: 45 * time.Second},
		lookPath:   exec.LookPath,
		now:        time.Now,
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
	}
}

func newUpdateCmd() *cobra.Command {
	opts := updateOptions{
		repo:    defaultUpdateRepo,
		cfgPath: "onr.yaml",
		backup:  true,
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update onr/onr-admin/providers from release assets",
	}
	setUpdateFlagErrorFunc(cmd)

	fs := cmd.PersistentFlags()
	fs.StringVar(&opts.version, "version", "", "target runtime version, e.g. v1.2.3 (default latest stable)")
	fs.StringVar(&opts.repo, "repo", defaultUpdateRepo, "github repo slug in owner/name format")
	fs.StringVar(&opts.cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.providersDir, "providers-dir", "", "providers dir path")
	fs.StringVar(&opts.onrBinPath, "onr-bin", "", "path to onr binary")
	fs.StringVar(&opts.onrAdminBinPath, "onr-admin-bin", "", "path to onr-admin binary")
	fs.BoolVar(&opts.backup, "backup", true, "backup existing files before overwrite")

	cmd.AddCommand(
		newUpdateTargetCmd("onr", &opts),
		newUpdateTargetCmd("onr-admin", &opts),
		newUpdateTargetCmd("providers", &opts),
		newUpdateTargetCmd("all", &opts),
	)
	return cmd
}

func newUpdateTargetCmd(target string, opts *updateOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   target,
		Short: "Update " + target,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateTarget(cmd.Context(), target, *opts, defaultUpdateDeps(), cmd.OutOrStdout())
		},
	}
	setUpdateFlagErrorFunc(cmd)
	return cmd
}

func setUpdateFlagErrorFunc(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "unknown flag: --all") {
			return errors.New("flag --all is not supported; use `onr-admin update all`")
		}
		return err
	})
}

func runUpdateTarget(ctx context.Context, target string, opts updateOptions, deps updateDeps, out io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("missing update target")
	}

	releaseTag, err := resolveUpdateReleaseTagFn(ctx, opts.version, opts.repo, deps.httpClient)
	if err != nil {
		return err
	}

	switch target {
	case "onr", "onr-admin", "providers":
		res, err := runUpdateSingle(ctx, target, releaseTag, opts, deps, out)
		if err != nil {
			return err
		}
		if res.applyHint {
			printUpdateApplyHint(out, opts.cfgPath)
		}
		return nil
	case "all":
		return runUpdateAll(ctx, releaseTag, opts, deps, out)
	default:
		return fmt.Errorf("unknown update target %q", target)
	}
}

func runUpdateAll(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) error {
	targets := []string{"onr", "providers", "onr-admin"}
	success := 0
	failed := 0
	needHint := false

	for _, target := range targets {
		res, err := runUpdateSingle(ctx, target, releaseTag, opts, deps, out)
		if err != nil {
			failed++
			_, _ = fmt.Fprintf(out, "update target=%s status=failed err=%q\n", target, err.Error())
			continue
		}
		success++
		_, _ = fmt.Fprintf(out, "update target=%s status=ok\n", target)
		if res.applyHint {
			needHint = true
		}
	}

	_, _ = fmt.Fprintf(out, "update all summary: total=%d success=%d failed=%d\n", len(targets), success, failed)
	if needHint {
		printUpdateApplyHint(out, opts.cfgPath)
	}
	if failed > 0 {
		return fmt.Errorf("update all completed with %d failure(s)", failed)
	}
	return nil
}

func runUpdateSingle(ctx context.Context, target string, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
	switch target {
	case "onr":
		return runUpdateOnrFn(ctx, releaseTag, opts, deps, out)
	case "onr-admin":
		return runUpdateOnrAdminFn(ctx, releaseTag, opts, deps, out)
	case "providers":
		return runUpdateProvidersFn(ctx, releaseTag, opts, deps, out)
	default:
		return updateResult{}, fmt.Errorf("unsupported update target %q", target)
	}
}

func runUpdateOnr(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
	binPath, err := resolveBinaryTargetPath(strings.TrimSpace(opts.onrBinPath), "onr", deps.lookPath)
	if err != nil {
		return updateResult{}, err
	}
	assetName, err := runtimeBinaryAssetName(releaseTag, deps.goos, deps.goarch)
	if err != nil {
		return updateResult{}, err
	}
	archivePath, cleanup, err := downloadReleaseAssetWithChecksum(ctx, deps.httpClient, strings.TrimSpace(opts.repo), releaseTag, assetName)
	if err != nil {
		return updateResult{}, err
	}
	defer cleanup()

	binBody, err := extractBinaryFromArchive(archivePath, "onr", deps.goos)
	if err != nil {
		return updateResult{}, err
	}
	changed, err := writeBinaryIfChanged(binPath, binBody, deps.now)
	if err != nil {
		return updateResult{}, err
	}
	_, _ = fmt.Fprintf(out, "updated onr: version=%s path=%s changed=%t\n", releaseTag, binPath, changed)
	return updateResult{applyHint: changed}, nil
}

func runUpdateOnrAdmin(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
	if deps.goos == "windows" {
		return updateResult{}, errors.New("updating onr-admin on windows is not supported; please download and replace manually")
	}
	binPath, err := resolveBinaryTargetPath(strings.TrimSpace(opts.onrAdminBinPath), "onr-admin", deps.lookPath)
	if err != nil {
		return updateResult{}, err
	}
	assetName, err := runtimeBinaryAssetName(releaseTag, deps.goos, deps.goarch)
	if err != nil {
		return updateResult{}, err
	}
	archivePath, cleanup, err := downloadReleaseAssetWithChecksum(ctx, deps.httpClient, strings.TrimSpace(opts.repo), releaseTag, assetName)
	if err != nil {
		return updateResult{}, err
	}
	defer cleanup()

	binBody, err := extractBinaryFromArchive(archivePath, "onr-admin", deps.goos)
	if err != nil {
		return updateResult{}, err
	}
	changed, err := writeBinaryIfChanged(binPath, binBody, deps.now)
	if err != nil {
		return updateResult{}, err
	}
	_, _ = fmt.Fprintf(out, "updated onr-admin: version=%s path=%s changed=%t\n", releaseTag, binPath, changed)
	return updateResult{}, nil
}

func runUpdateProviders(ctx context.Context, releaseTag string, opts updateOptions, deps updateDeps, out io.Writer) (updateResult, error) {
	source, err := resolveUpdateProvidersSource(opts)
	if err != nil {
		return updateResult{}, err
	}
	assetName := providersConfigAssetName(releaseTag)
	archivePath, cleanup, err := downloadReleaseAssetWithChecksum(ctx, deps.httpClient, strings.TrimSpace(opts.repo), releaseTag, assetName)
	if err != nil {
		return updateResult{}, err
	}
	defer cleanup()

	files, err := extractProviderFilesFromConfigArchive(archivePath)
	if err != nil {
		return updateResult{}, err
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var changed, unchanged int
	if source.EditableIsFile {
		changed, unchanged, err = updateMergedProviderSource(source, names, files, opts.backup, deps.now)
		if err != nil {
			return updateResult{}, err
		}
	} else {
		changed, unchanged, err = updateProviderDirectorySource(source, names, files, opts.backup, deps.now)
		if err != nil {
			return updateResult{}, err
		}
	}
	if _, err := dslconfig.ValidateProvidersPath(source.SourcePath); err != nil {
		return updateResult{}, fmt.Errorf("validate provider source %s failed after update: %w", source.SourcePath, err)
	}
	_, _ = fmt.Fprintf(out, "updated providers: version=%s source=%s editable=%s files=%d changed=%d unchanged=%d\n", releaseTag, source.SourcePath, source.EditablePath, len(names), changed, unchanged)
	return updateResult{applyHint: changed > 0}, nil
}

func updateProviderDirectorySource(source providersource.Info, names []string, files map[string][]byte, backup bool, nowFn func() time.Time) (int, int, error) {
	if err := os.MkdirAll(source.EditablePath, 0o750); err != nil {
		return 0, 0, err
	}
	changed := 0
	unchanged := 0
	for _, name := range names {
		dst := filepath.Join(source.EditablePath, name)
		didChange, err := writeProviderFileIfChanged(dst, files[name], backup, nowFn)
		if err != nil {
			return 0, 0, err
		}
		if didChange {
			changed++
		} else {
			unchanged++
		}
	}
	return changed, unchanged, nil
}

func updateMergedProviderSource(source providersource.Info, names []string, files map[string][]byte, backup bool, nowFn func() time.Time) (int, int, error) {
	// #nosec G304 -- editable provider source path is configured by the user.
	currentBody, err := os.ReadFile(source.EditablePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, 0, err
	}
	updated := string(currentBody)
	changed := 0
	unchanged := 0
	for _, name := range names {
		block, err := dslconfig.ProviderBlockFromFileContent(name, string(files[name]))
		if err != nil {
			return 0, 0, err
		}
		existing, ok, err := dslconfig.ExtractProviderBlockOptional(source.EditablePath, updated, block.Name)
		if err != nil {
			return 0, 0, err
		}
		if ok && strings.TrimSpace(existing) == strings.TrimSpace(block.Content) {
			unchanged++
			continue
		}
		updated, err = dslconfig.UpsertProviderBlock(source.EditablePath, updated, block.Name, block.Content)
		if err != nil {
			return 0, 0, err
		}
		changed++
	}
	if changed == 0 {
		return 0, unchanged, nil
	}
	if err := validateMergedProviderSourceCandidate(source, updated); err != nil {
		return 0, 0, err
	}
	if backup {
		if _, err := backupFile(source.EditablePath, nowFn); err != nil && !errors.Is(err, os.ErrNotExist) {
			return 0, 0, err
		}
	}
	if err := store.WriteAtomic(source.EditablePath, []byte(updated), false); err != nil {
		return 0, 0, err
	}
	return changed, unchanged, nil
}

func validateMergedProviderSourceCandidate(source providersource.Info, content string) error {
	configRoot := filepath.Dir(source.SourcePath)
	tmpRoot, err := os.MkdirTemp("", "onr-admin-update-providers-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	if err := copyDirectory(configRoot, tmpRoot); err != nil {
		return err
	}
	tmpSourcePath := filepath.Join(tmpRoot, filepath.Base(source.SourcePath))
	if err := os.WriteFile(tmpSourcePath, []byte(content), 0o600); err != nil {
		return err
	}
	_, err = dslconfig.ValidateProvidersPath(tmpSourcePath)
	return err
}

func copyDirectory(src, dst string) error {
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("providers dir %q is not directory", src)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not supported: %s", path)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		// #nosec G304 -- path is discovered by filepath.WalkDir under trusted config root.
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o600)
		if fi, err := d.Info(); err == nil {
			if p := fi.Mode().Perm(); p != 0 {
				mode = p
			}
		}
		return os.WriteFile(target, b, mode)
	})
}

func writeProviderFileIfChanged(dst string, content []byte, backup bool, nowFn func() time.Time) (bool, error) {
	existing, err := os.ReadFile(dst)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err == nil {
		if bytes.Equal(existing, content) {
			return false, nil
		}
		if backup {
			if _, berr := backupFile(dst, nowFn); berr != nil {
				return false, berr
			}
		}
	}
	if werr := store.WriteAtomic(dst, content, false); werr != nil {
		return false, werr
	}
	return true, nil
}

func printUpdateApplyHint(out io.Writer, cfgPath string) {
	cfg := strings.TrimSpace(cfgPath)
	if cfg == "" {
		cfg = "onr.yaml"
	}
	_, _ = fmt.Fprintf(out, "hint: apply runtime changes with: onr -s reload -c %s\n", cfg)
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func resolveUpdateReleaseTag(ctx context.Context, version string, repo string, client *http.Client) (string, error) {
	if client == nil {
		return "", errors.New("nil http client")
	}
	normalizedRepo := strings.TrimSpace(repo)
	if normalizedRepo == "" {
		normalizedRepo = defaultUpdateRepo
	}
	normalizedVersion := strings.TrimSpace(version)
	if normalizedVersion != "" {
		tag, err := normalizeRuntimeTag(normalizedVersion)
		if err != nil {
			return "", err
		}
		return tag, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100", normalizedRepo)
	body, err := fetchURL(ctx, client, url)
	if err != nil {
		return "", err
	}
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("decode releases response failed: %w", err)
	}
	for _, item := range releases {
		tag := strings.TrimSpace(item.TagName)
		if item.Draft || item.Prerelease {
			continue
		}
		if isRuntimeTag(tag) {
			return tag, nil
		}
	}
	return "", errors.New("no stable runtime release tag found")
}

func normalizeRuntimeTag(version string) (string, error) {
	v := strings.TrimSpace(version)
	if v == "" {
		return "", errors.New("empty version")
	}
	if strings.Contains(v, "/") {
		return "", fmt.Errorf("invalid runtime version %q: runtime tags must not contain '/'", v)
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !isRuntimeTag(v) {
		return "", fmt.Errorf("invalid runtime version %q", version)
	}
	return v, nil
}

func isRuntimeTag(tag string) bool {
	tag = strings.TrimSpace(tag)
	if len(tag) < 2 || !strings.HasPrefix(tag, "v") {
		return false
	}
	if strings.Contains(tag, "/") {
		return false
	}
	return true
}

func runtimeBinaryAssetName(releaseTag string, goos string, goarch string) (string, error) {
	tag := strings.TrimSpace(releaseTag)
	if !isRuntimeTag(tag) {
		return "", fmt.Errorf("invalid release tag %q", releaseTag)
	}
	releasePlain := strings.TrimPrefix(tag, "v")
	archPart, err := runtimeArchiveArch(goarch)
	if err != nil {
		return "", err
	}
	osPart := strings.TrimSpace(goos)
	if osPart != "linux" && osPart != "darwin" && osPart != "windows" {
		return "", fmt.Errorf("unsupported os %q", osPart)
	}
	ext := "tar.gz"
	if osPart == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("open-next-router_%s_%s_%s.%s", releasePlain, osPart, archPart, ext), nil
}

func runtimeArchiveArch(goarch string) (string, error) {
	switch strings.TrimSpace(goarch) {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch %q", goarch)
	}
}

func providersConfigAssetName(releaseTag string) string {
	return fmt.Sprintf("open-next-router_config_%s.tar.gz", strings.TrimSpace(releaseTag))
}

func resolveBinaryTargetPath(flagPath string, binaryName string, lookPath func(file string) (string, error)) (string, error) {
	p := strings.TrimSpace(flagPath)
	if p != "" {
		return p, nil
	}
	if lookPath == nil {
		return "", fmt.Errorf("cannot resolve %s path: lookPath is nil", binaryName)
	}
	resolved, err := lookPath(binaryName)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %s path: %w", binaryName, err)
	}
	return resolved, nil
}

func resolveUpdateProvidersSource(opts updateOptions) (providersource.Info, error) {
	override := strings.TrimSpace(opts.providersDir)
	var cfg *config.Config
	if override != "" {
		return providersource.Resolve(override)
	}
	cfgPath := strings.TrimSpace(opts.cfgPath)
	if cfgPath != "" {
		loaded, err := store.LoadConfigIfExists(cfgPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return providersource.Info{}, fmt.Errorf("load config %s failed: %w", cfgPath, err)
			}
		} else {
			cfg = loaded
		}
	}
	return providersource.Resolve(resolveProviderSourcePath(cfg, ""))
}

func fetchURL(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "onr-admin-update")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("request failed: url=%s status=%s body=%q", url, resp.Status, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func downloadFile(ctx context.Context, client *http.Client, url string, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "onr-admin-update")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download failed: url=%s status=%s body=%q", url, resp.Status, strings.TrimSpace(string(body)))
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return f.Close()
}

func releaseAssetURL(repo string, releaseTag string, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", strings.TrimSpace(repo), strings.TrimSpace(releaseTag), strings.TrimSpace(asset))
}

func downloadReleaseAssetWithChecksum(ctx context.Context, client *http.Client, repo string, releaseTag string, assetName string) (string, func(), error) {
	normalizedRepo := strings.TrimSpace(repo)
	if normalizedRepo == "" {
		normalizedRepo = defaultUpdateRepo
	}
	tmpDir, err := os.MkdirTemp("", "onr-admin-update-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	assetPath := filepath.Join(tmpDir, assetName)
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	if err := downloadFile(ctx, client, releaseAssetURL(normalizedRepo, releaseTag, assetName), assetPath); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := downloadFile(ctx, client, releaseAssetURL(normalizedRepo, releaseTag, "checksums.txt"), checksumsPath); err != nil {
		cleanup()
		return "", nil, err
	}
	sumsBody, err := os.ReadFile(checksumsPath)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	expected, err := findChecksumEntry(string(sumsBody), assetName)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if err := verifyFileSHA256(assetPath, expected); err != nil {
		cleanup()
		return "", nil, err
	}
	return assetPath, cleanup, nil
}

func findChecksumEntry(checksumsText string, assetName string) (string, error) {
	for _, line := range strings.Split(checksumsText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		gotAsset := path.Base(strings.TrimPrefix(strings.TrimSpace(fields[1]), "*"))
		if gotAsset == assetName {
			return strings.TrimSpace(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksum entry not found for %s", assetName)
}

func verifyFileSHA256(path string, expected string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(expected)) {
		return fmt.Errorf("checksum mismatch for %s", path)
	}
	return nil
}

func extractBinaryFromArchive(archivePath string, binaryName string, goos string) ([]byte, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractBinaryFromZip(archivePath, binaryName, goos)
	}
	return extractBinaryFromTarGz(archivePath, binaryName, goos)
}

func extractBinaryFromTarGz(archivePath string, binaryName string, goos string) ([]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()
	tr := tar.NewReader(gr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if !isArchiveBinaryName(path.Base(h.Name), binaryName, goos) {
			continue
		}
		return io.ReadAll(tr)
	}
	return nil, fmt.Errorf("binary %q not found in archive %s", binaryName, archivePath)
}

func extractBinaryFromZip(archivePath string, binaryName string, goos string) ([]byte, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !isArchiveBinaryName(path.Base(f.Name), binaryName, goos) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("binary %q not found in archive %s", binaryName, archivePath)
}

func isArchiveBinaryName(baseName string, binaryName string, goos string) bool {
	name := strings.TrimSpace(baseName)
	want := strings.TrimSpace(binaryName)
	if name == want {
		return true
	}
	return goos == "windows" && name == want+".exe"
}

func writeBinaryAtomic(dstPath string, body []byte, now time.Time) error {
	target := strings.TrimSpace(dstPath)
	if target == "" {
		return errors.New("empty binary path")
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o755)
	if st, err := os.Stat(target); err == nil {
		mode = st.Mode().Perm()
	}
	tmpName := fmt.Sprintf(".%s.tmp.%d", filepath.Base(target), now.UnixNano())
	tmpPath := filepath.Join(dir, tmpName)
	if err := os.WriteFile(tmpPath, body, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func writeBinaryIfChanged(dstPath string, body []byte, nowFn func() time.Time) (bool, error) {
	target := strings.TrimSpace(dstPath)
	if target == "" {
		return false, errors.New("empty binary path")
	}
	existing, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err == nil && bytes.Equal(existing, body) {
		return false, nil
	}
	if werr := writeBinaryAtomic(target, body, nowFn()); werr != nil {
		return false, werr
	}
	return true, nil
}

func extractProviderFilesFromConfigArchive(archivePath string) (map[string][]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()
	tr := tar.NewReader(gr)
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(strings.TrimSpace(h.Name))
		if strings.Contains(name, "..") {
			continue
		}
		if !strings.HasPrefix(name, "config/providers/") || !strings.HasSuffix(name, ".conf") {
			continue
		}
		base := path.Base(name)
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		out[base] = b
	}
	if len(out) == 0 {
		return nil, errors.New("no provider .conf files found in config archive")
	}
	return out, nil
}

func backupFile(src string, nowFn func() time.Time) (string, error) {
	target := strings.TrimSpace(src)
	if target == "" {
		return "", errors.New("empty backup source path")
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	ts := nowFn().Format("20060102-150405")
	backupPath := target + ".bak." + ts
	if err := os.WriteFile(backupPath, b, 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}
