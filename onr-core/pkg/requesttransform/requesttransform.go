package requesttransform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
)

const contentEncodingIdentity = "identity"

type ApplyOptions struct {
	ContentEncoding string
}

type Result struct {
	Body        []byte
	Value       any
	Root        map[string]any
	ContentType string
}

func Apply(meta *dslmeta.Meta, contentType string, body []byte, value any, t dslconfig.RequestTransform, opts ApplyOptions) (Result, error) {
	result := Result{
		Body:        body,
		Value:       value,
		ContentType: strings.TrimSpace(contentType),
	}

	out := value
	if root, ok := out.(map[string]any); ok && root != nil {
		if meta != nil && strings.TrimSpace(meta.DSLModelMapped) != "" {
			if _, exists := root["model"]; exists {
				root["model"] = strings.TrimSpace(meta.DSLModelMapped)
			}
		}
		result.Root = root
	}

	if out != nil && len(t.JSONOps) > 0 {
		var err error
		out, err = dslconfig.ApplyJSONOps(meta, out, t.JSONOps)
		if err != nil {
			return Result{}, err
		}
		result.Value = out
		if root, ok := out.(map[string]any); ok {
			result.Root = root
		} else {
			result.Root = nil
		}
	}

	reqBody, err := marshalBody(result.Body, result.Root, result.ContentType, out)
	if err != nil {
		return Result{}, err
	}
	result.Body = reqBody

	if strings.TrimSpace(t.ReqMapMode) == "" {
		return result, nil
	}

	mappedBody, err := ApplyReqMap(t.ReqMapMode, reqBody, opts)
	if err != nil {
		return Result{}, err
	}
	result.Body = mappedBody

	var mappedAny any
	if err := json.Unmarshal(mappedBody, &mappedAny); err != nil {
		return Result{}, err
	}
	result.Value = mappedAny
	if root, ok := mappedAny.(map[string]any); ok {
		result.Root = root
	} else {
		result.Root = nil
	}
	return result, nil
}

func ApplyReqMap(mode string, raw []byte, opts ApplyOptions) ([]byte, error) {
	ce := strings.ToLower(strings.TrimSpace(opts.ContentEncoding))
	if ce != "" && ce != contentEncodingIdentity {
		return nil, fmt.Errorf("cannot transform encoded client request (Content-Encoding=%q)", opts.ContentEncoding)
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "openai_chat_to_openai_responses":
		return apitransform.MapOpenAIChatCompletionsToResponsesRequest(raw)
	case "openai_chat_to_anthropic_messages":
		return apitransform.MapOpenAIChatCompletionsToClaudeMessagesRequest(raw)
	case "openai_chat_to_gemini_generate_content":
		return apitransform.MapOpenAIChatCompletionsToGeminiGenerateContentRequest(raw)
	case "anthropic_to_openai_chat":
		return apitransform.MapClaudeMessagesToOpenAIChatCompletions(raw)
	case "gemini_to_openai_chat":
		return apitransform.MapGeminiGenerateContentToOpenAIChatCompletions(raw)
	default:
		return nil, fmt.Errorf("unsupported req_map mode %q", mode)
	}
}

func marshalBody(originalBody []byte, root map[string]any, contentType string, value any) ([]byte, error) {
	if root == nil {
		if value == nil {
			return originalBody, nil
		}
		if requestcanon.IsMultipartFormData(contentType) {
			return originalBody, nil
		}
		return json.Marshal(value)
	}
	if requestcanon.IsMultipartFormData(contentType) {
		return originalBody, nil
	}
	return json.Marshal(root)
}
