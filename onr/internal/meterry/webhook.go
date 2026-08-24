package meterry

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	sdk "github.com/meterry-com/meterry-go"
)

func VerifyWebhookSignature(secret, timestamp, signature string, body []byte, now time.Time, tolerance time.Duration) error {
	secret = strings.TrimSpace(secret)
	timestamp = strings.TrimSpace(timestamp)
	signature = strings.TrimSpace(signature)
	if secret == "" || timestamp == "" || signature == "" {
		return errors.New("missing Meterry webhook signature fields")
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid Meterry webhook timestamp")
	}
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	if delta := now.Sub(time.Unix(unix, 0)); delta > tolerance || delta < -tolerance {
		return errors.New("expired Meterry webhook timestamp")
	}
	if err := sdk.VerifyWebhookSignature(secret, signature, unix, body); err != nil {
		return err
	}
	return nil
}

type WebhookEvent struct {
	ID        string         `json:"id"`
	EventID   string         `json:"event_id"`
	Type      string         `json:"type"`
	EventType string         `json:"event_type"`
	EventName string         `json:"event_name"`
	Data      map[string]any `json:"data"`
}

func ParseWebhookEvent(body []byte) (WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return WebhookEvent{}, fmt.Errorf("decode Meterry webhook: %w", err)
	}
	if strings.TrimSpace(event.ID) == "" {
		event.ID = strings.TrimSpace(event.EventID)
	}
	if strings.TrimSpace(event.Type) == "" {
		event.Type = strings.TrimSpace(event.EventType)
	}
	if strings.TrimSpace(event.Type) == "" {
		event.Type = strings.TrimSpace(event.EventName)
	}
	if strings.TrimSpace(event.Type) == "" && event.Data != nil {
		event.Type = stringValue(event.Data, "event_type")
		if event.Type == "" {
			event.Type = stringValue(event.Data, "event_name")
		}
	}
	if event.Type == "" {
		return WebhookEvent{}, errors.New("meterry webhook requires event type")
	}
	return event, nil
}

func (e WebhookEvent) Subject() (subjectType, subjectID string) {
	var subject map[string]any
	if e.Data != nil {
		if v, ok := e.Data["subject"].(map[string]any); ok {
			subject = v
		}
	}
	if subject == nil {
		subject = e.Data
	}
	return stringValue(subject, "subject_type"), stringValue(subject, "subject_id")
}

func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}
