package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/r9s-ai/open-next-router/onr-admin/internal/providersource"
	"github.com/r9s-ai/open-next-router/onr-admin/internal/store"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	cfgpkg "github.com/r9s-ai/open-next-router/pkg/config"
)

const (
	defaultDumpsDir    = "./dumps"
	defaultListen      = "0.0.0.0:3310"
	defaultAPIBaseURL  = "http://127.0.0.1:3300"
	envAPIBaseURL      = "ONR_ADMIN_WEB_CURL_API_BASE_URL"
	envListen          = "ONR_ADMIN_WEB_LISTEN"
	dumpBodyLimitBytes = 2 * 1024 * 1024
)

var providerNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type Options struct {
	ConfigPath   string
	ProvidersDir string
	Listen       string
}

type Server struct {
	providerSource providersource.Info
	dumpsDir       string
	indexHTML      string
	mu             sync.Mutex
}

type providerRequest struct {
	Provider string `json:"provider"`
	Content  string `json:"content"`
}

type providerResponse struct {
	OK              bool     `json:"ok"`
	Provider        string   `json:"provider,omitempty"`
	TargetFile      string   `json:"target_file,omitempty"`
	Content         string   `json:"content,omitempty"`
	LoadedProviders []string `json:"loaded_providers,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Providers       []string `json:"providers,omitempty"`
	Error           string   `json:"error,omitempty"`
}

type testRequest struct {
	BaseURL       string `json:"base_url"`
	Path          string `json:"path"`
	Authorization string `json:"authorization"`
	Provider      string `json:"provider"`
	Payload       string `json:"payload"`
}

type testResponse struct {
	OK      bool              `json:"ok"`
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type dumpByRequestIDResponse struct {
	OK        bool   `json:"ok"`
	RequestID string `json:"request_id,omitempty"`
	Path      string `json:"path,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	Content   string `json:"content,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Error     string `json:"error,omitempty"`
}

func Run(opts Options) error {
	providersPath := resolveProviderSourcePath(opts.ConfigPath, opts.ProvidersDir)
	dumpsDir := resolveDumpsDir(opts.ConfigPath)
	defaultBaseURL := resolveDefaultAPIBaseURL()
	srv, err := newServer(providersPath, dumpsDir, defaultBaseURL)
	if err != nil {
		return err
	}
	listen := resolveListenAddress(opts.Listen)
	log.Printf(
		"onr-admin web listening: url=%q providers_source=%q providers_edit_path=%q dumps_dir=%q default_curl_api_base_url=%q",
		"http://"+listen,
		srv.providerSource.SourcePath,
		srv.providerSource.EditablePath,
		dumpsDir,
		defaultBaseURL,
	)
	return http.ListenAndServe(listen, srv.Handler())
}

func NewServer(providersDir string) (*Server, error) {
	return newServer(providersDir, defaultDumpsDir, defaultAPIBaseURL)
}

func newServer(providersDir string, dumpsDir string, defaultBaseURL string) (*Server, error) {
	sourcePath := strings.TrimSpace(providersDir)
	if sourcePath == "" {
		return nil, errors.New("providers source is empty")
	}
	sourceInfo, err := providersource.Resolve(sourcePath)
	if err != nil {
		return nil, err
	}
	dumpDir := strings.TrimSpace(dumpsDir)
	if dumpDir == "" {
		dumpDir = defaultDumpsDir
	}
	return &Server{
		providerSource: sourceInfo,
		dumpsDir:       dumpDir,
		indexHTML:      renderIndexHTML(defaultBaseURL),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/providers", s.handleProviders)
	mux.HandleFunc("/api/provider", s.handleProvider)
	mux.HandleFunc("/api/providers/validate", s.handleValidate)
	mux.HandleFunc("/api/providers/save", s.handleSave)
	mux.HandleFunc("/api/test/request", s.handleTestRequest)
	mux.HandleFunc("/api/dumps/by-request-id", s.handleDumpByRequestID)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, s.indexHTML)
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	names, err := listProviders(s.providerSource)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, providerResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providerResponse{
		OK:        true,
		Providers: names,
	})
}

