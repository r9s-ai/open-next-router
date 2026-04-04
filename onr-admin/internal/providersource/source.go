package providersource

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

type Info struct {
	SourcePath     string
	SourceIsFile   bool
	EditablePath   string
	EditableIsFile bool
}

func Resolve(path string) (Info, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return Info{}, fmt.Errorf("provider source path is empty")
	}
	info, err := os.Stat(p)
	if err != nil {
		return Info{}, err
	}
	if info.IsDir() {
		return Info{
			SourcePath:     p,
			SourceIsFile:   false,
			EditablePath:   p,
			EditableIsFile: false,
		}, nil
	}
	// #nosec G304 -- provider source path is configured by the user.
	body, err := os.ReadFile(p)
	if err != nil {
		return Info{}, err
	}
	content := string(body)
	blocks, err := dslconfig.ListProviderBlocks(p, content)
	if err != nil {
		return Info{}, err
	}
	providerDirs, err := discoverIncludedProviderDirs(p, content)
	if err != nil {
		return Info{}, err
	}
	if len(blocks) > 0 {
		if len(providerDirs) > 0 {
			return Info{}, fmt.Errorf("provider source %q mixes inline provider blocks and included provider directories", p)
		}
		return Info{
			SourcePath:     p,
			SourceIsFile:   true,
			EditablePath:   p,
			EditableIsFile: true,
		}, nil
	}
	if len(providerDirs) == 1 {
		return Info{
			SourcePath:     p,
			SourceIsFile:   true,
			EditablePath:   providerDirs[0],
			EditableIsFile: false,
		}, nil
	}
	if len(providerDirs) > 1 {
		return Info{}, fmt.Errorf("provider source %q includes multiple provider directories: %s", p, strings.Join(providerDirs, ", "))
	}
	return Info{}, fmt.Errorf("provider source %q has no editable provider blocks or included provider directory", p)
}

func discoverIncludedProviderDirs(path string, content string) ([]string, error) {
	files, err := dslconfig.ListIncludedFiles(path, content)
	if err != nil {
		return nil, err
	}
	dirs := make(map[string]struct{})
	for _, file := range files {
		// #nosec G304 -- include files are resolved from explicit DSL include targets.
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if _, ok, err := dslconfig.FindProviderNameOptional(file, string(body)); err != nil {
			return nil, err
		} else if ok {
			dirs[filepath.Dir(file)] = struct{}{}
		}
	}
	out := make([]string, 0, len(dirs))
	for dir := range dirs {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out, nil
}
