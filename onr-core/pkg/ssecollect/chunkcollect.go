package ssecollect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

var (
	ErrInvalidChunkPayload = errors.New("sse chunk collect: invalid payload")
	ErrUnsupportedMode     = errors.New("sse chunk collect: unsupported mode")
)

type ChunkCollector interface {
	AddEvent([]byte) error
	Snapshot() (map[string]any, bool)
}

type ChunkCollecter = ChunkCollector

func NewChunkCollector(mode string) (ChunkCollector, error) {
	switch NormalizeMode(mode) {
	case "openai_chat_completions", "chat.completions":
		return newOpenAIChatChunkCollector(), nil
	case "openai_responses", "responses":
		return newOpenAIResponsesChunkCollector(), nil
	case "anthropic_messages", "claude.messages":
		return newAnthropicMessagesChunkCollector(), nil
	case "gemini_generate_content", "gemini.stream_generate_content", "gemini.streamgeneratecontent":
		return newGeminiGenerateContentChunkCollector(), nil
	default:
		return nil, fmt.Errorf("%w %q", ErrUnsupportedMode, mode)
	}
}

func NewChunkCollecter(mode string) (ChunkCollecter, error) {
	return NewChunkCollector(mode)
}

func parseChunkObject(payload []byte) (map[string]any, bool, error) {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil, true, nil
	}
	var root any
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrInvalidChunkPayload, err)
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("%w: top-level payload must be an object", ErrInvalidChunkPayload)
	}
	if _, ok := obj["error"].(map[string]any); ok {
		return nil, false, upstreamError(obj)
	}
	return obj, false, nil
}

type openAIChatChunkCollector struct {
	top     map[string]any
	usage   map[string]any
	choices map[int]*chatChoiceState
}

type chatChoiceState struct {
	index           int
	message         map[string]any
	content         strings.Builder
	refusal         strings.Builder
	reasoning       strings.Builder
	functionName    string
	functionArgs    strings.Builder
	toolCalls       map[int]*chatToolCallState
	finishReasonSet bool
	finishReason    any
}

type chatToolCallState struct {
	index     int
	id        string
	typ       string
	name      string
	arguments strings.Builder
}

func newOpenAIChatChunkCollector() *openAIChatChunkCollector {
	return &openAIChatChunkCollector{
		top:     map[string]any{"object": "chat.completion"},
		choices: map[int]*chatChoiceState{},
	}
}

func (c *openAIChatChunkCollector) AddEvent(payload []byte) error {
	root, done, err := parseChunkObject(payload)
	if err != nil || done {
		return err
	}
	for _, key := range []string{"id", "created", "model", "system_fingerprint", "service_tier"} {
		if v, ok := root[key]; ok {
			c.top[key] = v
		}
	}
	if usage, _ := root["usage"].(map[string]any); usage != nil {
		c.usage = cloneMap(usage)
	}

	rawChoices, hasChoices := root["choices"]
	if !hasChoices {
		if c.usage != nil {
			return nil
		}
		return fmt.Errorf("%w: chat.completions chunk missing choices", ErrInvalidChunkPayload)
	}
	choices, ok := rawChoices.([]any)
	if !ok {
		return fmt.Errorf("%w: chat.completions choices must be an array", ErrInvalidChunkPayload)
	}
	for pos, raw := range choices {
		choice, _ := raw.(map[string]any)
		if choice == nil {
			return fmt.Errorf("%w: chat.completions choice must be an object", ErrInvalidChunkPayload)
		}
		idx, err := boundedChunkIndex(choice["index"], pos, "choices.index")
		if err != nil {
			return err
		}
		state := c.choice(idx)
		if v, ok := choice["finish_reason"]; ok && v != nil {
			state.finishReasonSet = true
			state.finishReason = v
		}
		delta, _ := choice["delta"].(map[string]any)
		if delta == nil {
			continue
		}
		if err := state.applyDelta(delta); err != nil {
			return err
		}
	}
	return nil
}

