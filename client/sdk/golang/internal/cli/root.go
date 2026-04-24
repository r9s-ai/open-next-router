package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	anthropicprovider "github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/anthropic"
	"github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/config"
	geminiprovider "github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/gemini"
	"github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/metrics"
	openaiprovider "github.com/r9s-ai/open-next-router/client/sdk/golang/pkg/openai"
)

const (
	defaultOpenAIModel                = "gpt-4o-mini"
	defaultOpenAIEmbeddingModel       = "text-embedding-3-small"
	defaultAnthropicModel             = "claude-haiku-4-5"
	defaultGeminiModel                = "gemini-2.5-flash"
	defaultGeminiChatsModel           = "gemini-3-pro-image-preview"
	defaultAnthropicMaxTokens   int64 = 1024
)

type streams struct {
	stdout io.Writer
	stderr io.Writer
}

var (
	openAIChatFunc           = openaiprovider.Chat
	openAIStreamChatFunc     = openaiprovider.StreamChat
	openAIResponseFunc       = openaiprovider.CreateResponse
	openAIStreamResponseFunc = openaiprovider.StreamResponse
	openAIEmbeddingFunc      = openaiprovider.CreateEmbedding
	anthropicChatFunc        = anthropicprovider.Chat
	anthropicStreamChatFunc  = anthropicprovider.StreamChat
	geminiChatFunc           = geminiprovider.Chat
	geminiStreamChatFunc     = geminiprovider.StreamChat
	geminiMultimodalChatFunc = geminiprovider.StreamChatMultimodal
)

// NewRootCommand returns a non-nil root command.
func NewRootCommand() *cobra.Command {
	return NewRootCommandWithWriters(os.Stdout, os.Stderr)
}

// NewRootCommandWithWriters returns a non-nil root command.
func NewRootCommandWithWriters(stdout, stderr io.Writer) *cobra.Command {
	s := streams{stdout: stdout, stderr: stderr}
	root := &cobra.Command{
		Use:           "onr-sdk",
		Short:         "ONR SDK CLI - Unified interface for OpenAI, Anthropic, Gemini",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(s.stdout)
	root.SetErr(s.stderr)

	root.AddCommand(newCompletionCommand(root))
	root.AddCommand(newOpenAICommand(s))
	root.AddCommand(newAnthropicCommand(s))
	root.AddCommand(newGeminiCommand(s))
	return root
}

// newCompletionCommand returns a non-nil completion subcommand.
func newCompletionCommand(root *cobra.Command) *cobra.Command {
	var shell string
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Print shell completion script",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch shell {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return fmt.Errorf("unsupported shell: %s", shell)
			}
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "", "Shell type")
	_ = cmd.MarkFlagRequired("shell")
	return cmd
}

// newOpenAICommand returns a non-nil OpenAI subcommand.
func newOpenAICommand(s streams) *cobra.Command {
	cmd := &cobra.Command{Use: "openai", Short: "OpenAI API"}
	cmd.AddCommand(newOpenAIChatCommand(s))
	cmd.AddCommand(newOpenAIResponsesCommand(s))
	cmd.AddCommand(newOpenAIEmbeddingsCommand(s))
	return cmd
}

// newOpenAIChatCommand returns a non-nil chat subcommand.
func newOpenAIChatCommand(s streams) *cobra.Command {
	var model string
	var stream bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "chat_completions <prompt>",
		Short: "OpenAI chat completions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]
			cfg, err := config.FromEnv()
			if err != nil {
				return userError(err)
			}

			start := time.Now()
			textChars := 0
			status := "ok"
			exception := ""
			defer func() {
				if verbose {
					metrics.Print(s.stdout, metrics.RequestMetrics{
						Provider:         "openai/chat_completions",
						Model:            model,
						BaseURL:          cfg.BaseURL,
						Stream:           stream,
						ElapsedSec:       time.Since(start).Seconds(),
						TextChars:        textChars,
						Status:           status,
						ExceptionMessage: exception,
					})
				}
			}()

			if stream {
				out, errc := openAIStreamChatFunc(cmd.Context(), cfg, prompt, model)
				for text := range out {
					textChars += len(text)
					_, _ = fmt.Fprint(s.stdout, text)
				}
				if err := <-errc; err != nil {
					status = "error"
					exception = sanitizeError(err).Error()
					return sanitizeError(err)
				}
				_, _ = fmt.Fprintln(s.stdout)
				return nil
			}

			resp, err := openAIChatFunc(cmd.Context(), cfg, prompt, model)
			if err != nil {
				status = "error"
				exception = sanitizeError(err).Error()
				return sanitizeError(err)
			}
			textChars = len(resp)
			_, _ = fmt.Fprintln(s.stdout, resp)
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", defaultOpenAIModel, "Model name")
	cmd.Flags().BoolVar(&stream, "stream", false, "Enable streaming")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print request metrics")
	return cmd
}

