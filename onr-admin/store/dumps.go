package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DumpSummary struct {
	Path     string
	FileName string
	ModTime  time.Time
	Size     int64

	Time      time.Time
	RequestID string
	Method    string
	URLPath   string
	ClientIP  string
	Provider  string

	Model  string
	Stream *bool

	ProxyStatus  int
	StreamError  string
	HasTruncated bool
}

type DumpListOptions struct {
	Dir   string
	Limit int
}

func ListDumpSummaries(opts DumpListOptions) ([]DumpSummary, error) {
	dir := strings.TrimSpace(opts.Dir)
	if dir == "" {
		return nil, errors.New("dump dir is empty")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	type fileItem struct {
		path string
		info fs.FileInfo
	}
	items := make([]fileItem, 0, limit)
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".log") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		items = append(items, fileItem{path: path, info: info})
		return nil
	}); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		mi := items[i].info.ModTime()
		mj := items[j].info.ModTime()
		if !mi.Equal(mj) {
			return mi.After(mj) // newest first
		}
		return items[i].info.Name() > items[j].info.Name()
	})
	if len(items) > limit {
		items = items[:limit]
	}

	out := make([]DumpSummary, 0, len(items))
	for _, it := range items {
		sum, err := ParseDumpSummary(it.path, it.info)
		if err != nil {
			// Best-effort: still include file entry with basic stats.
			out = append(out, DumpSummary{
				Path:     it.path,
				FileName: it.info.Name(),
				ModTime:  it.info.ModTime(),
				Size:     it.info.Size(),
			})
			continue
		}
		out = append(out, sum)
	}
	return out, nil
}

func ParseDumpSummary(path string, info fs.FileInfo) (DumpSummary, error) {
	sum := DumpSummary{
		Path:     path,
		FileName: filepath.Base(path),
	}
	if info != nil {
		sum.ModTime = info.ModTime()
		sum.Size = info.Size()
	}

	f, err := os.Open(path) // #nosec G304 -- admin tool reads user-provided dump dir.
	if err != nil {
		return DumpSummary{}, err
	}
	defer func() { _ = f.Close() }()

	if err := parseDumpSummaryFromReader(&sum, f); err != nil {
		return DumpSummary{}, err
	}
	if sum.Time.IsZero() {
		sum.Time = sum.ModTime
	}
	return sum, nil
}

func parseDumpSummaryFromReader(sum *DumpSummary, r io.Reader) error {
	if sum == nil {
		return errors.New("nil summary")
	}

	br := bufio.NewReader(r)
	section := ""
	gotOrigin := false
	var originBuf strings.Builder
	originBytes := 0

	for {
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		t := strings.TrimSpace(trimmed)
		if strings.HasPrefix(t, "=== ") && strings.HasSuffix(t, " ===") {
			section = t
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}

		switch section {
		case "=== META ===":
			parseDumpMetaLine(sum, trimmed)
		case "=== ORIGIN REQUEST ===":
			// Content ends with an empty line (writeBlock always appends a blank line).
			if t == "" {
				gotOrigin = true
				parseDumpOrigin(sum, originBuf.String())
				originBuf.Reset()
				originBytes = 0
				break
			}
			// Avoid huge allocations: we only need a small prefix to extract model/stream.
			if originBytes < 256*1024 {
				originBuf.WriteString(trimmed)
				originBuf.WriteByte('\n')
				originBytes += len(trimmed) + 1
			}
		case "=== PROXY RESPONSE ===":
			if strings.HasPrefix(strings.ToLower(t), "status=") {
				v := strings.TrimSpace(strings.TrimPrefix(t, "status="))
				if n, xerr := strconv.Atoi(v); xerr == nil {
					sum.ProxyStatus = n
				}
			}
		case "=== STREAM ===":
			if strings.HasPrefix(strings.ToLower(t), "error=") {
				sum.StreamError = strings.TrimSpace(strings.TrimPrefix(trimmed, "error="))
			}
		default:
			if strings.EqualFold(t, "[truncated]") {
				sum.HasTruncated = true
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}

		// Fast path: once we have core fields, we can stop scanning.
		if sum.RequestID != "" &&
			sum.Method != "" &&
			sum.URLPath != "" &&
			sum.Provider != "" &&
			gotOrigin &&
			sum.ProxyStatus != 0 {
			break
		}
	}

	// If the file ended while still in ORIGIN REQUEST, best-effort parse.
	if section == "=== ORIGIN REQUEST ===" && originBuf.Len() > 0 {
		parseDumpOrigin(sum, originBuf.String())
	}

	return nil
}

func parseDumpMetaLine(sum *DumpSummary, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}

	if strings.HasPrefix(trimmed, "time=") {
		raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "time="))
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			sum.Time = ts
		}
		return
	}
	if strings.HasPrefix(trimmed, "request_id=") {
		sum.RequestID = strings.TrimSpace(strings.TrimPrefix(trimmed, "request_id="))
		return
	}
	if strings.HasPrefix(trimmed, "method=") {
		sum.Method = strings.TrimSpace(strings.TrimPrefix(trimmed, "method="))
		return
	}
	if strings.HasPrefix(trimmed, "path=") {
		sum.URLPath = strings.TrimSpace(strings.TrimPrefix(trimmed, "path="))
		return
	}
	if strings.HasPrefix(trimmed, "client_ip=") {
		sum.ClientIP = strings.TrimSpace(strings.TrimPrefix(trimmed, "client_ip="))
		return
	}

	// headers section lines: "  Key: Value"
	if strings.HasPrefix(line, "  ") {
		kv := strings.TrimSpace(strings.TrimPrefix(line, "  "))
		parts := strings.SplitN(kv, ":", 2)
		if len(parts) != 2 {
			return
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if strings.EqualFold(key, "X-Onr-Provider") {
			sum.Provider = val
		}
		return
	}
}

