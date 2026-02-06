package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/internal/config"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"gopkg.in/yaml.v3"
)

func loadConfigIfExists(path string) (*config.Config, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, nil
	}
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	return config.Load(p)
}

func loadOrInitKeysDoc(path string) (*yaml.Node, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, errors.New("missing keys path")
	}
	b, err := os.ReadFile(p) // #nosec G304 -- admin tool reads user-provided file.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return initEmptyKeysDoc(), nil
		}
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 {
		return initEmptyKeysDoc(), nil
	}
	ensureProvidersMap(&doc)
	return &doc, nil
}

func loadOrInitModelsDoc(path string) (*yaml.Node, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, errors.New("missing models path")
	}
	b, err := os.ReadFile(p) // #nosec G304 -- admin tool reads user-provided file.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return initEmptyModelsDoc(), nil
		}
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 {
		return initEmptyModelsDoc(), nil
	}
	ensureModelsMap(&doc)
	return &doc, nil
}

func initEmptyKeysDoc() *yaml.Node {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	m := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = []*yaml.Node{m}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "providers", Tag: "!!str"},
		&yaml.Node{Kind: yaml.MappingNode},
	)
	return doc
}

func initEmptyModelsDoc() *yaml.Node {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	m := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = []*yaml.Node{m}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "models", Tag: "!!str"},
		&yaml.Node{Kind: yaml.MappingNode},
	)
	return doc
}

func ensureProvidersMap(doc *yaml.Node) {
	if doc == nil {
		return
	}
	if doc.Kind != yaml.DocumentNode {
		return
	}
	if len(doc.Content) == 0 || doc.Content[0] == nil {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return
	}
	if _, ok := mappingGet(root, "providers"); ok {
		return
	}
	mappingSet(root, "providers", &yaml.Node{Kind: yaml.MappingNode})
}

func ensureModelsMap(doc *yaml.Node) {
	if doc == nil {
		return
	}
	if doc.Kind != yaml.DocumentNode {
		return
	}
	if len(doc.Content) == 0 || doc.Content[0] == nil {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return
	}
	if _, ok := mappingGet(root, "models"); ok {
		return
	}
	mappingSet(root, "models", &yaml.Node{Kind: yaml.MappingNode})
}

type keyUpdate struct {
	Name            *string
	Value           *string
	BaseURLOverride *string
}

type accessKeyUpdate struct {
	Name     *string
	Value    *string
	Disabled *bool
	Comment  *string
}

type modelUpdate struct {
	Providers *[]string
	Strategy  *string
	OwnedBy   *string
}

func ptr[T any](v T) *T { return &v }

func parseProviders(s string) []string {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func encodeYAML(doc *yaml.Node) ([]byte, error) {
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, err
	}
	_ = enc.Close()
	return []byte(sb.String()), nil
}

func writeAtomic(path string, data []byte, backup bool) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return errors.New("missing path")
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if backup {
		if _, err := os.Stat(p); err == nil {
			ts := time.Now().Format("20060102-150405")
			bpath := p + ".bak." + ts
			if err := copyFile(p, bpath); err != nil {
				return err
			}
		}
	}

	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 -- admin tool reads user-provided file.
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func valueHint(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "(empty)"
	}
	if strings.HasPrefix(s, "ENC[") {
		return "(ENC[...])"
	}
	if len(s) <= 8 {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%q...(%d)", s[:4], len(s))
}

// YAML helpers (order-preserving)

func mappingGet(m *yaml.Node, key string) (*yaml.Node, bool) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		v := m.Content[i+1]
		if k != nil && k.Value == key {
			return v, true
		}
	}
	return nil, false
}

func mappingSet(m *yaml.Node, key string, val *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k != nil && k.Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		val,
	)
}

func mappingDel(m *yaml.Node, key string) bool {
	if m == nil || m.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k != nil && k.Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return true
		}
	}
	return false
}

func providersMap(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return nil, errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("root is not mapping")
	}
	pn, ok := mappingGet(root, "providers")
	if !ok || pn == nil {
		pn = &yaml.Node{Kind: yaml.MappingNode}
		mappingSet(root, "providers", pn)
	}
	if pn.Kind != yaml.MappingNode {
		return nil, errors.New("providers is not mapping")
	}
	return pn, nil
}

func modelsMap(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return nil, errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("root is not mapping")
	}
	mn, ok := mappingGet(root, "models")
	if !ok || mn == nil {
		mn = &yaml.Node{Kind: yaml.MappingNode}
		mappingSet(root, "models", mn)
	}
	if mn.Kind != yaml.MappingNode {
		return nil, errors.New("models is not mapping")
	}
	return mn, nil
}

