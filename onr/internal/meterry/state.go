package meterry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type subjectStateStore struct {
	mu         sync.Mutex
	path       string
	blocked    map[string]bool
	webhookIDs map[string]struct{}
}

type persistedSubjectState struct {
	Blocked    map[string]bool `json:"blocked"`
	WebhookIDs []string        `json:"webhook_ids"`
}

func openSubjectState(dir string) (*subjectStateStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("meterry subject state directory is empty")
	}
	s := &subjectStateStore{
		path:       filepath.Join(dir, "balance-state.json"),
		blocked:    map[string]bool{},
		webhookIDs: map[string]struct{}{},
	}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read meterry balance state: %w", err)
	}
	var p persistedSubjectState
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("decode meterry balance state: %w", err)
	}
	for k, v := range p.Blocked {
		s.blocked[k] = v
	}
	for _, id := range p.WebhookIDs {
		if strings.TrimSpace(id) != "" {
			s.webhookIDs[id] = struct{}{}
		}
	}
	return s, nil
}

func (s *subjectStateStore) isBlocked(subjectType, subjectID string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blocked[subjectKey(subjectType, subjectID)]
}

func (s *subjectStateStore) applyWebhook(eventID, eventType, subjectType, subjectID string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if eventID != "" {
		if _, ok := s.webhookIDs[eventID]; ok {
			return nil
		}
		s.webhookIDs[eventID] = struct{}{}
	}
	key := subjectKey(subjectType, subjectID)
	switch eventType {
	case "wallet.insufficient_balance", "usage_limit.exhausted":
		if key != "" {
			s.blocked[key] = true
		}
	case "wallet.balance_changed":
		if key != "" {
			delete(s.blocked, key)
		}
	}
	return s.persistLocked()
}

func (s *subjectStateStore) persistLocked() error {
	ids := make([]string, 0, len(s.webhookIDs))
	for id := range s.webhookIDs {
		ids = append(ids, id)
	}
	b, err := json.Marshal(persistedSubjectState{Blocked: s.blocked, WebhookIDs: ids})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".balance-state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func subjectKey(subjectType, subjectID string) string {
	subjectType = strings.TrimSpace(subjectType)
	subjectID = strings.TrimSpace(subjectID)
	if subjectType == "" || subjectID == "" {
		return ""
	}
	return subjectType + "/" + subjectID
}
