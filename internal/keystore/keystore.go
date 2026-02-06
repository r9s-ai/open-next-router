package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Store struct {
	mu      sync.Mutex
	byProv  map[string][]Key
	nextIdx map[string]int

	accessKeys []AccessKey
}

type Key struct {
	Name            string `yaml:"name"`
	Value           string `yaml:"value"`
	BaseURLOverride string `yaml:"base_url_override"`
}

type AccessKey struct {
	Name     string `yaml:"name"`
	Value    string `yaml:"value"`
	Disabled bool   `yaml:"disabled"`
	Comment  string `yaml:"comment"`
}

type fileFormat struct {
	Providers map[string]struct {
		Keys []Key `yaml:"keys"`
	} `yaml:"providers"`

	AccessKeys []AccessKey `yaml:"access_keys"`
}

var encValuePattern = regexp.MustCompile(`^ENC\[v1:aesgcm:([A-Za-z0-9+/=]+)\]$`)

func Load(path string) (*Store, error) {
	// #nosec G304 -- path is provided by trusted config.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ff fileFormat
	if err := yaml.Unmarshal(b, &ff); err != nil {
		return nil, err
	}
	out := &Store{
		byProv:  map[string][]Key{},
		nextIdx: map[string]int{},
	}
	for prov, v := range ff.Providers {
		p := normalizeProvider(prov)
		if p == "" {
			continue
		}
		keys := make([]Key, 0, len(v.Keys))
		for i, k := range v.Keys {
			k.Name = strings.TrimSpace(k.Name)
			k.BaseURLOverride = strings.TrimSpace(k.BaseURLOverride)

			raw := strings.TrimSpace(k.Value)
			if envVal := strings.TrimSpace(os.Getenv(envVarForUpstreamKey(p, k.Name, i))); envVal != "" {
				raw = envVal
			}
			if raw == "" {
				continue
			}

			val, err := decryptIfNeeded(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid key value for provider=%s name=%q: %w", p, k.Name, err)
			}
			k.Value = val
			keys = append(keys, k)
		}
		if len(keys) > 0 {
			out.byProv[p] = keys
		}
	}

	aks := make([]AccessKey, 0, len(ff.AccessKeys))
	for i, ak := range ff.AccessKeys {
		if ak.Disabled {
			continue
		}
		ak.Name = strings.TrimSpace(ak.Name)
		ak.Comment = strings.TrimSpace(ak.Comment)

		raw := strings.TrimSpace(ak.Value)
		if envVal := strings.TrimSpace(os.Getenv(envVarForAccessKey(ak.Name, i))); envVal != "" {
			raw = envVal
		}
		if raw == "" {
			continue
		}
		val, err := decryptIfNeeded(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid access_keys value name=%q: %w", ak.Name, err)
		}
		ak.Value = val
		aks = append(aks, ak)
	}
	out.accessKeys = aks

	if len(out.byProv) == 0 && len(out.accessKeys) == 0 {
		return nil, errors.New("keys.yaml has no provider keys or access_keys configured")
	}
	return out, nil
}

func (s *Store) NextKey(provider string) (Key, bool) {
	if s == nil {
		return Key{}, false
	}
	p := normalizeProvider(provider)
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := s.byProv[p]
	if len(keys) == 0 {
		return Key{}, false
	}
	i := s.nextIdx[p] % len(keys)
	s.nextIdx[p] = (i + 1) % len(keys)
	return keys[i], true
}

func (s *Store) MatchAccessKey(value string) (AccessKey, bool) {
	if s == nil {
		return AccessKey{}, false
	}
	v := strings.TrimSpace(value)
	if v == "" {
		return AccessKey{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ak := range s.accessKeys {
		if subtle.ConstantTimeCompare([]byte(v), []byte(ak.Value)) == 1 {
			return ak, true
		}
	}
	return AccessKey{}, false
}

func (s *Store) AccessKeys() []AccessKey {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AccessKey, len(s.accessKeys))
	copy(out, s.accessKeys)
	return out
}

func (s *Store) HasProvider(provider string) bool {
	if s == nil {
		return false
	}
	p := normalizeProvider(provider)
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byProv[p]) > 0
}

func normalizeProvider(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func envVarForUpstreamKey(provider string, name string, index int) string {
	p := strings.ToUpper(strings.TrimSpace(provider))
	n := strings.ToUpper(strings.TrimSpace(name))
	if n == "" {
		return fmt.Sprintf("ONR_UPSTREAM_KEY_%s_%d", sanitizeEnvToken(p), index+1)
	}
	return fmt.Sprintf("ONR_UPSTREAM_KEY_%s_%s", sanitizeEnvToken(p), sanitizeEnvToken(n))
}

func envVarForAccessKey(name string, index int) string {
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

func decryptIfNeeded(raw string) (string, error) {
	m := encValuePattern.FindStringSubmatch(raw)
	if m == nil {
		return raw, nil
	}
	key, err := loadMasterKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(m[1])
	if err != nil {
		return "", fmt.Errorf("invalid base64 ciphertext: %w", err)
	}
	if len(data) < 12 {
		return "", errors.New("ciphertext too short")
	}
	nonce := data[:12]
	ct := data[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}
	return string(pt), nil
}

func Encrypt(plain string) (string, error) {
	key, err := loadMasterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), nil)
	buf := make([]byte, 0, len(nonce)+len(ct))
	buf = append(buf, nonce...)
	buf = append(buf, ct...)
	return "ENC[v1:aesgcm:" + base64.StdEncoding.EncodeToString(buf) + "]", nil
}

func loadMasterKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("ONR_MASTER_KEY"))
	if raw == "" {
		return nil, errors.New("ONR_MASTER_KEY is required to decrypt ENC[...] values")
	}
	// Accept either raw 32-byte string or base64.
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("ONR_MASTER_KEY must be 32 bytes or base64-encoded 32 bytes")
	}
	if len(b) != 32 {
		return nil, errors.New("ONR_MASTER_KEY must be 32 bytes (AES-256)")
	}
	return b, nil
}