func listProviders(doc *yaml.Node) []string {
	pm, err := providersMap(doc)
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(pm.Content); i += 2 {
		k := pm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) != "" {
			out = append(out, strings.TrimSpace(k.Value))
		}
	}
	return out
}

func listModelIDs(doc *yaml.Node) []string {
	mm, err := modelsMap(doc)
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(mm.Content); i += 2 {
		k := mm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) != "" {
			out = append(out, strings.TrimSpace(k.Value))
		}
	}
	return out
}

func getProviderNode(doc *yaml.Node, provider string) (*yaml.Node, bool) {
	pm, err := providersMap(doc)
	if err != nil {
		return nil, false
	}
	want := strings.TrimSpace(provider)
	for i := 0; i+1 < len(pm.Content); i += 2 {
		k := pm.Content[i]
		v := pm.Content[i+1]
		if k != nil && strings.TrimSpace(k.Value) == want {
			return v, true
		}
	}
	return nil, false
}

func getModelNode(doc *yaml.Node, modelID string) (*yaml.Node, bool) {
	mm, err := modelsMap(doc)
	if err != nil {
		return nil, false
	}
	want := strings.TrimSpace(modelID)
	for i := 0; i+1 < len(mm.Content); i += 2 {
		k := mm.Content[i]
		v := mm.Content[i+1]
		if k != nil && strings.TrimSpace(k.Value) == want {
			return v, true
		}
	}
	return nil, false
}

func ensureProviderNode(doc *yaml.Node, provider string) *yaml.Node {
	pm, _ := providersMap(doc)
	p := strings.TrimSpace(provider)
	if p == "" {
		return nil
	}
	if n, ok := getProviderNode(doc, p); ok && n != nil {
		return n
	}
	// Append at end to preserve existing order.
	provNode := &yaml.Node{Kind: yaml.MappingNode}
	keysSeq := &yaml.Node{Kind: yaml.SequenceNode}
	mappingSet(provNode, "keys", keysSeq)
	pm.Content = append(pm.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: p, Tag: "!!str"},
		provNode,
	)
	return provNode
}

func ensureModelNode(doc *yaml.Node, modelID string) *yaml.Node {
	mm, _ := modelsMap(doc)
	id := strings.TrimSpace(modelID)
	if id == "" {
		return nil
	}
	if n, ok := getModelNode(doc, id); ok && n != nil {
		return n
	}
	rtNode := &yaml.Node{Kind: yaml.MappingNode}
	mm.Content = append(mm.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: id, Tag: "!!str"},
		rtNode,
	)
	return rtNode
}

func deleteProvider(doc *yaml.Node, provider string) error {
	pm, err := providersMap(doc)
	if err != nil {
		return err
	}
	want := strings.TrimSpace(provider)
	for i := 0; i+1 < len(pm.Content); i += 2 {
		k := pm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) == want {
			pm.Content = append(pm.Content[:i], pm.Content[i+2:]...)
			return nil
		}
	}
	return fmt.Errorf("provider not found: %s", want)
}

func deleteModel(doc *yaml.Node, modelID string) error {
	mm, err := modelsMap(doc)
	if err != nil {
		return err
	}
	want := strings.TrimSpace(modelID)
	for i := 0; i+1 < len(mm.Content); i += 2 {
		k := mm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) == want {
			mm.Content = append(mm.Content[:i], mm.Content[i+2:]...)
			return nil
		}
	}
	return fmt.Errorf("model not found: %s", want)
}

func getModelRoute(doc *yaml.Node, modelID string) (models.Route, bool) {
	n, ok := getModelNode(doc, modelID)
	if !ok || n == nil || n.Kind != yaml.MappingNode {
		return models.Route{}, false
	}
	rt := models.Route{}
	if v, ok := mappingGet(n, "providers"); ok && v != nil && v.Kind == yaml.SequenceNode {
		for _, it := range v.Content {
			if it == nil {
				continue
			}
			p := strings.ToLower(strings.TrimSpace(it.Value))
			if p != "" {
				rt.Providers = append(rt.Providers, p)
			}
		}
	}
	if v, ok := mappingGet(n, "strategy"); ok && v != nil {
		rt.Strategy = models.Strategy(strings.TrimSpace(v.Value))
	}
	if v, ok := mappingGet(n, "owned_by"); ok && v != nil {
		rt.OwnedBy = strings.TrimSpace(v.Value)
	}
	return rt, true
}