func (s *Server) handleProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	provider, err := normalizeProviderName(r.URL.Query().Get("name"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, providerResponse{OK: false, Error: err.Error()})
		return
	}
	target, b, err := readProviderContent(s.providerSource, provider)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, providerResponse{OK: false, Error: "provider file not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, providerResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providerResponse{
		OK:         true,
		Provider:   provider,
		TargetFile: target,
		Content:    string(b),
	})
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	in, err := decodeProviderRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, providerResponse{OK: false, Error: err.Error()})
		return
	}
	res, target, err := s.validateCandidate(in.Provider, in.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, providerResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providerResponse{
		OK:              true,
		Provider:        in.Provider,
		TargetFile:      target,
		LoadedProviders: res.LoadedProviders,
		Warnings:        formatWarnings(res.Warnings),
	})
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	in, err := decodeProviderRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, providerResponse{OK: false, Error: err.Error()})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	res, target, err := s.validateCandidate(in.Provider, in.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, providerResponse{OK: false, Error: err.Error()})
		return
	}
	if err := writeProviderContent(s.providerSource, in.Provider, in.Content); err != nil {
		writeJSON(w, http.StatusInternalServerError, providerResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providerResponse{
		OK:              true,
		Provider:        in.Provider,
		TargetFile:      target,
		LoadedProviders: res.LoadedProviders,
		Warnings:        formatWarnings(res.Warnings),
	})
}

func (s *Server) handleTestRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	in, err := decodeTestRequest(r)
	if err != nil {
		writeJSONAny(w, http.StatusBadRequest, testResponse{OK: false, Error: err.Error()})
		return
	}
	ctx := r.Context()
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.BaseURL+in.Path, strings.NewReader(in.Payload))
	if err != nil {
		writeJSONAny(w, http.StatusBadRequest, testResponse{OK: false, Error: err.Error()})
		return
	}
	req.Header.Set("Authorization", in.Authorization)
	req.Header.Set("Content-Type", "application/json")
	if in.Provider != "" {
		req.Header.Set("x-onr-provider", in.Provider)
	}

	resp, err := client.Do(req)
	if err != nil {
		writeJSONAny(w, http.StatusBadGateway, testResponse{OK: false, Error: err.Error()})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, truncated, readErr := readBodyLimit(resp.Body, 2*1024*1024)
	if readErr != nil {
		writeJSONAny(w, http.StatusBadGateway, testResponse{OK: false, Error: readErr.Error()})
		return
	}
	outBody := string(bodyBytes)
	if truncated {
		outBody += "\n...[truncated]"
	}
	headers := map[string]string{}
	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		headers[k] = strings.Join(v, ", ")
	}
	writeJSONAny(w, http.StatusOK, testResponse{
		OK:      true,
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    outBody,
	})
}

func (s *Server) handleDumpByRequestID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	rid := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if rid == "" {
		writeJSONAny(w, http.StatusBadRequest, dumpByRequestIDResponse{OK: false, Error: "request_id is empty"})
		return
	}
	sum, found, err := store.FindDumpByRequestID(store.DumpFindOptions{
		Dir:       s.dumpsDir,
		RequestID: rid,
	})
	if err != nil {
		writeJSONAny(w, http.StatusInternalServerError, dumpByRequestIDResponse{
			OK:        false,
			RequestID: rid,
			Error:     err.Error(),
		})
		return
	}
	if !found {
		writeJSONAny(w, http.StatusNotFound, dumpByRequestIDResponse{
			OK:        false,
			RequestID: rid,
			Error:     "dump file not found",
		})
		return
	}

	path := strings.TrimSpace(sum.Path)
	if path == "" {
		writeJSONAny(w, http.StatusInternalServerError, dumpByRequestIDResponse{
			OK:        false,
			RequestID: rid,
			Error:     "dump file path is empty",
		})
		return
	}
	// #nosec G304 -- path is selected from scanned dump directory.
	f, err := os.Open(path)
	if err != nil {
		writeJSONAny(w, http.StatusInternalServerError, dumpByRequestIDResponse{
			OK:        false,
			RequestID: rid,
			Path:      path,
			Error:     err.Error(),
		})
		return
	}
	defer func() { _ = f.Close() }()

	bodyBytes, truncated, err := readBodyLimit(f, dumpBodyLimitBytes)
	if err != nil {
		writeJSONAny(w, http.StatusInternalServerError, dumpByRequestIDResponse{
			OK:        false,
			RequestID: rid,
			Path:      path,
			Error:     err.Error(),
		})
		return
	}

	content := string(bodyBytes)
	if truncated {
		content += "\n...[truncated]"
	}
	fileName := strings.TrimSpace(sum.FileName)
	if fileName == "" {
		fileName = filepath.Base(path)
	}
	requestID := strings.TrimSpace(sum.RequestID)
	if requestID == "" {
		requestID = rid
	}
	writeJSONAny(w, http.StatusOK, dumpByRequestIDResponse{
		OK:        true,
		RequestID: requestID,
		Path:      path,
		FileName:  fileName,
		Content:   content,
		Truncated: truncated,
	})
}

