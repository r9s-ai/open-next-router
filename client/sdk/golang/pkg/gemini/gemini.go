package gemini

import (
	"context"
	"strings"

	"google.golang.org/genai"

	"github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/config"
)

type StreamEvent struct {
	Type          string
	Text          string
	ImageBytes    []byte
	ImageMIMEType string
}

var newClient = func(ctx context.Context, cfg config.ClientConfig) (*genai.Client, error) {
	return genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: strings.TrimRight(cfg.BaseURL, "/"),
		},
	})
}

func Chat(ctx context.Context, cfg config.ClientConfig, prompt, model string) (string, error) {
	client, err := newClient(ctx, cfg)
	if err != nil {
		return "", err
	}
	resp, err := client.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return "", err
	}
	return resp.Text(), nil
}

func StreamChat(ctx context.Context, cfg config.ClientConfig, prompt, model string) (<-chan string, <-chan error) {
	out := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		client, err := newClient(ctx, cfg)
		if err != nil {
			errc <- err
			return
		}
		for chunk, err := range client.Models.GenerateContentStream(ctx, model, genai.Text(prompt), nil) {
			if err != nil {
				errc <- err
				return
			}
			if text := chunk.Text(); text != "" {
				out <- text
			}
		}
	}()

	return out, errc
}

func StreamChatMultimodal(ctx context.Context, cfg config.ClientConfig, prompt, model string, modalities []string) (<-chan StreamEvent, <-chan error) {
	out := make(chan StreamEvent)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		client, err := newClient(ctx, cfg)
		if err != nil {
			errc <- err
			return
		}

		chat, err := client.Chats.Create(ctx, model, &genai.GenerateContentConfig{
			ResponseModalities: modalities,
		}, nil)
		if err != nil {
			errc <- err
			return
		}

		for chunk, err := range chat.SendMessageStream(ctx, genai.Part{Text: prompt}) {
			if err != nil {
				errc <- err
				return
			}
			for _, event := range responseEvents(chunk) {
				out <- event
			}
		}
	}()

	return out, errc
}

func responseEvents(resp *genai.GenerateContentResponse) []StreamEvent {
	if resp == nil {
		return nil
	}
	var out []StreamEvent
	for _, candidate := range resp.Candidates {
		if candidate == nil || candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part == nil {
				continue
			}
			if part.Text != "" {
				out = append(out, StreamEvent{Type: "text", Text: part.Text})
			}
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				out = append(out, StreamEvent{
					Type:          "image",
					ImageBytes:    part.InlineData.Data,
					ImageMIMEType: part.InlineData.MIMEType,
				})
			}
		}
	}
	return out
}
