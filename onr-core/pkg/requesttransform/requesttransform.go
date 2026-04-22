package requesttransform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
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

// Apply transforms a request body and request value using request transform rules.
// meta must be non-nil.
func Apply(meta *dslmeta.Meta, contentType string, body []byte, value any, t dslconfig.RequestTransform, opts ApplyOptions) (Result, error) {
	result := Result{
		Body:        body,
		Value:       value,
		ContentType: strings.TrimSpace(contentType),
	}

	out := value
	changedByModelRewrite := false
	modelMapped := strings.TrimSpace(meta.DSLModelMapped)
	if root, ok := out.(map[string]any); ok && root != nil {
		if modelMapped != "" {
			if _, exists := root["model"]; exists {
				if current, ok := root["model"].(string); !ok || current != modelMapped {
					root["model"] = modelMapped
					changedByModelRewrite = true
				}
			}
		}
		result.Root = root
	}

	changedByJSONOps := false
	if out != nil && len(t.JSONOps) > 0 {
		var err error
		out, err = dslconfig.ApplyJSONOps(meta, out, t.JSONOps)
		if err != nil {
			return Result{}, err
		}
		changedByJSONOps = true
		result.Value = out
		if root, ok := out.(map[string]any); ok {
			result.Root = root
		} else {
			result.Root = nil
		}
	}

	reqBody := result.Body
	if changedByModelRewrite || changedByJSONOps || (reqBody == nil && out != nil) {
		var err error
		reqBody, err = marshalBody(result.Body, result.Root, result.ContentType, out)
		if err != nil {
			return Result{}, err
		}
		result.Body = reqBody
	}

	if strings.TrimSpace(t.ReqMapMode) == "" {
		return result, nil
	}

	mappedBody, mappedAny, err := ApplyReqMap(t.ReqMapMode, reqBody, out, opts)
	if err != nil {
		return Result{}, err
	}
	result.Body = mappedBody

	result.Value = mappedAny
	if root, ok := mappedAny.(map[string]any); ok {
		result.Root = root
	} else {
		result.Root = nil
	}
	return result, nil
}

func ApplyReqMap(mode string, raw []byte, out any, opts ApplyOptions) ([]byte, any, error) {
	ce := strings.ToLower(strings.TrimSpace(opts.ContentEncoding))
	if ce != "" && ce != contentEncodingIdentity {
		return nil, nil, fmt.Errorf("cannot transform encoded client request (Content-Encoding=%q)", opts.ContentEncoding)
	}
	if root, ok := out.(map[string]any); ok && root != nil {
		return applyReqMapObject(mode, apitypes.JSONObject(root))
	}
	root, err := parseReqMapInputObject(mode, raw)
	if err != nil {
		return nil, nil, err
	}
	return applyReqMapObject(mode, root)
}

func applyReqMapObject(mode string, root apitypes.JSONObject) ([]byte, any, error) {
	var (
		mappedObj apitypes.JSONObject
		err       error
	)
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "openai_chat_to_openai_responses":
		mappedObj, err = apitransform.MapOpenAIChatCompletionsToResponsesObject(root)
	case "openai_chat_to_anthropic_messages":
		mappedObj, err = apitransform.MapOpenAIChatCompletionsToClaudeMessagesRequestObject(root)
	case "openai_chat_to_gemini_generate_content":
		mappedObj, err = apitransform.MapOpenAIChatCompletionsToGeminiGenerateContentRequestObject(root)
	case "anthropic_to_openai_chat":
		mappedObj, err = apitransform.MapClaudeMessagesToOpenAIChatCompletionsObject(root)
	case "gemini_to_openai_chat":
		mappedObj, err = apitransform.MapGeminiGenerateContentToOpenAIChatCompletionsObject(root)
	default:
		return nil, nil, fmt.Errorf("unsupported req_map mode %q", mode)
	}
	if err != nil {
		return nil, nil, err
	}
	mappedBody, err := json.Marshal(mappedObj)
	if err != nil {
		return nil, nil, err
	}
	return mappedBody, map[string]any(mappedObj), nil
}

func parseReqMapInputObject(mode string, raw []byte) (apitypes.JSONObject, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "openai_chat_to_openai_responses", "openai_chat_to_anthropic_messages", "openai_chat_to_gemini_generate_content":
	case "anthropic_to_openai_chat":
	case "gemini_to_openai_chat":
	default:
		return nil, fmt.Errorf("unsupported req_map mode %q", mode)
	}
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("parse json object: %w", err)
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return nil, fmt.Errorf("json is not an object")
	}
	return apitypes.JSONObject(root), nil
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