func (s *Server) validateCandidate(provider string, content string) (dslconfig.LoadResult, string, error) {
	name, err := normalizeProviderName(provider)
	if err != nil {
		return dslconfig.LoadResult{}, "", err
	}
	target, err := providersource.ResolveProviderTarget(s.providerSource, name)
	if err != nil {
		return dslconfig.LoadResult{}, "", err
	}

	tmpRoot, err := os.MkdirTemp("", "onr-admin-web-providers-*")
	if err != nil {
		return dslconfig.LoadResult{}, "", err
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	configRoot := sourceConfigRoot(s.providerSource)
	if err := copyDirectory(configRoot, tmpRoot); err != nil {
		return dslconfig.LoadResult{}, "", err
	}
	tmpSourcePath, err := mapSourcePathUnderTemp(tmpRoot, configRoot, s.providerSource.SourcePath)
	if err != nil {
		return dslconfig.LoadResult{}, "", err
	}
	tmpTargetPath, err := mapSourcePathUnderTemp(tmpRoot, configRoot, target.Path)
	if err != nil {
		return dslconfig.LoadResult{}, "", err
	}
	if s.providerSource.SourceIsFile && target.Path == s.providerSource.SourcePath {
		// #nosec G304 -- temp source path is derived from a trusted temp root and source layout.
		body, err := os.ReadFile(tmpSourcePath)
		if err != nil {
			return dslconfig.LoadResult{}, "", err
		}
		updated, err := dslconfig.UpsertProviderBlock(tmpSourcePath, string(body), name, content)
		if err != nil {
			return dslconfig.LoadResult{}, "", err
		}
		if err := os.WriteFile(tmpSourcePath, []byte(updated), 0o600); err != nil {
			return dslconfig.LoadResult{}, "", err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(tmpTargetPath), 0o750); err != nil {
			return dslconfig.LoadResult{}, "", err
		}
		if err := os.WriteFile(tmpTargetPath, []byte(content), 0o600); err != nil {
			return dslconfig.LoadResult{}, "", err
		}
	}
	res, err := dslconfig.ValidateProvidersPath(tmpSourcePath)
	if err != nil {
		return dslconfig.LoadResult{}, "", err
	}
	return res, target.Path, nil
}

func normalizeProviderName(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return "", errors.New("provider is empty")
	}
	if !providerNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid provider name %q", raw)
	}
	return name, nil
}

func decodeProviderRequest(r *http.Request) (providerRequest, error) {
	if r == nil || r.Body == nil {
		return providerRequest{}, errors.New("empty request body")
	}
	defer func() { _ = r.Body.Close() }()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var in providerRequest
	if err := dec.Decode(&in); err != nil {
		return providerRequest{}, err
	}
	name, err := normalizeProviderName(in.Provider)
	if err != nil {
		return providerRequest{}, err
	}
	in.Provider = name
	if strings.TrimSpace(in.Content) == "" {
		return providerRequest{}, errors.New("content is empty")
	}
	return in, nil
}

func formatWarnings(warnings []dslconfig.ValidationWarning) []string {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]string, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, w.String())
	}
	return out
}

func decodeTestRequest(r *http.Request) (testRequest, error) {
	if r == nil || r.Body == nil {
		return testRequest{}, errors.New("empty request body")
	}
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var in testRequest
	if err := dec.Decode(&in); err != nil {
		return testRequest{}, err
	}
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.Path = strings.TrimSpace(in.Path)
	in.Authorization = strings.TrimSpace(in.Authorization)
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Payload = strings.TrimSpace(in.Payload)
	if in.BaseURL == "" {
		return testRequest{}, errors.New("base_url is empty")
	}
	u, err := url.Parse(in.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return testRequest{}, errors.New("base_url is invalid")
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return testRequest{}, errors.New("base_url scheme must be http or https")
	}
	if in.Path == "" || !strings.HasPrefix(in.Path, "/") {
		return testRequest{}, errors.New("path must start with /")
	}
	if in.Authorization == "" {
		return testRequest{}, errors.New("authorization is empty")
	}
	if in.Payload == "" {
		return testRequest{}, errors.New("payload is empty")
	}
	var tmp any
	if err := json.Unmarshal([]byte(in.Payload), &tmp); err != nil {
		return testRequest{}, errors.New("payload must be valid json")
	}
	return in, nil
}

func writeJSON(w http.ResponseWriter, status int, body providerResponse) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(body)
}

func writeJSONAny(w http.ResponseWriter, status int, body any) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(body)
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	if w == nil {
		return
	}
	if strings.TrimSpace(allowed) != "" {
		w.Header().Set("Allow", allowed)
	}
	writeJSON(w, http.StatusMethodNotAllowed, providerResponse{OK: false, Error: "method not allowed"})
}

func resolveProviderSourcePath(cfgPath, override string) string {
	if path := strings.TrimSpace(override); path != "" {
		return path
	}
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	path, _ := cfgpkg.ResolveProviderDSLSource(cfg)
	return path
}

func resolveDumpsDir(cfgPath string) string {
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	if cfg != nil && strings.TrimSpace(cfg.TrafficDump.Dir) != "" {
		return strings.TrimSpace(cfg.TrafficDump.Dir)
	}
	return defaultDumpsDir
}