func parseDumpOrigin(sum *DumpSummary, raw string) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return
	}
	if strings.HasPrefix(body, "[binary body omitted]") {
		return
	}

	type req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}

	var v req
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return
	}
	if strings.TrimSpace(v.Model) != "" {
		sum.Model = strings.TrimSpace(v.Model)
	}
	if v.Stream != nil {
		sum.Stream = v.Stream
	}
}

func DumpUniqueOptions(dumps []DumpSummary) (providers []string, models []string, paths []string, statuses []int) {
	pset := map[string]struct{}{}
	mset := map[string]struct{}{}
	pathset := map[string]struct{}{}
	sset := map[int]struct{}{}

	for _, d := range dumps {
		if v := strings.TrimSpace(d.Provider); v != "" {
			pset[v] = struct{}{}
		}
		if v := strings.TrimSpace(d.Model); v != "" {
			mset[v] = struct{}{}
		}
		if v := strings.TrimSpace(d.URLPath); v != "" {
			pathset[v] = struct{}{}
		}
		if d.ProxyStatus != 0 {
			sset[d.ProxyStatus] = struct{}{}
		}
	}

	for v := range pset {
		providers = append(providers, v)
	}
	sort.Strings(providers)

	for v := range mset {
		models = append(models, v)
	}
	sort.Strings(models)

	for v := range pathset {
		paths = append(paths, v)
	}
	sort.Strings(paths)

	for v := range sset {
		statuses = append(statuses, v)
	}
	sort.Ints(statuses)

	return providers, models, paths, statuses
}

func FormatDumpRow(d DumpSummary) string {
	ts := d.Time
	if ts.IsZero() {
		ts = d.ModTime
	}
	timeText := "-"
	if !ts.IsZero() {
		timeText = ts.Format("2006-01-02 15:04:05")
	}

	status := "-"
	if d.ProxyStatus != 0 {
		status = strconv.Itoa(d.ProxyStatus)
	}
	provider := strings.TrimSpace(d.Provider)
	if provider == "" {
		provider = "-"
	}
	model := strings.TrimSpace(d.Model)
	if model == "" {
		model = "-"
	}
	path := strings.TrimSpace(d.URLPath)
	if path == "" {
		path = "-"
	}
	rid := strings.TrimSpace(d.RequestID)
	if rid == "" {
		rid = strings.TrimSuffix(d.FileName, filepath.Ext(d.FileName))
	}
	return fmt.Sprintf("%s status=%s provider=%s model=%s path=%s rid=%s", timeText, status, provider, model, path, rid)
}
