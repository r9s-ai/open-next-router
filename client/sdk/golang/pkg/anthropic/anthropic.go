package anthropic

import (
	"context"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/config"
)

var newClient = func(cfg config.ClientConfig) anthropic.Client {
	return anthropic.NewClient(
		anthropicoption.WithAPIKey(cfg.APIKey),
		anthropicoption.WithBaseURL(cfg.BaseURL),
	)
}

func Chat(ctx context.Context, cfg config.ClientConfig, prompt, model string, maxTokens int64) (string, error) {
	client := newClient(cfg)
	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", err
	}
	return messageText(msg), nil
}

func StreamChat(ctx context.Context, cfg config.ClientConfig, prompt, model string, maxTokens int64) (<-chan string, <-chan error) {
	out := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		client := newClient(cfg)
		stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: maxTokens,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
			},
		})

		for stream.Next() {
			event := stream.Current()
			switch v := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				if text := contentBlockDeltaText(v); text != "" {
					out <- text
				}
			}
		}
		if err := stream.Err(); err != nil {
			errc <- err
		}
	}()

	return out, errc
}

func messageText(msg *anthropic.Message) string {
	if msg == nil {
		return ""
	}
	for _, block := range msg.Content {
		if block.Text != "" {
			return block.Text
		}
	}
	return ""
}

func contentBlockDeltaText(event anthropic.ContentBlockDeltaEvent) string {
	if delta, ok := event.Delta.AsAny().(anthropic.TextDelta); ok && delta.Text != "" {
		return delta.Text
	}
	if event.Delta.Type == "text_delta" {
		return event.Delta.Text
	}
	return ""
}
