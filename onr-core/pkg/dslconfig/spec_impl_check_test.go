package dslconfig

import (
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslspec"
)

func TestSpecImplCheck_DirectiveCoverage(t *testing.T) {
	spec, err := dslspec.LoadBuiltinSpec()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := dslspec.ValidateSpec(spec); err != nil {
		t.Fatalf("validate spec: %v", err)
	}

	for _, d := range spec.Directives {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			continue
		}
		allowedRuntime := toSet(DirectiveAllowedBlocks(name))
		if len(allowedRuntime) == 0 {
			t.Fatalf("spec directive %q is not recognized by runtime metadata", name)
		}
		for _, block := range d.AllowedIn {
			b := strings.TrimSpace(block)
			if b == "" {
				continue
			}
			if _, ok := allowedRuntime[b]; !ok {
				t.Fatalf("spec directive %q allows block %q, but runtime metadata does not", name, b)
			}
		}

		if len(d.Modes) > 0 {
			runtimeModes := toSet(ModesByDirective(name))
			for _, mode := range d.Modes {
				m := strings.TrimSpace(mode)
				if m == "" {
					continue
				}
				if _, ok := runtimeModes[m]; !ok {
					t.Fatalf("spec directive %q mode %q is missing in runtime metadata", name, m)
				}
			}
		}

		for argIdx, arg := range d.Args {
			if len(arg.Enum) == 0 {
				continue
			}
			for _, block := range d.AllowedIn {
				runtimeEnums := toSet(DirectiveArgEnumValuesInBlock(name, block, argIdx))
				if len(runtimeEnums) == 0 {
					// Many mode directives in current runtime metadata store allowlists in Modes.
					runtimeEnums = toSet(ModesByDirective(name))
				}
				for _, enumValue := range arg.Enum {
					v := strings.TrimSpace(enumValue)
					if v == "" {
						continue
					}
					if _, ok := runtimeEnums[v]; !ok {
						t.Fatalf("spec directive %q arg[%d]=%q enum %q is missing in runtime metadata (block=%q)", name, argIdx, arg.Name, v, block)
					}
				}
			}
		}
	}

	specByName := make(map[string]dslspec.DirectiveSpec, len(spec.Directives))
	for _, d := range spec.Directives {
		specByName[strings.TrimSpace(d.Name)] = d
	}

	for _, item := range DirectiveMetadataList() {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		sd, ok := specByName[name]
		if !ok {
			t.Fatalf("runtime metadata directive %q is missing in spec", name)
		}

		block := normalizeMetaBlock(item.Block)
		if block != "" {
			if _, ok := toSet(sd.AllowedIn)[block]; !ok {
				t.Fatalf("runtime metadata directive %q block %q is missing in spec allowed_in=%v", name, block, sd.AllowedIn)
			}
		}

		if len(item.Modes) > 0 {
			specModes := toSet(sd.Modes)
			for _, mode := range item.Modes {
				m := strings.TrimSpace(mode)
				if m == "" {
					continue
				}
				if _, ok := specModes[m]; !ok {
					t.Fatalf("runtime metadata directive %q mode %q is missing in spec", name, m)
				}
			}
		}

		for argIdx, arg := range item.Args {
			if len(arg.Enum) == 0 {
				continue
			}
			if len(sd.Args) <= argIdx {
				t.Fatalf("runtime metadata directive %q arg index %d is missing in spec", name, argIdx)
			}
			specEnums := toSet(sd.Args[argIdx].Enum)
			for _, ev := range arg.Enum {
				v := strings.TrimSpace(ev)
				if v == "" {
					continue
				}
				if _, ok := specEnums[v]; !ok {
					t.Fatalf("runtime metadata directive %q arg[%d] enum %q is missing in spec", name, argIdx, v)
				}
			}
		}
	}
}

func toSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}