// newOpenAIResponsesCommand returns a non-nil responses subcommand.
func newOpenAIResponsesCommand(s streams) *cobra.Command {
	var model string
	var stream bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "responses <prompt>",
		Short: "OpenAI responses",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]
			cfg, err := config.FromEnv()
			if err != nil {
				return userError(err)
			}

			start := time.Now()
			textChars := 0
			status := "ok"
			exception := ""
			defer func() {
				if verbose {
					metrics.Print(s.stdout, metrics.RequestMetrics{
						Provider:         "openai/responses",
						Model:            model,
						BaseURL:          cfg.BaseURL,
						Stream:           stream,
						ElapsedSec:       time.Since(start).Seconds(),
						TextChars:        textChars,
						Status:           status,
						ExceptionMessage: exception,
					})
				}
			}()

			if stream {
				out, errc := openAIStreamResponseFunc(cmd.Context(), cfg, prompt, model)
				for text := range out {
					textChars += len(text)
					_, _ = fmt.Fprint(s.stdout, text)
				}
				if err := <-errc; err != nil {
					status = "error"
					exception = sanitizeError(err).Error()
					return sanitizeError(err)
				}
				_, _ = fmt.Fprintln(s.stdout)
				return nil
			}

			resp, err := openAIResponseFunc(cmd.Context(), cfg, prompt, model)
			if err != nil {
				status = "error"
				exception = sanitizeError(err).Error()
				return sanitizeError(err)
			}
			textChars = len(resp)
			_, _ = fmt.Fprintln(s.stdout, resp)
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", defaultOpenAIModel, "Model name")
	cmd.Flags().BoolVar(&stream, "stream", false, "Enable streaming")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print request metrics")
	return cmd
}

// newOpenAIEmbeddingsCommand returns a non-nil embeddings subcommand.
func newOpenAIEmbeddingsCommand(s streams) *cobra.Command {
	var model string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "embeddings <text>",
		Short: "OpenAI embeddings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromEnv()
			if err != nil {
				return userError(err)
			}
			start := time.Now()
			textChars := 0
			status := "ok"
			exception := ""
			defer func() {
				if verbose {
					metrics.Print(s.stdout, metrics.RequestMetrics{
						Provider:         "openai/embeddings",
						Model:            model,
						BaseURL:          cfg.BaseURL,
						ElapsedSec:       time.Since(start).Seconds(),
						TextChars:        textChars,
						Status:           status,
						ExceptionMessage: exception,
					})
				}
			}()

			resp, err := openAIEmbeddingFunc(cmd.Context(), cfg, args[0], model)
			if err != nil {
				status = "error"
				exception = sanitizeError(err).Error()
				return sanitizeError(err)
			}
			previewLen := 8
			if len(resp.Embedding) < previewLen {
				previewLen = len(resp.Embedding)
			}
			preview := resp.Embedding[:previewLen]
			textChars = len(fmt.Sprint(preview))
			_, _ = fmt.Fprintf(s.stdout, "object: %s\n", resp.Object)
			_, _ = fmt.Fprintf(s.stdout, "model: %s\n", resp.Model)
			_, _ = fmt.Fprintf(s.stdout, "dimensions: %d\n", resp.Dimensions)
			_, _ = fmt.Fprintf(s.stdout, "prompt_tokens: %d\n", resp.PromptTokens)
			_, _ = fmt.Fprintf(s.stdout, "total_tokens: %d\n", resp.TotalTokens)
			_, _ = fmt.Fprintf(s.stdout, "embedding_preview: %v\n", preview)
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", defaultOpenAIEmbeddingModel, "Model name")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print request metrics")
	return cmd
}

// newAnthropicCommand returns a non-nil Anthropic subcommand.
func newAnthropicCommand(s streams) *cobra.Command {
	cmd := &cobra.Command{Use: "anthropic", Short: "Anthropic API"}
	cmd.AddCommand(newAnthropicMessagesCommand(s))
	return cmd
}

// newAnthropicMessagesCommand returns a non-nil messages subcommand.
func newAnthropicMessagesCommand(s streams) *cobra.Command {
	var model string
	var stream bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "messages <prompt>",
		Short: "Anthropic messages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromEnv()
			if err != nil {
				return userError(err)
			}
			start := time.Now()
			textChars := 0
			status := "ok"
			exception := ""
			defer func() {
				if verbose {
					metrics.Print(s.stdout, metrics.RequestMetrics{
						Provider:         "anthropic/messages",
						Model:            model,
						BaseURL:          cfg.BaseURL,
						Stream:           stream,
						ElapsedSec:       time.Since(start).Seconds(),
						TextChars:        textChars,
						Status:           status,
						ExceptionMessage: exception,
					})
				}
			}()

			if stream {
				out, errc := anthropicStreamChatFunc(cmd.Context(), cfg, args[0], model, defaultAnthropicMaxTokens)
				for text := range out {
					textChars += len(text)
					_, _ = fmt.Fprint(s.stdout, text)
				}
				if err := <-errc; err != nil {
					status = "error"
					exception = sanitizeError(err).Error()
					return sanitizeError(err)
				}
				_, _ = fmt.Fprintln(s.stdout)
				return nil
			}

			resp, err := anthropicChatFunc(cmd.Context(), cfg, args[0], model, defaultAnthropicMaxTokens)
			if err != nil {
				status = "error"
				exception = sanitizeError(err).Error()
				return sanitizeError(err)
			}
			textChars = len(resp)
			_, _ = fmt.Fprintln(s.stdout, resp)
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", defaultAnthropicModel, "Model name")
	cmd.Flags().BoolVar(&stream, "stream", false, "Enable streaming")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print request metrics")
	return cmd
}

