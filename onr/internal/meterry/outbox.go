package meterry

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type outbox struct {
	mu   sync.Mutex
	path string
}

type eventOutbox interface {
	append(event Event) error
	first() (Event, string, error)
	ack(token string) error
	fail(token string, event Event) error
}

func openOutbox(dir string) (*outbox, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("meterry outbox directory is empty")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create meterry outbox directory: %w", err)
	}
	return &outbox{path: filepath.Join(dir, "events.jsonl")}, nil
}

func (o *outbox) append(event Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	f, err := os.OpenFile(o.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	b, err := json.Marshal(event)
	if err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (o *outbox) first() (Event, string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	f, err := os.Open(o.path)
	if errors.Is(err, os.ErrNotExist) {
		return Event{}, "", io.EOF
	}
	if err != nil {
		return Event{}, "", err
	}
	s := bufio.NewScanner(f)
	if !s.Scan() {
		if err := s.Err(); err != nil {
			_ = f.Close()
			return Event{}, "", err
		}
		if err := f.Close(); err != nil {
			return Event{}, "", err
		}
		return Event{}, "", io.EOF
	}
	var event Event
	if err := json.Unmarshal(s.Bytes(), &event); err != nil {
		_ = f.Close()
		return Event{}, "", fmt.Errorf("decode meterry outbox event: %w", err)
	}
	if err := f.Close(); err != nil {
		return Event{}, "", err
	}
	return event, event.IdempotencyKey, nil
}

func (o *outbox) ack(idempotencyKey string) (retErr error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	f, err := os.Open(o.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); retErr == nil && closeErr != nil {
			retErr = closeErr
		}
	}()
	tmp, err := os.CreateTemp(filepath.Dir(o.path), ".events-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepFirst := true
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Bytes()
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("decode meterry outbox event while acking: %w", err)
		}
		if keepFirst && event.IdempotencyKey == idempotencyKey {
			keepFirst = false
			continue
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := s.Err(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, o.path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (o *outbox) fail(token string, event Event) error { return nil }