func setModelRoute(doc *yaml.Node, modelID string, rt models.Route) error {
	n := ensureModelNode(doc, modelID)
	if n == nil {
		return errors.New("invalid model id")
	}
	if n.Kind != yaml.MappingNode {
		return errors.New("model route is not mapping")
	}
	// providers
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, p := range rt.Providers {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: p})
	}
	mappingSet(n, "providers", seq)
	// strategy
	if strings.TrimSpace(string(rt.Strategy)) != "" {
		mappingSet(n, "strategy", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(string(rt.Strategy))})
	}
	// owned_by
	if strings.TrimSpace(rt.OwnedBy) != "" {
		mappingSet(n, "owned_by", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(rt.OwnedBy)})
	}
	return nil
}

func updateModelRoute(doc *yaml.Node, modelID string, up modelUpdate) error {
	n, ok := getModelNode(doc, modelID)
	if !ok || n == nil {
		return fmt.Errorf("model not found: %s", strings.TrimSpace(modelID))
	}
	if n.Kind != yaml.MappingNode {
		return errors.New("model route is not mapping")
	}
	if up.Providers != nil {
		if *up.Providers == nil {
			mappingDel(n, "providers")
		} else {
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, p := range *up.Providers {
				p = strings.ToLower(strings.TrimSpace(p))
				if p == "" {
					continue
				}
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: p})
			}
			mappingSet(n, "providers", seq)
		}
	}
	if up.Strategy != nil {
		if strings.TrimSpace(*up.Strategy) == "" {
			mappingDel(n, "strategy")
		} else {
			mappingSet(n, "strategy", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Strategy)})
		}
	}
	if up.OwnedBy != nil {
		if strings.TrimSpace(*up.OwnedBy) == "" {
			mappingDel(n, "owned_by")
		} else {
			mappingSet(n, "owned_by", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.OwnedBy)})
		}
	}
	return nil
}

func providerKeysSeq(doc *yaml.Node, provider string) (*yaml.Node, error) {
	pn := ensureProviderNode(doc, provider)
	if pn == nil || pn.Kind != yaml.MappingNode {
		return nil, errors.New("invalid provider node")
	}
	kn, ok := mappingGet(pn, "keys")
	if !ok || kn == nil {
		kn = &yaml.Node{Kind: yaml.SequenceNode}
		mappingSet(pn, "keys", kn)
	}
	if kn.Kind != yaml.SequenceNode {
		return nil, errors.New("provider.keys is not sequence")
	}
	return kn, nil
}

func listProviderKeys(doc *yaml.Node, provider string) ([]keystore.Key, error) {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return nil, err
	}
	var out []keystore.Key
	for _, it := range seq.Content {
		if it == nil || it.Kind != yaml.MappingNode {
			continue
		}
		k := keystore.Key{}
		if v, ok := mappingGet(it, "name"); ok && v != nil {
			k.Name = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "value"); ok && v != nil {
			k.Value = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "base_url_override"); ok && v != nil {
			k.BaseURLOverride = strings.TrimSpace(v.Value)
		}
		out = append(out, k)
	}
	return out, nil
}

func appendProviderKey(doc *yaml.Node, provider string, k keystore.Key) error {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return err
	}
	m := &yaml.Node{Kind: yaml.MappingNode}
	if strings.TrimSpace(k.Name) != "" {
		mappingSet(m, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(k.Name)})
	}
	mappingSet(m, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(k.Value)})
	if strings.TrimSpace(k.BaseURLOverride) != "" {
		mappingSet(m, "base_url_override", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(k.BaseURLOverride)})
	}
	seq.Content = append(seq.Content, m)
	return nil
}

func updateProviderKey(doc *yaml.Node, provider string, index int, up keyUpdate) error {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	it := seq.Content[index]
	if it == nil || it.Kind != yaml.MappingNode {
		return errors.New("invalid key node")
	}
	if up.Name != nil {
		if strings.TrimSpace(*up.Name) == "" {
			mappingDel(it, "name")
		} else {
			mappingSet(it, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Name)})
		}
	}
	if up.Value != nil {
		mappingSet(it, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Value)})
	}
	if up.BaseURLOverride != nil {
		if strings.TrimSpace(*up.BaseURLOverride) == "" {
			mappingDel(it, "base_url_override")
		} else {
			mappingSet(it, "base_url_override", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.BaseURLOverride)})
		}
	}
	return nil
}

func deleteProviderKey(doc *yaml.Node, provider string, index int) error {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	seq.Content = append(seq.Content[:index], seq.Content[index+1:]...)
	return nil
}

