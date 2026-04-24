package store

import (
	"fmt"
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/models"
	"github.com/r9s-ai/open-next-router/pkg/config"
	"gopkg.in/yaml.v3"
)

type KeyUpdate = keyUpdate
type AccessKeyUpdate = accessKeyUpdate
type ModelUpdate = modelUpdate

// Ptr returns a non-nil pointer to v.
func Ptr[T any](v T) *T { return ptr(v) }

// LoadConfigIfExists returns nil, nil when path is empty; otherwise it returns a non-nil config on success.
func LoadConfigIfExists(path string) (*config.Config, error) { return loadConfigIfExists(path) }

// LoadOrInitKeysDoc returns a non-nil YAML document on success.
func LoadOrInitKeysDoc(path string) (*yaml.Node, error) { return loadOrInitKeysDoc(path) }

// LoadOrInitModelsDoc returns a non-nil YAML document on success.
func LoadOrInitModelsDoc(path string) (*yaml.Node, error) { return loadOrInitModelsDoc(path) }
func ParseProviders(s string) []string                    { return parseProviders(s) }
func EncodeYAML(doc *yaml.Node) ([]byte, error)           { return encodeYAML(doc) }
func WriteAtomic(path string, data []byte, backup bool) error {
	return writeAtomic(path, data, backup)
}

func ValueHint(v string) string { return valueHint(v) }

func ResolveDataPaths(cfg *config.Config, keysPath, modelsPath string) (string, string) {
	kp := strings.TrimSpace(keysPath)
	mp := strings.TrimSpace(modelsPath)
	if kp == "" {
		if cfg != nil && strings.TrimSpace(cfg.Keys.File) != "" {
			kp = strings.TrimSpace(cfg.Keys.File)
		} else {
			kp = "./keys.yaml"
		}
	}
	if mp == "" {
		if cfg != nil && strings.TrimSpace(cfg.Models.File) != "" {
			mp = strings.TrimSpace(cfg.Models.File)
		} else {
			mp = "./models.yaml"
		}
	}
	return kp, mp
}
func ResolveMasterKey(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.Auth.APIKey) != "" {
		return strings.TrimSpace(cfg.Auth.APIKey)
	}
	return strings.TrimSpace(os.Getenv("ONR_API_KEY"))
}

func ValidateKeysDoc(doc *yaml.Node) error   { return validateKeysDoc(doc) }
func ValidateModelsDoc(doc *yaml.Node) error { return validateModelsDoc(doc) }

func ListProviders(doc *yaml.Node) []string { return listProviders(doc) }

// GetProviderNode returns nil, false when the provider node does not exist.
func GetProviderNode(doc *yaml.Node, provider string) (*yaml.Node, bool) {
	return getProviderNode(doc, provider)
}

// EnsureProviderNode returns nil when provider is empty; otherwise it returns a non-nil provider node.
func EnsureProviderNode(doc *yaml.Node, provider string) *yaml.Node {
	return ensureProviderNode(doc, provider)
}
func DeleteProvider(doc *yaml.Node, provider string) error { return deleteProvider(doc, provider) }

func ListProviderKeys(doc *yaml.Node, provider string) ([]keystore.Key, error) {
	return listProviderKeys(doc, provider)
}
func AppendProviderKey(doc *yaml.Node, provider string, k keystore.Key) error {
	return appendProviderKey(doc, provider, k)
}
func UpdateProviderKey(doc *yaml.Node, provider string, index int, up KeyUpdate) error {
	return updateProviderKey(doc, provider, index, up)
}
func DeleteProviderKey(doc *yaml.Node, provider string, index int) error {
	return deleteProviderKey(doc, provider, index)
}
func UpstreamEnvVar(provider, name string, index int) string {
	return upstreamEnvVar(provider, name, index)
}

func ListAccessKeysDoc(doc *yaml.Node) ([]keystore.AccessKey, error) { return listAccessKeysDoc(doc) }
func AppendAccessKeyDoc(doc *yaml.Node, ak keystore.AccessKey) error {
	return appendAccessKeyDoc(doc, ak)
}
func UpdateAccessKeyDoc(doc *yaml.Node, index int, up AccessKeyUpdate) error {
	return updateAccessKeyDoc(doc, index, up)
}
func DeleteAccessKeyDoc(doc *yaml.Node, index int) error { return deleteAccessKeyDoc(doc, index) }
func AccessEnvVar(name string, index int) string         { return accessEnvVar(name, index) }

func ListModelIDs(doc *yaml.Node) []string { return listModelIDs(doc) }

// GetModelNode returns nil, false when the model node does not exist.
func GetModelNode(doc *yaml.Node, modelID string) (*yaml.Node, bool) {
	return getModelNode(doc, modelID)
}
func DeleteModel(doc *yaml.Node, modelID string) error { return deleteModel(doc, modelID) }
func GetModelRoute(doc *yaml.Node, modelID string) (models.Route, bool) {
	return getModelRoute(doc, modelID)
}
func SetModelRoute(doc *yaml.Node, modelID string, rt models.Route) error {
	return setModelRoute(doc, modelID, rt)
}
func UpdateModelRoute(doc *yaml.Node, modelID string, up ModelUpdate) error {
	return updateModelRoute(doc, modelID, up)
}

func EncryptKeysDocValues(doc *yaml.Node) (int, error) {
	changed := 0
	pm, err := providersMap(doc)
	if err != nil {
		return 0, err
	}
	for i := 0; i+1 < len(pm.Content); i += 2 {
		providerNode := pm.Content[i+1]
		if providerNode == nil || providerNode.Kind != yaml.MappingNode {
			continue
		}
		keysNode, ok := mappingGet(providerNode, "keys")
		if !ok || keysNode == nil || keysNode.Kind != yaml.SequenceNode {
			continue
		}
		for _, it := range keysNode.Content {
			c, err := encryptValueFieldIfNeeded(it)
			if err != nil {
				return 0, err
			}
			changed += c
		}
	}

	aks, err := accessKeysSeq(doc)
	if err != nil {
		return 0, err
	}
	for _, it := range aks.Content {
		c, err := encryptValueFieldIfNeeded(it)
		if err != nil {
			return 0, err
		}
		changed += c
	}
	return changed, nil
}

func encryptValueFieldIfNeeded(item *yaml.Node) (int, error) {
	if item == nil || item.Kind != yaml.MappingNode {
		return 0, nil
	}
	v, ok := mappingGet(item, "value")
	if !ok || v == nil {
		return 0, nil
	}
	raw := strings.TrimSpace(v.Value)
	if raw == "" || strings.HasPrefix(raw, "ENC[") {
		return 0, nil
	}
	enc, err := keystore.Encrypt(raw)
	if err != nil {
		return 0, fmt.Errorf("encrypt value failed: %w", err)
	}
	v.Kind = yaml.ScalarNode
	v.Tag = "!!str"
	v.Value = enc
	return 1, nil
}
