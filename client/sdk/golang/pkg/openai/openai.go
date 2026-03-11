package openai

import (
	"context"

	"github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"

	"github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/config"
)

type EmbeddingResult struct {
	Object       string
	Model        string
	Dimensions   int
	Embedding    []float64
	PromptTokens int64
	TotalTokens  int64
}

type clientFactory func(config.ClientConfig) openai.Client

var newClient = func(cfg config.ClientConfig) openai.Client {
	return openai.NewClient(
		openaioption.WithAPIKey(cfg.APIKey),
		openaioption.WithBaseURL(cfg.BaseURL+"/v1"),
	)
}

func Chat(ctx context.Context, cfg config.ClientConfig, prompt, model string) (string, error) {
	client := newClient(cfg)
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", err
	}
	return chatCompletionText(resp), nil
}

func StreamChat(ctx context.Context, cfg config.ClientConfig, prompt, model string) (<-chan string, <-chan error) {
	out := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		client := newClient(cfg)
		stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model: openai.ChatModel(model),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(prompt),
			},
		})
		for stream.Next() {
			chunk := stream.Current()
			if text := chatCompletionChunkText(chunk); text != "" {
				out <- text
			}
		}
		if err := stream.Err(); err != nil {
			errc <- err
		}
	}()

	return out, errc
}

func CreateResponse(ctx context.Context, cfg config.ClientConfig, prompt, model string) (string, error) {
	client := newClient(cfg)
	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: openai.ChatModel(model),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
	})
	if err != nil {
		return "", err
	}
	return resp.OutputText(), nil
}

func StreamResponse(ctx context.Context, cfg config.ClientConfig, prompt, model string) (<-chan string, <-chan error) {
	out := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		client := newClient(cfg)
		stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
			Model: openai.ChatModel(model),
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(prompt),
			},
		})
		for stream.Next() {
			event := stream.Current()
			if text := responseDeltaText(event); text != "" {
				out <- text
			}
		}
		if err := stream.Err(); err != nil {
			errc <- err
		}
	}()

	return out, errc
}

func CreateEmbedding(ctx context.Context, cfg config.ClientConfig, text, model string) (EmbeddingResult, error) {
	client := newClient(cfg)
	resp, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
	})
	if err != nil {
		return EmbeddingResult{}, err
	}
	return embeddingResultFromResponse(resp, model), nil
}

func chatCompletionText(resp *openai.ChatCompletion) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

func chatCompletionChunkText(chunk openai.ChatCompletionChunk) string {
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

func responseDeltaText(event responses.ResponseStreamEventUnion) string {
	if event.Type != "response.output_text.delta" {
		return ""
	}
	return event.Delta
}

func embeddingResultFromResponse(resp *openai.CreateEmbeddingResponse, model string) EmbeddingResult {
	if resp == nil || len(resp.Data) == 0 {
		return EmbeddingResult{Object: "embedding", Model: model}
	}
	return EmbeddingResult{
		Object:       "embedding",
		Model:        model,
		Dimensions:   len(resp.Data[0].Embedding),
		Embedding:    resp.Data[0].Embedding,
		PromptTokens: resp.Usage.PromptTokens,
		TotalTokens:  resp.Usage.TotalTokens,
	}
}