// newGeminiCommand returns a non-nil Gemini subcommand.
func newGeminiCommand(s streams) *cobra.Command {
	cmd := &cobra.Command{Use: "gemini", Short: "Google Gemini API"}
	cmd.AddCommand(newGeminiModelsCommand(s))
	cmd.AddCommand(newGeminiChatsCommand(s))
	return cmd
}

// newGeminiModelsCommand returns a non-nil models subcommand.
func newGeminiModelsCommand(s streams) *cobra.Command {
	var model string
	var stream bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "models <prompt>",
		Short: "Gemini models.generate_content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromEnv()
			if err != nil {
				return userError(err)
			}
			start := time.Now()
			textChars := 0
			status := "ok"
			exception := ""
			defer func() {
				if verbose {
					metrics.Print(s.stdout, metrics.RequestMetrics{
						Provider:         "gemini/models",
						Model:            model,
						BaseURL:          cfg.BaseURL,
						Stream:           stream,
						ElapsedSec:       time.Since(start).Seconds(),
						TextChars:        textChars,
						Status:           status,
						ExceptionMessage: exception,
					})
				}
			}()
			if stream {
				out, errc := geminiStreamChatFunc(cmd.Context(), cfg, args[0], model)
				for text := range out {
					textChars += len(text)
					_, _ = fmt.Fprint(s.stdout, text)
				}
				if err := <-errc; err != nil {
					status = "error"
					exception = sanitizeError(err).Error()
					return sanitizeError(err)
				}
				_, _ = fmt.Fprintln(s.stdout)
				return nil
			}

			resp, err := geminiChatFunc(cmd.Context(), cfg, args[0], model)
			if err != nil {
				status = "error"
				exception = sanitizeError(err).Error()
				return sanitizeError(err)
			}
			textChars = len(resp)
			_, _ = fmt.Fprintln(s.stdout, resp)
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", defaultGeminiModel, "Model name")
	cmd.Flags().BoolVar(&stream, "stream", false, "Enable streaming")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print request metrics")
	return cmd
}

// newGeminiChatsCommand returns a non-nil chats subcommand.
func newGeminiChatsCommand(s streams) *cobra.Command {
	var model string
	var modalities string
	var outputDir string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "chats <prompt>",
		Short: "Gemini chats.send_message_stream",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromEnv()
			if err != nil {
				return userError(err)
			}
			start := time.Now()
			textChars := 0
			imageCount := 0
			status := "ok"
			exception := ""
			defer func() {
				if verbose {
					metrics.Print(s.stdout, metrics.RequestMetrics{
						Provider:         "gemini/chats",
						Model:            model,
						BaseURL:          cfg.BaseURL,
						Stream:           true,
						ElapsedSec:       time.Since(start).Seconds(),
						TextChars:        textChars,
						ImageCount:       imageCount,
						Status:           status,
						ExceptionMessage: exception,
					})
				}
			}()

			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				status = "error"
				exception = sanitizeError(err).Error()
				return sanitizeError(err)
			}

			parts := splitModalities(modalities)
			out, errc := geminiMultimodalChatFunc(cmd.Context(), cfg, args[0], model, parts)
			for event := range out {
				switch event.Type {
				case "text":
					textChars += len(event.Text)
					_, _ = fmt.Fprint(s.stdout, event.Text)
				case "image":
					imageCount++
					imagePath := filepath.Join(outputDir, fmt.Sprintf("gemini_%d%s", imageCount, imageExt(event.ImageMIMEType)))
					if err := os.WriteFile(imagePath, event.ImageBytes, 0o644); err != nil {
						status = "error"
						exception = sanitizeError(err).Error()
						return sanitizeError(err)
					}
					_, _ = fmt.Fprintf(s.stdout, "\n[image saved] %s\n", imagePath)
				}
			}
			if err := <-errc; err != nil {
				status = "error"
				exception = sanitizeError(err).Error()
				return sanitizeError(err)
			}
			_, _ = fmt.Fprintln(s.stdout)
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", defaultGeminiChatsModel, "Model name")
	cmd.Flags().StringVar(&modalities, "response_modalities", "TEXT,IMAGE", "Comma-separated response modalities")
	cmd.Flags().StringVar(&outputDir, "image-output-dir", ".", "Directory to save generated images")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print request metrics")
	return cmd
}

func ExecuteContext(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	root := NewRootCommandWithWriters(stdout, stderr)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetContext(ctx)
	return root.Execute()
}

func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("request failed: %s", err)
}

func userError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	return fmt.Errorf("error: %s", err)
}

func splitModalities(raw string) []string {
	items := strings.Split(raw, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func imageExt(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}
