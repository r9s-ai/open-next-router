package meterry

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/r9s-ai/open-next-router/pkg/controlplane"
)

type redisOutbox struct {
	controlPlane *controlplane.Client
}

func (o *redisOutbox) append(event Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = o.controlPlane.EnqueueBillingEvent(context.Background(), raw)
	return err
}

func (o *redisOutbox) first() (Event, string, error) {
	messages, err := o.controlPlane.AutoClaimBilling(context.Background(), time.Second, 1)
	if err == nil && len(messages) == 0 {
		messages, err = o.controlPlane.ReadPendingBilling(context.Background(), 1)
	}
	if err == nil && len(messages) == 0 {
		messages, err = o.controlPlane.ReadBilling(context.Background(), 1, time.Second)
	}
	if err != nil {
		return Event{}, "", err
	}
	if len(messages) == 0 {
		return Event{}, "", io.EOF
	}
	var event Event
	if err := json.Unmarshal([]byte(messages[0].Values["payload"]), &event); err != nil {
		return Event{}, messages[0].ID, err
	}
	return event, messages[0].ID, nil
}

func (o *redisOutbox) ack(token string) error {
	return o.controlPlane.AckBilling(context.Background(), token)
}

func (o *redisOutbox) fail(token string, event Event) error {
	attempts, err := o.controlPlane.IncrementBillingAttempt(context.Background(), token)
	if err != nil {
		return err
	}
	if attempts < int64(o.controlPlane.BillingMaxAttempts()) {
		return nil
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return o.controlPlane.DeadLetterBilling(context.Background(), token, raw)
}