func accessKeysSeq(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return nil, errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("root is not mapping")
	}
	kn, ok := mappingGet(root, "access_keys")
	if !ok || kn == nil {
		kn = &yaml.Node{Kind: yaml.SequenceNode}
		mappingSet(root, "access_keys", kn)
	}
	if kn.Kind != yaml.SequenceNode {
		return nil, errors.New("access_keys is not sequence")
	}
	return kn, nil
}

func listAccessKeysDoc(doc *yaml.Node) ([]keystore.AccessKey, error) {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return nil, err
	}
	var out []keystore.AccessKey
	for _, it := range seq.Content {
		if it == nil || it.Kind != yaml.MappingNode {
			continue
		}
		ak := keystore.AccessKey{}
		if v, ok := mappingGet(it, "name"); ok && v != nil {
			ak.Name = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "value"); ok && v != nil {
			ak.Value = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "disabled"); ok && v != nil {
			switch strings.ToLower(strings.TrimSpace(v.Value)) {
			case "true", "y", "yes", "1":
				ak.Disabled = true
			}
		}
		if v, ok := mappingGet(it, "comment"); ok && v != nil {
			ak.Comment = strings.TrimSpace(v.Value)
		}
		out = append(out, ak)
	}
	return out, nil
}

func appendAccessKeyDoc(doc *yaml.Node, ak keystore.AccessKey) error {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return err
	}
	m := &yaml.Node{Kind: yaml.MappingNode}
	if strings.TrimSpace(ak.Name) != "" {
		mappingSet(m, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(ak.Name)})
	}
	mappingSet(m, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(ak.Value)})
	if ak.Disabled {
		mappingSet(m, "disabled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
	if strings.TrimSpace(ak.Comment) != "" {
		mappingSet(m, "comment", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(ak.Comment)})
	}
	seq.Content = append(seq.Content, m)
	return nil
}

func updateAccessKeyDoc(doc *yaml.Node, index int, up accessKeyUpdate) error {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	it := seq.Content[index]
	if it == nil || it.Kind != yaml.MappingNode {
		return errors.New("invalid access_key node")
	}
	if up.Name != nil {
		if strings.TrimSpace(*up.Name) == "" {
			mappingDel(it, "name")
		} else {
			mappingSet(it, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Name)})
		}
	}
	if up.Value != nil {
		mappingSet(it, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Value)})
	}
	if up.Disabled != nil {
		if *up.Disabled {
			mappingSet(it, "disabled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
		} else {
			mappingDel(it, "disabled")
		}
	}
	if up.Comment != nil {
		if strings.TrimSpace(*up.Comment) == "" {
			mappingDel(it, "comment")
		} else {
			mappingSet(it, "comment", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Comment)})
		}
	}
	return nil
}

func deleteAccessKeyDoc(doc *yaml.Node, index int) error {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	seq.Content = append(seq.Content[:index], seq.Content[index+1:]...)
	return nil
}

func validateKeysDoc(doc *yaml.Node) error {
	if _, err := providersMap(doc); err != nil {
		return err
	}
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return errors.New("root is not mapping")
	}
	if akn, ok := mappingGet(root, "access_keys"); ok && akn != nil && akn.Kind != yaml.SequenceNode {
		return errors.New("access_keys is not sequence")
	}
	return nil
}

func validateModelsDoc(doc *yaml.Node) error {
	if _, err := modelsMap(doc); err != nil {
		return err
	}
	b, err := encodeYAML(doc)
	if err != nil {
		return err
	}
	var f models.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}
	return nil
}

func upstreamEnvVar(provider, name string, index int) string {
	// Keep consistent with internal/keystore env var behavior.
	p := strings.ToUpper(strings.TrimSpace(provider))
	n := strings.ToUpper(strings.TrimSpace(name))
	if n == "" {
		return fmt.Sprintf("ONR_UPSTREAM_KEY_%s_%d", sanitizeEnvToken(p), index+1)
	}
	return fmt.Sprintf("ONR_UPSTREAM_KEY_%s_%s", sanitizeEnvToken(p), sanitizeEnvToken(n))
}

func accessEnvVar(name string, index int) string {
	n := strings.ToUpper(strings.TrimSpace(name))
	if n == "" {
		return fmt.Sprintf("ONR_ACCESS_KEY_%d", index+1)
	}
	return fmt.Sprintf("ONR_ACCESS_KEY_%s", sanitizeEnvToken(n))
}

func sanitizeEnvToken(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
