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
	EditableError  string
}

type Target struct {
	Path   string
	IsFile bool
}

func Resolve(path string) (Info, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return Info{}, fmt.Errorf("provider source path is empty")
	}
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			if looksLikeDSLFilePath(p) {
				return Info{
					SourcePath:     p,
					SourceIsFile:   true,
					EditablePath:   p,
					EditableIsFile: true,
				}, nil
			}
			return Info{
				SourcePath:     p,
				SourceIsFile:   false,
				EditablePath:   p,
				EditableIsFile: false,
			}, nil
		}
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
			return Info{
				SourcePath:    p,
				SourceIsFile:  true,
				EditableError: fmt.Sprintf("provider source %q mixes inline provider blocks and included provider directories", p),
			}, nil
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
		return Info{
			SourcePath:    p,
			SourceIsFile:  true,
			EditableError: fmt.Sprintf("provider source %q includes multiple provider directories: %s", p, strings.Join(providerDirs, ", ")),
		}, nil
	}
	return Info{
		SourcePath:     p,
		SourceIsFile:   true,
		EditablePath:   p,
		EditableIsFile: true,
	}, nil
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

func ResolveProviderTarget(source Info, provider string) (Target, error) {
	name := dslconfig.NormalizeProviderName(provider)
	if strings.TrimSpace(name) == "" {
		return Target{}, fmt.Errorf("provider name is empty")
	}
	if strings.TrimSpace(source.EditablePath) != "" {
		if source.EditableIsFile {
			return Target{Path: source.EditablePath, IsFile: true}, nil
		}
		return Target{Path: filepath.Join(source.EditablePath, name+".conf"), IsFile: true}, nil
	}
	if !source.SourceIsFile {
		return Target{Path: filepath.Join(source.SourcePath, name+".conf"), IsFile: true}, nil
	}

	// #nosec G304 -- provider source path is configured by the user.
	body, err := os.ReadFile(source.SourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Target{Path: source.SourcePath, IsFile: true}, nil
		}
		return Target{}, err
	}
	content := string(body)
	if _, ok, err := dslconfig.ExtractProviderBlockOptional(source.SourcePath, content, name); err != nil {
		return Target{}, err
	} else if ok {
		return Target{Path: source.SourcePath, IsFile: true}, nil
	}

	files, err := dslconfig.ListIncludedFiles(source.SourcePath, content)
	if err != nil {
		return Target{}, err
	}
	matchedFiles := make([]string, 0)
	providerDirs := make(map[string]struct{})
	for _, file := range files {
		// #nosec G304 -- include files are resolved from explicit DSL include targets.
		b, err := os.ReadFile(file)
		if err != nil {
			return Target{}, err
		}
		if _, ok, err := dslconfig.ExtractProviderBlockOptional(file, string(b), name); err != nil {
			return Target{}, err
		} else if ok {
			matchedFiles = append(matchedFiles, file)
		}
		if _, ok, err := dslconfig.FindProviderNameOptional(file, string(b)); err != nil {
			return Target{}, err
		} else if ok {
			providerDirs[filepath.Dir(file)] = struct{}{}
		}
	}
	sort.Strings(matchedFiles)
	if len(matchedFiles) == 1 {
		return Target{Path: matchedFiles[0], IsFile: true}, nil
	}
	if len(matchedFiles) > 1 {
		return Target{}, fmt.Errorf("provider %q is defined in multiple included files: %s", name, strings.Join(matchedFiles, ", "))
	}

	dirs := make([]string, 0, len(providerDirs))
	for dir := range providerDirs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	if len(dirs) == 1 {
		return Target{Path: filepath.Join(dirs[0], name+".conf"), IsFile: true}, nil
	}
	if len(dirs) > 1 {
		return Target{}, fmt.Errorf("provider %q has no unique editable target across included provider directories: %s", name, strings.Join(dirs, ", "))
	}
	return Target{Path: source.SourcePath, IsFile: true}, nil
}

func looksLikeDSLFilePath(path string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".conf")
}