func listProviders(source providersource.Info) ([]string, error) {
	if strings.TrimSpace(source.EditablePath) == "" {
		if _, err := os.Stat(source.SourcePath); err != nil {
			if os.IsNotExist(err) {
				return []string{}, nil
			}
			return nil, err
		}
		res, err := dslconfig.ValidateProvidersPath(source.SourcePath)
		if err != nil {
			return nil, err
		}
		out := append([]string(nil), res.LoadedProviders...)
		sort.Strings(out)
		return out, nil
	}
	if source.EditableIsFile {
		// #nosec G304 -- editable provider source path is configured by the user.
		b, err := os.ReadFile(source.EditablePath)
		if err != nil {
			if os.IsNotExist(err) {
				return []string{}, nil
			}
			return nil, err
		}
		blocks, err := dslconfig.ListProviderBlocks(source.EditablePath, string(b))
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(blocks))
		for _, block := range blocks {
			out = append(out, block.Name)
		}
		sort.Strings(out)
		return out, nil
	}
	return listProvidersInDir(source.EditablePath)
}

func listProvidersInDir(providersDir string) ([]string, error) {
	entries, err := os.ReadDir(providersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if filepath.Ext(name) != ".conf" {
			continue
		}
		base := strings.TrimSuffix(name, ".conf")
		if base == "" {
			continue
		}
		path := filepath.Join(providersDir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if _, ok, err := dslconfig.FindProviderNameOptional(path, string(b)); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		out = append(out, strings.ToLower(base))
	}
	sort.Strings(out)
	return out, nil
}

func readProviderContent(source providersource.Info, provider string) (string, []byte, error) {
	target, err := providersource.ResolveProviderTarget(source, provider)
	if err != nil {
		return "", nil, err
	}
	if source.SourceIsFile && target.Path == source.SourcePath {
		// #nosec G304 -- source path is configured by the user.
		b, err := os.ReadFile(source.SourcePath)
		if err != nil {
			return "", nil, err
		}
		block, ok, err := dslconfig.ExtractProviderBlockOptional(source.SourcePath, string(b), provider)
		if err != nil {
			return "", nil, err
		}
		if !ok {
			return "", nil, os.ErrNotExist
		}
		return source.SourcePath, []byte(block), nil
	}
	// #nosec G304 -- target path is resolved from configured source and provider name.
	b, err := os.ReadFile(target.Path)
	if err != nil {
		return "", nil, err
	}
	return target.Path, b, nil
}

func writeProviderContent(source providersource.Info, provider string, content string) error {
	target, err := providersource.ResolveProviderTarget(source, provider)
	if err != nil {
		return err
	}
	if source.SourceIsFile && target.Path == source.SourcePath {
		if err := os.MkdirAll(filepath.Dir(source.SourcePath), 0o750); err != nil {
			return err
		}
		// #nosec G304 -- source path is configured by the user.
		b, err := os.ReadFile(source.SourcePath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		updated, err := dslconfig.UpsertProviderBlock(source.SourcePath, string(b), provider, content)
		if err != nil {
			return err
		}
		return store.WriteAtomic(source.SourcePath, []byte(updated), false)
	}
	if err := os.MkdirAll(filepath.Dir(target.Path), 0o750); err != nil {
		return err
	}
	return store.WriteAtomic(target.Path, []byte(content), false)
}

func mapSourcePathUnderTemp(tmpRoot string, configRoot string, path string) (string, error) {
	rel, err := filepath.Rel(configRoot, path)
	if err != nil {
		return "", err
	}
	return filepath.Join(tmpRoot, rel), nil
}

func sourceConfigRoot(source providersource.Info) string {
	return filepath.Dir(source.SourcePath)
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
		// #nosec G304 -- path is discovered by filepath.WalkDir under trusted providers dir.
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

func readBodyLimit(rc io.Reader, limit int64) ([]byte, bool, error) {
	if rc == nil {
		return nil, false, nil
	}
	r := io.LimitReader(rc, limit+1)
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}
	if int64(len(b)) > limit {
		return b[:limit], true, nil
	}
	return b, false, nil
}

func resolveDefaultAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv(envAPIBaseURL)); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultAPIBaseURL
}

func resolveListenAddress(override string) string {
	if v := strings.TrimSpace(override); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envListen)); v != "" {
		return v
	}
	return defaultListen
}

func renderIndexHTML(defaultBaseURL string) string {
	base := strings.TrimSpace(defaultBaseURL)
	if base == "" {
		base = defaultAPIBaseURL
	}
	base = strings.TrimRight(base, "/")
	return strings.ReplaceAll(
		indexHTML,
		"__ONR_ADMIN_WEB_CURL_API_BASE_URL__",
		html.EscapeString(base),
	)
}