func (c *openAIChatChunkCollector) Snapshot() (map[string]any, bool) {
	if len(c.choices) == 0 && c.usage == nil && len(c.top) <= 1 {
		return nil, false
	}
	out := cloneMap(c.top)
	indexes := make([]int, 0, len(c.choices))
	for idx := range c.choices {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	choices := make([]any, 0, len(indexes))
	for _, idx := range indexes {
		choices = append(choices, c.choices[idx].snapshot())
	}
	out["choices"] = choices
	if c.usage != nil {
		out["usage"] = cloneMap(c.usage)
	}
	return out, true
}

func (c *openAIChatChunkCollector) choice(idx int) *chatChoiceState {
	state := c.choices[idx]
	if state != nil {
		return state
	}
	state = &chatChoiceState{
		index:     idx,
		message:   map[string]any{},
		toolCalls: map[int]*chatToolCallState{},
	}
	c.choices[idx] = state
	return state
}

func (c *chatChoiceState) applyDelta(delta map[string]any) error {
	if role := strings.TrimSpace(jsonutil.CoerceScalarString(delta["role"])); role != "" {
		c.message["role"] = role
	}
	if v, ok := delta["content"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%w: chat.completions delta.content must be a string", ErrInvalidChunkPayload)
		}
		c.content.WriteString(s)
	}
	if v, ok := delta["refusal"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%w: chat.completions delta.refusal must be a string", ErrInvalidChunkPayload)
		}
		c.refusal.WriteString(s)
	}
	if v, ok := delta["reasoning_content"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%w: chat.completions delta.reasoning_content must be a string", ErrInvalidChunkPayload)
		}
		c.reasoning.WriteString(s)
	}
	if fn, _ := delta["function_call"].(map[string]any); fn != nil {
		if name := jsonutil.CoerceScalarString(fn["name"]); name != "" {
			if c.functionName != "" && c.functionName != name {
				return fmt.Errorf("%w: chat.completions function_call name conflict", ErrInvalidChunkPayload)
			}
			c.functionName = name
		}
		if v, ok := fn["arguments"]; ok && v != nil {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("%w: chat.completions function_call arguments must be a string", ErrInvalidChunkPayload)
			}
			c.functionArgs.WriteString(s)
		}
	}
	if rawToolCalls, ok := delta["tool_calls"]; ok {
		toolCalls, ok := rawToolCalls.([]any)
		if !ok {
			return fmt.Errorf("%w: chat.completions delta.tool_calls must be an array", ErrInvalidChunkPayload)
		}
		for pos, raw := range toolCalls {
			item, _ := raw.(map[string]any)
			if item == nil {
				return fmt.Errorf("%w: chat.completions tool_call delta must be an object", ErrInvalidChunkPayload)
			}
			idx, err := boundedChunkIndex(item["index"], pos, "tool_calls.index")
			if err != nil {
				return err
			}
			if err := c.toolCall(idx).apply(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *chatChoiceState) toolCall(idx int) *chatToolCallState {
	state := c.toolCalls[idx]
	if state != nil {
		return state
	}
	state = &chatToolCallState{index: idx}
	c.toolCalls[idx] = state
	return state
}

func (c *chatChoiceState) snapshot() map[string]any {
	msg := cloneMap(c.message)
	if c.content.Len() > 0 {
		msg["content"] = c.content.String()
	}
	if c.refusal.Len() > 0 {
		msg["refusal"] = c.refusal.String()
	}
	if c.reasoning.Len() > 0 {
		msg["reasoning_content"] = c.reasoning.String()
	}
	if c.functionName != "" || c.functionArgs.Len() > 0 {
		fn := map[string]any{}
		if c.functionName != "" {
			fn["name"] = c.functionName
		}
		if c.functionArgs.Len() > 0 {
			fn["arguments"] = c.functionArgs.String()
		}
		msg["function_call"] = fn
	}
	if len(c.toolCalls) > 0 {
		indexes := make([]int, 0, len(c.toolCalls))
		for idx := range c.toolCalls {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)
		toolCalls := make([]any, 0, len(indexes))
		for _, idx := range indexes {
			toolCalls = append(toolCalls, c.toolCalls[idx].snapshot())
		}
		msg["tool_calls"] = toolCalls
	}
	out := map[string]any{
		"index":   c.index,
		"message": msg,
	}
	if c.finishReasonSet {
		out["finish_reason"] = c.finishReason
	}
	return out
}

func (c *chatToolCallState) apply(item map[string]any) error {
	if id := jsonutil.CoerceScalarString(item["id"]); id != "" {
		if c.id != "" && c.id != id {
			return fmt.Errorf("%w: chat.completions tool_call id conflict", ErrInvalidChunkPayload)
		}
		c.id = id
	}
	if typ := jsonutil.CoerceScalarString(item["type"]); typ != "" {
		if c.typ != "" && c.typ != typ {
			return fmt.Errorf("%w: chat.completions tool_call type conflict", ErrInvalidChunkPayload)
		}
		c.typ = typ
	}
	fn, _ := item["function"].(map[string]any)
	if fn == nil {
		return nil
	}
	if name := jsonutil.CoerceScalarString(fn["name"]); name != "" {
		if c.name != "" && c.name != name {
			return fmt.Errorf("%w: chat.completions tool_call function name conflict", ErrInvalidChunkPayload)
		}
		c.name = name
	}
	if v, ok := fn["arguments"]; ok && v != nil {
		switch t := v.(type) {
		case string:
			c.arguments.WriteString(t)
		case map[string]any, []any:
			b, err := json.Marshal(t)
			if err != nil {
				return fmt.Errorf("%w: chat.completions tool_call arguments marshal failed: %v", ErrInvalidChunkPayload, err)
			}
			c.arguments.Write(b)
		default:
			return fmt.Errorf("%w: chat.completions tool_call arguments must be a string or JSON value", ErrInvalidChunkPayload)
		}
	}
	return nil
}

func (c *chatToolCallState) snapshot() map[string]any {
	out := map[string]any{"index": c.index}
	if c.id != "" {
		out["id"] = c.id
	}
	if c.typ != "" {
		out["type"] = c.typ
	}
	fn := map[string]any{}
	if c.name != "" {
		fn["name"] = c.name
	}
	if c.arguments.Len() > 0 {
		fn["arguments"] = c.arguments.String()
	}
	if len(fn) > 0 {
		out["function"] = fn
	}
	return out
}

type openAIResponsesChunkCollector struct {
	response map[string]any
	items    map[int]map[string]any
}

func newOpenAIResponsesChunkCollector() *openAIResponsesChunkCollector {
	return &openAIResponsesChunkCollector{items: map[int]map[string]any{}}
}

func (c *openAIResponsesChunkCollector) AddEvent(payload []byte) error {
	root, done, err := parseChunkObject(payload)
	if err != nil || done {
		return err
	}
	typ := strings.TrimSpace(jsonutil.CoerceScalarString(root["type"]))
	switch typ {
	case "error", "response.failed":
		return upstreamError(root)
	case "response.created", "response.queued", "response.in_progress", "response.completed", "response.incomplete":
		c.applyResponseState(root)
	case "response.output_item.added", "response.output_item.done":
		return c.applyResponseOutputItem(root)
	case "response.output_text.delta":
		return c.applyResponseOutputTextDelta(root)
	case "response.output_text.done":
		return c.applyResponseOutputTextDone(root)
	case "response.output_text.annotation.added":
		return c.applyResponseOutputTextAnnotation(root)
	case "response.refusal.delta":
		return c.applyResponseRefusalDelta(root)
	case "response.refusal.done":
		return c.applyResponseRefusalDone(root)
	case "response.function_call_arguments.delta":
		return c.applyResponseFunctionCallArgumentsDelta(root)
	case "response.function_call_arguments.done":
		return c.applyResponseFunctionCallArgumentsDone(root)
	case "response.custom_tool_call_input.delta":
		return c.appendResponseItemString(root, "custom_tool_call", "input", "delta", "custom_tool_call_input")
	case "response.custom_tool_call_input.done":
		return c.setResponseItemString(root, "custom_tool_call", "input", "input")
	case "response.mcp_call_arguments.delta":
		return c.appendResponseItemString(root, "mcp_call", "arguments", "delta", "mcp_call_arguments")
	case "response.mcp_call_arguments.done":
		return c.setResponseItemString(root, "mcp_call", "arguments", "arguments")
	case "response.code_interpreter_call_code.delta":
		return c.appendResponseItemString(root, "code_interpreter_call", "code", "delta", "code_interpreter_call_code")
	case "response.code_interpreter_call_code.done":
		return c.setResponseItemString(root, "code_interpreter_call", "code", "code")
	case "response.content_part.added", "response.content_part.done":
		return c.applyResponseContentPart(root)
	case "response.reasoning_text.delta", "response.reasoning.delta":
		return c.applyResponseReasoningTextDelta(root)
	case "response.reasoning_text.done", "response.reasoning.done":
		return c.applyResponseReasoningTextDone(root)
	case "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
		return c.applyResponseReasoningSummaryPart(root)
	case "response.reasoning_summary_text.delta":
		return c.applyResponseReasoningSummaryTextDelta(root)
	case "response.reasoning_summary_text.done":
		return c.applyResponseReasoningSummaryTextDone(root)
	case "":
		return fmt.Errorf("%w: responses chunk missing type", ErrInvalidChunkPayload)
	default:
		return nil
	}
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseState(root map[string]any) {
	if r, _ := root["response"].(map[string]any); r != nil {
		c.response = cloneMap(r)
	}
}

func (c *openAIResponsesChunkCollector) applyResponseOutputItem(root map[string]any) error {
	item, _ := root["item"].(map[string]any)
	if item == nil {
		return fmt.Errorf("%w: responses output item event missing item", ErrInvalidChunkPayload)
	}
	idx, err := boundedChunkIndex(root["output_index"], len(c.items), "output_index")
	if err != nil {
		return err
	}
	if v, ok := item["output_index"]; ok {
		idx, err = boundedChunkIndex(v, idx, "item.output_index")
		if err != nil {
			return err
		}
	}
	c.items[idx] = cloneMap(item)
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseOutputTextDelta(root map[string]any) error {
	idx, contentIdx, err := responseOutputContentIndexes(root)
	if err != nil {
		return err
	}
	text, ok := root["delta"].(string)
	if !ok {
		return fmt.Errorf("%w: responses output_text delta must be a string", ErrInvalidChunkPayload)
	}
	c.ensureResponseMessageText(idx, contentIdx, text)
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseOutputTextDone(root map[string]any) error {
	idx, contentIdx, err := responseOutputContentIndexes(root)
	if err != nil {
		return err
	}
	if text, ok := root["text"].(string); ok {
		c.setResponseMessageContentPart(idx, contentIdx, map[string]any{"type": "output_text", "text": text})
	}
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseOutputTextAnnotation(root map[string]any) error {
	idx, contentIdx, err := responseOutputContentIndexes(root)
	if err != nil {
		return err
	}
	annotation, ok := root["annotation"]
	if !ok {
		return fmt.Errorf("%w: responses output_text annotation event missing annotation", ErrInvalidChunkPayload)
	}
	annotationIdx, err := boundedChunkIndex(root["annotation_index"], 0, "annotation_index")
	if err != nil {
		return err
	}
	c.addResponseTextAnnotation(idx, contentIdx, annotationIdx, annotation)
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseRefusalDelta(root map[string]any) error {
	idx, contentIdx, err := responseOutputContentIndexes(root)
	if err != nil {
		return err
	}
	delta, ok := root["delta"].(string)
	if !ok {
		return fmt.Errorf("%w: responses refusal delta must be a string", ErrInvalidChunkPayload)
	}
	c.appendResponseMessagePartField(idx, contentIdx, "refusal", "refusal", delta)
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseRefusalDone(root map[string]any) error {
	idx, contentIdx, err := responseOutputContentIndexes(root)
	if err != nil {
		return err
	}
	if refusal, ok := root["refusal"].(string); ok {
		c.setResponseMessageContentPart(idx, contentIdx, map[string]any{"type": "refusal", "refusal": refusal})
	}
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseFunctionCallArgumentsDelta(root map[string]any) error {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return err
	}
	delta, ok := root["delta"].(string)
	if !ok {
		return fmt.Errorf("%w: responses function_call_arguments delta must be a string", ErrInvalidChunkPayload)
	}
	item := c.ensureResponseItem(idx, "function_call")
	item["arguments"] = jsonutil.CoerceScalarString(item["arguments"]) + delta
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseFunctionCallArgumentsDone(root map[string]any) error {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return err
	}
	if args, ok := root["arguments"].(string); ok {
		item := c.ensureResponseItem(idx, "function_call")
		item["arguments"] = args
	}
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseContentPart(root map[string]any) error {
	idx, contentIdx, err := responseOutputContentIndexes(root)
	if err != nil {
		return err
	}
	part, _ := root["part"].(map[string]any)
	if part == nil {
		return fmt.Errorf("%w: responses content part event missing part", ErrInvalidChunkPayload)
	}
	c.setResponseMessageContentPart(idx, contentIdx, part)
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseReasoningTextDelta(root map[string]any) error {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return err
	}
	delta, ok := root["delta"].(string)
	if !ok {
		return fmt.Errorf("%w: responses reasoning text delta must be a string", ErrInvalidChunkPayload)
	}
	item := c.ensureResponseItem(idx, "reasoning")
	item["text"] = jsonutil.CoerceScalarString(item["text"]) + delta
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseReasoningTextDone(root map[string]any) error {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return err
	}
	if text, ok := root["text"].(string); ok {
		item := c.ensureResponseItem(idx, "reasoning")
		item["text"] = text
	}
	return nil
}

func responseReasoningSummaryIndexes(root map[string]any) (int, int, error) {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return 0, 0, err
	}
	contentIdx, err := boundedChunkIndex(root["content_index"], 0, "content_index")
	if err != nil {
		return 0, 0, err
	}
	summaryIdx, err := boundedChunkIndex(root["summary_index"], contentIdx, "summary_index")
	if err != nil {
		return 0, 0, err
	}
	return idx, summaryIdx, nil
}

func (c *openAIResponsesChunkCollector) applyResponseReasoningSummaryPart(root map[string]any) error {
	idx, summaryIdx, err := responseReasoningSummaryIndexes(root)
	if err != nil {
		return err
	}
	part, _ := root["part"].(map[string]any)
	if part == nil {
		return fmt.Errorf("%w: responses reasoning summary part event missing part", ErrInvalidChunkPayload)
	}
	c.setReasoningSummaryPart(idx, summaryIdx, part)
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseReasoningSummaryTextDelta(root map[string]any) error {
	idx, summaryIdx, err := responseReasoningSummaryIndexes(root)
	if err != nil {
		return err
	}
	delta, ok := root["delta"].(string)
	if !ok {
		return fmt.Errorf("%w: responses reasoning summary text delta must be a string", ErrInvalidChunkPayload)
	}
	part := c.ensureReasoningSummaryPart(idx, summaryIdx)
	part["text"] = jsonutil.CoerceScalarString(part["text"]) + delta
	return nil
}

func (c *openAIResponsesChunkCollector) applyResponseReasoningSummaryTextDone(root map[string]any) error {
	idx, summaryIdx, err := responseReasoningSummaryIndexes(root)
	if err != nil {
		return err
	}
	if text, ok := root["text"].(string); ok {
		part := c.ensureReasoningSummaryPart(idx, summaryIdx)
		part["text"] = text
	}
	return nil
}

func (c *openAIResponsesChunkCollector) Snapshot() (map[string]any, bool) {
	if c.response == nil && len(c.items) == 0 {
		return nil, false
	}
	out := map[string]any{}
	if c.response != nil {
		out = cloneMap(c.response)
	}
	if len(c.items) > 0 {
		out["output"] = orderedMapValues(c.items)
	}
	return out, true
}

func (c *openAIResponsesChunkCollector) ensureResponseItem(idx int, typ string) map[string]any {
	item := c.items[idx]
	if item != nil {
		return item
	}
	item = map[string]any{"type": typ}
	c.items[idx] = item
	return item
}

func (c *openAIResponsesChunkCollector) ensureResponseMessageText(outputIdx, contentIdx int, text string) {
	c.appendResponseMessagePartField(outputIdx, contentIdx, "output_text", "text", text)
}

func (c *openAIResponsesChunkCollector) appendResponseMessagePartField(outputIdx, contentIdx int, typ, field, text string) {
	item := c.ensureResponseItem(outputIdx, "message")
	if _, ok := item["role"]; !ok {
		item["role"] = "assistant"
	}
	content, _ := item["content"].([]any)
	for len(content) <= contentIdx {
		content = append(content, map[string]any{"type": "output_text", "text": ""})
	}
	part, _ := content[contentIdx].(map[string]any)
	if part == nil {
		part = map[string]any{"type": typ, field: ""}
	}
	part["type"] = typ
	part[field] = jsonutil.CoerceScalarString(part[field]) + text
	content[contentIdx] = part
	item["content"] = content
}

func (c *openAIResponsesChunkCollector) setResponseMessageContentPart(outputIdx, contentIdx int, part map[string]any) {
	item := c.ensureResponseItem(outputIdx, "message")
	if _, ok := item["role"]; !ok {
		item["role"] = "assistant"
	}
	content, _ := item["content"].([]any)
	for len(content) <= contentIdx {
		content = append(content, map[string]any{"type": "output_text", "text": ""})
	}
	content[contentIdx] = cloneMap(part)
	item["content"] = content
}

func (c *openAIResponsesChunkCollector) addResponseTextAnnotation(outputIdx, contentIdx, annotationIdx int, annotation any) {
	item := c.ensureResponseItem(outputIdx, "message")
	if _, ok := item["role"]; !ok {
		item["role"] = "assistant"
	}
	content, _ := item["content"].([]any)
	for len(content) <= contentIdx {
		content = append(content, map[string]any{"type": "output_text", "text": ""})
	}
	part, _ := content[contentIdx].(map[string]any)
	if part == nil {
		part = map[string]any{"type": "output_text", "text": ""}
	}
	part["type"] = "output_text"
	annotations, _ := part["annotations"].([]any)
	for len(annotations) <= annotationIdx {
		annotations = append(annotations, map[string]any{})
	}
	annotations[annotationIdx] = cloneAny(annotation)
	part["annotations"] = annotations
	content[contentIdx] = part
	item["content"] = content
}

func (c *openAIResponsesChunkCollector) appendResponseItemString(root map[string]any, itemType, field, rootField, eventName string) error {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return err
	}
	delta, ok := root[rootField].(string)
	if !ok {
		return fmt.Errorf("%w: responses %s delta must be a string", ErrInvalidChunkPayload, eventName)
	}
	item := c.ensureResponseItem(idx, itemType)
	item[field] = jsonutil.CoerceScalarString(item[field]) + delta
	return nil
}

func (c *openAIResponsesChunkCollector) setResponseItemString(root map[string]any, itemType, field, rootField string) error {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return err
	}
	if value, ok := root[rootField].(string); ok {
		item := c.ensureResponseItem(idx, itemType)
		item[field] = value
	}
	return nil
}

func responseOutputContentIndexes(root map[string]any) (int, int, error) {
	idx, err := boundedChunkIndex(root["output_index"], 0, "output_index")
	if err != nil {
		return 0, 0, err
	}
	contentIdx, err := boundedChunkIndex(root["content_index"], 0, "content_index")
	if err != nil {
		return 0, 0, err
	}
	return idx, contentIdx, nil
}

func (c *openAIResponsesChunkCollector) ensureReasoningSummaryPart(outputIdx, summaryIdx int) map[string]any {
	item := c.ensureResponseItem(outputIdx, "reasoning")
	summary, _ := item["summary"].([]any)
	for len(summary) <= summaryIdx {
		summary = append(summary, map[string]any{"type": "summary_text", "text": ""})
	}
	part, _ := summary[summaryIdx].(map[string]any)
	if part == nil {
		part = map[string]any{"type": "summary_text", "text": ""}
	}
	if _, ok := part["type"]; !ok {
		part["type"] = "summary_text"
	}
	summary[summaryIdx] = part
	item["summary"] = summary
	return part
}

func (c *openAIResponsesChunkCollector) setReasoningSummaryPart(outputIdx, summaryIdx int, part map[string]any) {
	item := c.ensureResponseItem(outputIdx, "reasoning")
	summary, _ := item["summary"].([]any)
	for len(summary) <= summaryIdx {
		summary = append(summary, map[string]any{"type": "summary_text", "text": ""})
	}
	summary[summaryIdx] = cloneMap(part)
	item["summary"] = summary
}

type anthropicMessagesChunkCollector struct {
	msg      map[string]any
	blocks   map[int]*anthropicBlock
	seenStop bool
}

func newAnthropicMessagesChunkCollector() *anthropicMessagesChunkCollector {
	return &anthropicMessagesChunkCollector{blocks: map[int]*anthropicBlock{}}
}

func (c *anthropicMessagesChunkCollector) AddEvent(payload []byte) error {
	root, done, err := parseChunkObject(payload)
	if err != nil || done {
		return err
	}
	typ := strings.TrimSpace(jsonutil.CoerceScalarString(root["type"]))
	switch typ {
	case "error":
		return upstreamError(root)
	case "message_start":
		m, _ := root["message"].(map[string]any)
		if m == nil {
			return fmt.Errorf("%w: anthropic message_start missing message", ErrInvalidChunkPayload)
		}
		c.msg = cloneMap(m)
		c.msg["content"] = []any{}
	case "content_block_start":
		idx, err := boundedChunkIndex(root["index"], len(c.blocks), "index")
		if err != nil {
			return err
		}
		block, _ := root["content_block"].(map[string]any)
		if block == nil {
			return fmt.Errorf("%w: anthropic content_block_start missing content_block", ErrInvalidChunkPayload)
		}
		if existing := c.blocks[idx]; existing != nil {
			if oldType := jsonutil.CoerceScalarString(existing.block["type"]); oldType != "" && oldType != jsonutil.CoerceScalarString(block["type"]) {
				return fmt.Errorf("%w: anthropic content block type conflict", ErrInvalidChunkPayload)
			}
		}
		c.blocks[idx] = &anthropicBlock{block: cloneMap(block)}
	case "content_block_delta":
		idx, err := boundedChunkIndex(root["index"], 0, "index")
		if err != nil {
			return err
		}
		delta, _ := root["delta"].(map[string]any)
		if delta == nil {
			return fmt.Errorf("%w: anthropic content_block_delta missing delta", ErrInvalidChunkPayload)
		}
		b := c.blocks[idx]
		if b == nil {
			switch jsonutil.CoerceScalarString(delta["type"]) {
			case "text_delta":
				b = &anthropicBlock{block: map[string]any{"type": "text"}}
				c.blocks[idx] = b
			default:
				return fmt.Errorf("%w: anthropic delta arrived before content_block_start", ErrInvalidChunkPayload)
			}
		}
		applyAnthropicDelta(b, delta)
	case "content_block_stop":
		idx, err := boundedChunkIndex(root["index"], 0, "index")
		if err != nil {
			return err
		}
		if b := c.blocks[idx]; b != nil {
			finalizeAnthropicBlock(b)
		}
	case "message_delta":
		if c.msg == nil {
			c.msg = map[string]any{}
		}
		if delta, _ := root["delta"].(map[string]any); delta != nil {
			for _, key := range []string{"stop_reason", "stop_sequence"} {
				if v, ok := delta[key]; ok {
					c.msg[key] = v
				}
			}
		}
		if usage, _ := root["usage"].(map[string]any); usage != nil {
			c.msg["usage"] = mergeMaps(asMap(c.msg["usage"]), usage)
		}
	case "message_stop":
		c.seenStop = true
	case "ping":
		return nil
	case "":
		return fmt.Errorf("%w: anthropic chunk missing type", ErrInvalidChunkPayload)
	default:
		return nil
	}
	return nil
}

func (c *anthropicMessagesChunkCollector) Snapshot() (map[string]any, bool) {
	if c.msg == nil && len(c.blocks) == 0 {
		return nil, false
	}
	out := map[string]any{}
	if c.msg != nil {
		out = cloneMap(c.msg)
	}
	out["content"] = c.snapshotContent()
	return out, true
}

func (c *anthropicMessagesChunkCollector) snapshotContent() []any {
	indexes := make([]int, 0, len(c.blocks))
	for idx := range c.blocks {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	content := make([]any, 0, len(indexes))
	for _, idx := range indexes {
		b := c.blocks[idx]
		if b == nil {
			continue
		}
		block := cloneMap(b.block)
		finalizeAnthropicBlockSnapshot(block, b.jsonParts.String())
		content = append(content, block)
	}
	return content
}

func finalizeAnthropicBlockSnapshot(block map[string]any, raw string) {
	if raw == "" {
		return
	}
	var obj any
	if json.Unmarshal([]byte(raw), &obj) == nil {
		block["input"] = obj
	} else {
		block["input"] = raw
	}
}

type geminiGenerateContentChunkCollector struct {
	out        map[string]any
	candidates map[int]map[string]any
}

func newGeminiGenerateContentChunkCollector() *geminiGenerateContentChunkCollector {
	return &geminiGenerateContentChunkCollector{
		out:        map[string]any{},
		candidates: map[int]map[string]any{},
	}
}

func (c *geminiGenerateContentChunkCollector) AddEvent(payload []byte) error {
	root, done, err := parseChunkObject(payload)
	if err != nil || done {
		return err
	}
	for _, key := range []string{"usageMetadata", "promptFeedback", "modelVersion", "responseId"} {
		if v, ok := root[key]; ok {
			c.out[key] = v
		}
	}
	rawCandidates, ok := root["candidates"]
	if !ok {
		if len(c.out) > 0 {
			return nil
		}
		return fmt.Errorf("%w: gemini chunk missing candidates", ErrInvalidChunkPayload)
	}
	candidates, ok := rawCandidates.([]any)
	if !ok {
		return fmt.Errorf("%w: gemini candidates must be an array", ErrInvalidChunkPayload)
	}
	for pos, raw := range candidates {
		in, _ := raw.(map[string]any)
		if in == nil {
			return fmt.Errorf("%w: gemini candidate must be an object", ErrInvalidChunkPayload)
		}
		idx, err := boundedChunkIndex(in["index"], pos, "candidates.index")
		if err != nil {
			return err
		}
		dst := c.candidates[idx]
		if dst == nil {
			dst = map[string]any{"index": idx}
			c.candidates[idx] = dst
		}
		mergeGeminiCandidate(dst, in)
	}
	return nil
}

func (c *geminiGenerateContentChunkCollector) Snapshot() (map[string]any, bool) {
	if len(c.out) == 0 && len(c.candidates) == 0 {
		return nil, false
	}
	out := cloneMap(c.out)
	if len(c.candidates) > 0 {
		out["candidates"] = orderedMapValues(c.candidates)
	}
	return out, true
}

func orderedMapValues(items map[int]map[string]any) []any {
	indexes := make([]int, 0, len(items))
	for idx := range items {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	out := make([]any, 0, len(indexes))
	for _, idx := range indexes {
		out = append(out, cloneAny(items[idx]))
	}
	return out
}

func cloneAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, v := range t {
			out[k] = cloneAny(v)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, v := range t {
			out[i] = cloneAny(v)
		}
		return out
	default:
		return t
	}
}
